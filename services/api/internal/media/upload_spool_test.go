package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUploadServiceSpoolCapIncludesBlockedPutsAndCanceledWaiter(t *testing.T) {
	dir := t.TempDir()
	repo, store := newUploadRepositoryAndObjectStore()
	putBlock := make(chan struct{})
	store.putEntered, store.putBlock = make(chan struct{}, 2), putBlock
	var releaseOnce sync.Once
	releasePuts := func() { releaseOnce.Do(func() { close(putBlock) }) }
	defer releasePuts()
	service := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir, Clock: func() time.Time { return testClock() }})
	waiterContext, cancelWaiter := context.WithCancel(context.Background())
	defer cancelWaiter()
	observed := &observedContext{Context: waiterContext, evaluated: make(chan struct{})}
	results := make(chan prepareResult, 3)
	requestIDs := [...]string{testRequestID, "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"}
	body := testJPEG(t, 10, 10)
	start := func(index int, receiveContext context.Context) {
		go func(index int, receiveContext context.Context) {
			_, err := service.PrepareImageUpload(receiveContext, "user-1", testReceiverWithID(body, requestIDs[index]))
			results <- prepareResult{index: index, err: err}
		}(index, receiveContext)
	}
	start(0, context.Background())
	start(1, context.Background())
	for range 2 {
		waitSignal(t, store.putEntered)
	}
	assertScratchFiles(t, dir, 2)
	start(2, observed)
	waitSignal(t, observed.evaluated)
	assertScratchFiles(t, dir, 2)
	if cap(spoolSlots) != 2 || len(spoolSlots) != 2 || store.putCount() != 2 || len(repo.pendingInputs) != 2 {
		t.Fatalf("full-slot barrier put=%d pending=%d", store.putCount(), len(repo.pendingInputs))
	}
	cancelWaiter()
	waiterResult := waitPrepareResult(t, results)
	for waiterResult.index != 2 {
		waiterResult = waitPrepareResult(t, results)
	}
	if !errors.Is(waiterResult.err, context.Canceled) {
		t.Fatalf("waiter err=%v", waiterResult.err)
	}
	if len(spoolSlots) != 2 {
		t.Fatalf("spool slots after cancel=%d", len(spoolSlots))
	}
	releasePuts()
	for range 2 {
		completedResult := waitPrepareResult(t, results)
		if completedResult.index == 2 || completedResult.err != nil {
			t.Fatalf("blocked put result=%+v", completedResult)
		}
	}
	assertScratchEmpty(t, dir)
	if len(repo.uploads) != 2 || store.putCount() != 2 || len(spoolSlots) != 0 {
		t.Fatalf("final rows=%d puts=%d", len(repo.uploads), store.putCount())
	}
}

func TestUploadServicePanickingReceiverCleansScratchAndReleasesSpool(t *testing.T) {
	now, body := testClock(), testJPEG(t, 10, 10)
	dir := t.TempDir()
	repo, store := newUploadRepositoryAndObjectStore()
	service := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir, Clock: func() time.Time { return now }})
	panicValue := errors.New("receiver panic")
	func() {
		defer func() {
			if recovered := recover(); recovered != panicValue {
				t.Fatalf("panic=%v", recovered)
			}
		}()
		_, _ = service.PrepareImageUpload(context.Background(), "user-1", func(_ context.Context, writer io.Writer) (string, error) {
			if _, err := writer.Write(body); err != nil {
				t.Fatal(err)
			}
			panic(panicValue)
		})
	}()
	assertNoPublication(t, repo, store, dir)
	capacity := cap(spoolSlots)
	entered := make(chan struct{}, capacity)
	release := make(chan struct{})
	done := make(chan error, capacity)
	cancels := make([]context.CancelFunc, 0, capacity)
	for range capacity {
		ctx, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		go func(ctx context.Context) {
			staged, _, err := service.spool(ctx, func(_ context.Context, writer io.Writer) (string, error) {
				if _, err := writer.Write(body); err != nil {
					return "", err
				}
				entered <- struct{}{}
				<-release
				return testRequestID, nil
			})
			if staged != nil {
				err = errors.Join(err, staged.CloseRemove())
			}
			done <- err
		}(ctx)
	}
	timedOut := false
	deadline := time.NewTimer(time.Second)
	for range capacity {
		select {
		case <-entered:
		case <-deadline.C:
			timedOut = true
		}
		if timedOut {
			break
		}
	}
	if !deadline.Stop() {
		select {
		case <-deadline.C:
		default:
		}
	}
	close(release)
	if timedOut {
		for _, cancel := range cancels {
			cancel()
		}
	}
	var spoolErr error
	for range capacity {
		if err := <-done; err != nil && spoolErr == nil {
			spoolErr = err
		}
	}
	if !timedOut {
		for _, cancel := range cancels {
			cancel()
		}
	}
	if timedOut {
		t.Fatal("spool capacity was not restored")
	}
	if spoolErr != nil {
		t.Fatalf("spool err=%v", spoolErr)
	}
	assertNoPublication(t, repo, store, dir)
}

func TestUploadServiceReceiverErrorsAreStickyAndCallbackWins(t *testing.T) {
	t.Run("sticky multiwrite", func(t *testing.T) {
		repo, store := newUploadRepositoryAndObjectStore()
		dir := t.TempDir()
		var firstErr, secondErr, thirdErr error
		var firstCount, secondCount, thirdCount int
		receive := func(_ context.Context, writer io.Writer) (string, error) {
			firstCount, firstErr = writer.Write(bytes.Repeat([]byte{'x'}, int(MaxEncodedImageSize)))
			secondCount, secondErr = writer.Write([]byte{'x'})
			thirdCount, thirdErr = writer.Write([]byte{'x'})
			return testRequestID, nil
		}
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", receive)
		if err != ErrMediaTooLarge || firstCount != int(MaxEncodedImageSize) || firstErr != nil || secondCount != 0 || secondErr != ErrMediaTooLarge || thirdCount != 0 || thirdErr != ErrMediaTooLarge {
			t.Fatalf("counts=%d,%d,%d errors=%v,%v,%v result=%v", firstCount, secondCount, thirdCount, firstErr, secondErr, thirdErr, err)
		}
		assertNoPublication(t, repo, store, dir)
	})
	t.Run("callback precedence", func(t *testing.T) {
		repo, store := newUploadRepositoryAndObjectStore()
		dir := t.TempDir()
		callbackErr := errors.New("receiver callback failed")
		receive := func(_ context.Context, writer io.Writer) (string, error) {
			_, _ = writer.Write(bytes.Repeat([]byte{'x'}, int(MaxEncodedImageSize)))
			_, _ = writer.Write([]byte{'x'})
			return testRequestID, callbackErr
		}
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", receive)
		if err != callbackErr {
			t.Fatalf("callback error=%v", err)
		}
		assertNoPublication(t, repo, store, dir)
	})
}

func TestUploadServiceRejectsInvalidRequestIDs(t *testing.T) {
	body := testJPEG(t, 10, 10)
	for _, requestID := range []string{"not-a-uuid", strings.ToUpper("abcdefabcdef4abc8defabcdefabcdef")} {
		repo, store := newUploadRepositoryAndObjectStore()
		scratchDir := t.TempDir()
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: scratchDir}).PrepareImageUpload(context.Background(), "user-1", testReceiverWithID(body, requestID))
		if !errors.Is(err, ErrInvalidUploadRequest) {
			t.Fatalf("requestID=%q err=%v", requestID, err)
		}
		assertNoPublication(t, repo, store, scratchDir)
	}
}

func TestPrepareImageUploadReportsScratchErrors(t *testing.T) {
	repo, store := newUploadRepositoryAndObjectStore()
	_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: filepath.Join(t.TempDir(), "missing")}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if !errors.Is(err, ErrMediaStorageUnavailable) || !strings.Contains(err.Error(), "create image scratch file") {
		t.Fatalf("create error=%v", err)
	}
	repo, store = newUploadRepositoryAndObjectStore()
	dir := t.TempDir()
	var removeErr error
	store.putHook = func(input PutObject) { removeErr = os.Remove(input.Body.(*os.File).Name()) }
	_, err = newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if removeErr != nil || !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("cleanup hook=%v prepare=%v", removeErr, err)
	}
	assertScratchEmpty(t, dir)
}
