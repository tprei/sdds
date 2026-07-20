package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestUploadServiceStoresExactPutAndReplays(t *testing.T) {
	now, body := testClock(), testJPEG(t, 12, 8)
	repo, store := newUploadRepositoryAndObjectStore()
	service := newUploadService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }})
	receipt, err := service.PrepareImageUpload(context.Background(), "user-1", testReceiver(body))
	if err != nil || receipt.ContentType != "image/jpeg" || receipt.ByteSize != int64(len(body)) || receipt.Width != 12 || receipt.Height != 8 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if len(repo.pendingInputs) != 1 || store.putCount() != 1 || len(repo.readyInputs) != 1 {
		t.Fatalf("counts pending=%d put=%d ready=%d", len(repo.pendingInputs), store.putCount(), len(repo.readyInputs))
	}
	upload, ok := repo.uploads["user-1/"+testRequestID]
	if !ok || upload.State != UploadReady {
		t.Fatalf("stored upload=%+v present=%v", upload, ok)
	}
	put := store.putCall(0)
	digest := sha256.Sum256(body)
	if put.key != upload.StorageKey || !bytes.Equal(put.body, body) || put.size != int64(len(body)) || put.digest != digest || put.contentType != "image/jpeg" {
		t.Fatalf("put=%+v", put)
	}
	pending := repo.pendingInputs[0]
	wantReady := ReadyInput{ID: upload.ID, UserID: "user-1", UploadRequestID: testRequestID, SHA256: upload.SHA256, WriteLeaseUntil: pending.WriteLeaseUntil}
	if got := repo.readyInputs[0]; got != wantReady {
		t.Fatalf("ready input=%+v want=%+v", got, wantReady)
	}
	pendingCount, putCount, readyCount := len(repo.pendingInputs), store.putCount(), len(repo.readyInputs)
	replay, err := service.PrepareImageUpload(context.Background(), "user-1", testReceiver(body))
	if err != nil || replay.ImageUploadID != receipt.ImageUploadID || len(repo.pendingInputs) != pendingCount || store.putCount() != putCount || len(repo.readyInputs) != readyCount {
		t.Fatalf("replay=%+v err=%v counts pending=%d put=%d ready=%d", replay, err, len(repo.pendingInputs), store.putCount(), len(repo.readyInputs))
	}
	_, err = service.PrepareImageUpload(context.Background(), "user-1", testReceiver(testPNGGray(t, 12, 8)))
	if !errors.Is(err, ErrUploadIdempotencyConflict) || len(repo.pendingInputs) != pendingCount || store.putCount() != putCount || len(repo.readyInputs) != readyCount {
		t.Fatalf("conflict=%v counts pending=%d put=%d ready=%d", err, len(repo.pendingInputs), store.putCount(), len(repo.readyInputs))
	}
	pngBody := testPNGGray(t, 13, 9)
	pngRepo, pngStore := newUploadRepositoryAndObjectStore()
	pngReceipt, pngErr := newUploadService(t, pngRepo, pngStore, UploadConfig{Clock: func() time.Time { return testClock() }}).PrepareImageUpload(context.Background(), "user-1", testReceiver(pngBody))
	pngPut, pngDigest := pngStore.putCall(0), sha256.Sum256(pngBody)
	if pngErr != nil || pngReceipt.ContentType != "image/png" || pngReceipt.ByteSize != int64(len(pngBody)) || pngReceipt.Width != 13 || pngReceipt.Height != 9 || pngReceipt.ImageUploadID == "" || pngPut.contentType != "image/png" || pngPut.size != int64(len(pngBody)) || pngPut.digest != pngDigest || pngPut.key != pngRepo.pendingInputs[0].StorageKey || !bytes.Equal(pngPut.body, pngBody) {
		t.Fatalf("receipt=%+v put=%+v err=%v", pngReceipt, pngPut, pngErr)
	}
}

func TestUploadServiceUsesFreshClockAfterCleanup(t *testing.T) {
	first, second := testClock(), testClock().Add(3*time.Minute)
	clock := advancingTestClock(first, second)
	repo, store := newUploadRepositoryAndObjectStore()
	_, err := newUploadService(t, repo, store, UploadConfig{Clock: clock, CleanupBatch: 7}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.claimCalls) != 1 || !repo.claimCalls[0].now.Equal(first) || len(repo.pendingInputs) != 1 {
		t.Fatalf("claim=%+v pending=%+v", repo.claimCalls, repo.pendingInputs)
	}
	pending := repo.pendingInputs[0]
	if !pending.UpdatedAt.Equal(second) || !pending.WriteLeaseUntil.Equal(second.Add(UploadLeaseDuration)) || !pending.CreatedAt.Equal(second) {
		t.Fatalf("pending times=%+v", pending)
	}
}

func TestUploadServiceCleanupHonorsCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	repo, store := newUploadRepositoryAndObjectStore()
	repo.expired = []Upload{cleanupRow("expired", "note-images/expired")}
	store.deleteEntered, store.deleteBlock = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- newUploadService(t, repo, store, UploadConfig{CleanupTimeout: time.Second}).CleanupExpired(parent, testClock())
	}()
	waitSignal(t, store.deleteEntered)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || len(store.deleteCalls) != 1 || store.deleteCalls[0] != repo.expired[0].StorageKey || len(repo.finalizeCalls) != 0 || len(repo.compactInputs) != 1 || repo.compactInputs[0] != (repositoryCompactCall{now: testClock(), limit: DefaultCleanupBatch}) {
			t.Fatalf("err=%v deletes=%v finalizes=%v compactions=%v", err, store.deleteCalls, repo.finalizeCalls, repo.compactInputs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cleanup")
	}
}

func TestUploadServiceCleanupErrors(t *testing.T) {
	now, batch := testClock(), 7
	row := cleanupRow("expired", "note-images/expired")
	claimErr, compactErr := errors.New("claim failed"), errors.New("compact failed")
	for _, test := range []struct {
		name                             string
		rows                             []Upload
		deleteErrors                     []error
		claim, final, compact, wantCause error
		want                             error
		keys                             []ObjectKey
		ids                              []string
		states                           []UploadState
		doCompact                        bool
	}{
		{name: "claim", rows: []Upload{row}, claim: claimErr, wantCause: claimErr, want: ErrMediaStorageUnavailable, states: []UploadState{UploadDeleting}},
		{name: "exhausted", rows: []Upload{row}, deleteErrors: []error{ErrObjectUnavailable, ErrObjectUnavailable, ErrObjectUnavailable}, want: ErrMediaStorageUnavailable, keys: []ObjectKey{row.StorageKey, row.StorageKey, row.StorageKey}, states: []UploadState{UploadDeleting}, doCompact: true},
		{name: "not found", rows: []Upload{row}, deleteErrors: []error{ErrObjectNotFound}, ids: []string{row.ID}, keys: []ObjectKey{row.StorageKey}, states: []UploadState{UploadExpired}, doCompact: true},
		{name: "integrity", rows: []Upload{row}, deleteErrors: []error{ErrObjectIntegrity}, want: ErrMediaIntegrity, keys: []ObjectKey{row.StorageKey}, states: []UploadState{UploadDeleting}, doCompact: true},
		{name: "invalid key", rows: []Upload{row}, deleteErrors: []error{ErrInvalidObjectKey}, want: ErrMediaIntegrity, keys: []ObjectKey{row.StorageKey}, states: []UploadState{UploadDeleting}, doCompact: true},
		{name: "multiple rows", rows: []Upload{cleanupRow("first", "note-images/first"), cleanupRow("second", "note-images/second")}, deleteErrors: []error{ErrObjectIntegrity}, want: ErrMediaIntegrity, keys: []ObjectKey{"note-images/first", "note-images/second"}, ids: []string{"second"}, states: []UploadState{UploadDeleting, UploadExpired}, doCompact: true},
		{name: "finalize", rows: []Upload{row}, final: ErrUploadStateConflict, wantCause: ErrUploadStateConflict, want: ErrMediaStorageUnavailable, keys: []ObjectKey{row.StorageKey}, ids: []string{row.ID}, states: []UploadState{UploadDeleting}, doCompact: true},
		{name: "compact", rows: []Upload{row}, compact: compactErr, wantCause: compactErr, keys: []ObjectKey{row.StorageKey}, ids: []string{row.ID}, states: []UploadState{UploadExpired}, want: ErrMediaStorageUnavailable, doCompact: true},
		{name: "both", rows: []Upload{row}, final: ErrUploadStateConflict, compact: compactErr, wantCause: ErrUploadStateConflict, keys: []ObjectKey{row.StorageKey}, ids: []string{row.ID}, states: []UploadState{UploadDeleting}, want: ErrMediaStorageUnavailable, doCompact: true},
	} {
		repo, store := newUploadRepositoryAndObjectStore()
		repo.expired, repo.claimErrors, repo.finalizeErrors, repo.compactErrors = test.rows, []error{test.claim}, []error{test.final}, []error{test.compact}
		store.deleteErrors = test.deleteErrors
		err := newUploadService(t, repo, store, UploadConfig{CleanupBatch: batch}).CleanupExpired(context.Background(), now)
		if test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) || test.wantCause != nil && !errors.Is(err, test.wantCause) || test.final != nil && !errors.Is(err, test.final) || test.compact != nil && !errors.Is(err, test.compact) {
			t.Fatalf("%s: cleanup=%v want=%v", test.name, err, test.want)
		}
		assertCleanup(t, repo, store, now, batch, test.keys, test.ids, test.states, test.doCompact)
	}
}
