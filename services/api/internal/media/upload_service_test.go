package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeUploadRepository struct {
	mu                 sync.Mutex
	clock              func() time.Time
	uploads            map[string]Upload
	readyResponses     []repositoryReadyResponse
	clearLeaseErrors   []error
	markDeletingErrors []error
	claimErrors        []error
	expired            []Upload
	finalizeErrors     []error
	compactErrors      []error
	findCalls          int
	pendingInputs      []PendingInput
	readyInputs        []ReadyInput
	clearLeaseInputs   []LeaseInput
	markDeletingInputs []LeaseInput
	claimCalls         []repositoryClaimCall
	finalizeCalls      []repositoryFinalizeCall
	compactCalls       int
	compactInputs      []repositoryCompactCall
}
type repositoryReadyResponse struct {
	ready bool
	err   error
	apply bool
}
type repositoryClaimCall struct {
	now   time.Time
	limit int
}
type repositoryFinalizeCall struct {
	id  string
	now time.Time
}
type repositoryCompactCall struct {
	now   time.Time
	limit int
}

func (repo *fakeUploadRepository) FindByUserRequest(_ context.Context, userID, requestID string) (Upload, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.findCalls++
	if upload, ok := repo.uploads[userID+"/"+requestID]; ok {
		return upload, nil
	}
	return Upload{}, ErrUploadNotFound
}
func (repo *fakeUploadRepository) BeginPending(_ context.Context, input PendingInput) (Upload, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.pendingInputs = append(repo.pendingInputs, input)
	key := input.UserID + "/" + input.UploadRequestID
	if upload, ok := repo.uploads[key]; ok {
		upload.UpdatedAt, upload.WriteLeaseUntil = input.UpdatedAt, input.WriteLeaseUntil
		repo.uploads[key] = upload
		return upload, nil
	}
	upload := Upload{ID: input.ID, UserID: input.UserID, StorageKey: input.StorageKey, UploadRequestID: input.UploadRequestID, State: UploadPending, ContentType: input.ContentType, ByteSize: input.ByteSize, Width: input.Width, Height: input.Height, SHA256: input.SHA256, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt, WriteLeaseUntil: input.WriteLeaseUntil, ExpiresAt: input.ExpiresAt, RequestRetentionUntil: input.RequestRetentionUntil}
	repo.uploads[key] = upload
	return upload, nil
}
func normalizeFakeTime(value time.Time) time.Time {
	return time.UnixMilli(value.UnixMilli()).UTC()
}
func (repo *fakeUploadRepository) MarkReady(_ context.Context, input ReadyInput) (bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.readyInputs = append(repo.readyInputs, input)
	response := repositoryReadyResponse{ready: true}
	if len(repo.readyResponses) > 0 {
		response = repo.readyResponses[0]
		repo.readyResponses = repo.readyResponses[1:]
	}
	now := time.Time{}
	if repo.clock != nil {
		now = normalizeFakeTime(repo.clock())
	}
	for key, upload := range repo.uploads {
		storedLease, inputLease := normalizeFakeTime(upload.WriteLeaseUntil), normalizeFakeTime(input.WriteLeaseUntil)
		if upload.ID != input.ID || upload.UserID != input.UserID || upload.UploadRequestID != input.UploadRequestID || upload.SHA256 != input.SHA256 || upload.State != UploadPending || !storedLease.Equal(inputLease) || !storedLease.After(now) {
			continue
		}
		if response.apply || (response.err == nil && response.ready) {
			upload.State, upload.WriteLeaseUntil = UploadReady, time.Time{}
			repo.uploads[key] = upload
		}
		return response.ready, response.err
	}
	return false, response.err
}
func (repo *fakeUploadRepository) ClearLease(_ context.Context, input LeaseInput) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.clearLeaseInputs = append(repo.clearLeaseInputs, input)
	if err := nextError(&repo.clearLeaseErrors); err != nil {
		return err
	}
	repo.updateLease(input, func(upload *Upload) { upload.WriteLeaseUntil = time.Time{} })
	return nil
}
func (repo *fakeUploadRepository) MarkDeleting(_ context.Context, input LeaseInput) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.markDeletingInputs = append(repo.markDeletingInputs, input)
	if err := nextError(&repo.markDeletingErrors); err != nil {
		return err
	}
	repo.updateLease(input, func(upload *Upload) { upload.State, upload.WriteLeaseUntil = UploadDeleting, time.Time{} })
	return nil
}
func (repo *fakeUploadRepository) ClaimExpired(ctx context.Context, now time.Time, limit int) ([]Upload, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.claimCalls = append(repo.claimCalls, repositoryClaimCall{now: now, limit: limit})
	if err := nextError(&repo.claimErrors); err != nil {
		return nil, err
	}
	return append([]Upload(nil), repo.expired...), nil
}
func (repo *fakeUploadRepository) FinalizeExpired(_ context.Context, id string, now time.Time) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.finalizeCalls = append(repo.finalizeCalls, repositoryFinalizeCall{id: id, now: now})
	if err := nextError(&repo.finalizeErrors); err != nil {
		return err
	}
	for key, upload := range repo.uploads {
		if upload.ID == id {
			upload.State, upload.WriteLeaseUntil = UploadExpired, time.Time{}
			repo.uploads[key] = upload
		}
	}
	for index := range repo.expired {
		if repo.expired[index].ID == id {
			repo.expired[index].State, repo.expired[index].WriteLeaseUntil = UploadExpired, time.Time{}
		}
	}
	return nil
}
func (repo *fakeUploadRepository) CompactExpired(_ context.Context, now time.Time, limit int) (int64, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.compactCalls++
	repo.compactInputs = append(repo.compactInputs, repositoryCompactCall{now: now, limit: limit})
	return 0, nextError(&repo.compactErrors)
}
func (repo *fakeUploadRepository) QuotaSnapshot(_ context.Context, _ string, _ time.Time) (Quota, error) {
	return Quota{}, nil
}
func (repo *fakeUploadRepository) updateLease(input LeaseInput, update func(*Upload)) {
	for key, upload := range repo.uploads {
		if upload.ID == input.ID && upload.UserID == input.UserID && upload.UploadRequestID == input.UploadRequestID {
			update(&upload)
			repo.uploads[key] = upload
		}
	}
}
func nextError(queue *[]error) error {
	if len(*queue) == 0 {
		return nil
	}
	err := (*queue)[0]
	*queue = (*queue)[1:]
	return err
}
func TestFakeUploadRepositoryMarkReadyRequiresPendingFence(t *testing.T) {
	now := testClock()
	lease := now.Add(time.Minute)
	upload := Upload{
		ID:                    "upload-1",
		UserID:                "user-1",
		StorageKey:            "note-images/upload-1",
		UploadRequestID:       testRequestID,
		State:                 UploadPending,
		ContentType:           "image/jpeg",
		ByteSize:              123,
		Width:                 10,
		Height:                12,
		SHA256:                strings.Repeat("a", 64),
		CreatedAt:             now,
		UpdatedAt:             now,
		WriteLeaseUntil:       lease,
		ExpiresAt:             now.Add(time.Hour),
		RequestRetentionUntil: now.Add(2 * time.Hour),
	}
	lost := errors.New("lost mark-ready response")
	for _, test := range []struct {
		name        string
		state       UploadState
		sha         string
		storedLease time.Time
		lease       time.Time
		now         time.Time
		response    *repositoryReadyResponse
		ready       bool
		wantErr     error
	}{
		{name: "matching pending fence", state: UploadPending, sha: upload.SHA256, storedLease: lease, lease: lease, now: now, ready: true},
		{name: "stale lease", state: UploadPending, sha: upload.SHA256, storedLease: lease, lease: lease.Add(-time.Second), now: now},
		{name: "wrong digest", state: UploadPending, sha: strings.Repeat("b", 64), storedLease: lease, lease: lease, now: now},
		{name: "invalid state", state: UploadReady, sha: upload.SHA256, storedLease: lease, lease: lease, now: now},
		{name: "expired lease", state: UploadPending, sha: upload.SHA256, storedLease: now, lease: now, now: now},
		{name: "same-millisecond future", state: UploadPending, sha: upload.SHA256, storedLease: now.Add(800 * time.Nanosecond), lease: now.Add(800 * time.Nanosecond), now: now.Add(200 * time.Nanosecond)},
		{name: "failed fence ignores applied response", state: UploadPending, sha: upload.SHA256, storedLease: lease, lease: lease.Add(-time.Second), now: now, response: &repositoryReadyResponse{ready: false, err: lost, apply: true}, wantErr: lost},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := upload
			current.State, current.WriteLeaseUntil = test.state, test.storedLease
			repo := &fakeUploadRepository{
				clock:   func() time.Time { return test.now },
				uploads: map[string]Upload{"user-1/" + testRequestID: current},
			}
			if test.response != nil {
				repo.readyResponses = []repositoryReadyResponse{*test.response}
			}
			ready, err := repo.MarkReady(context.Background(), ReadyInput{ID: current.ID, UserID: current.UserID, UploadRequestID: current.UploadRequestID, SHA256: test.sha, WriteLeaseUntil: test.lease})
			if ready != test.ready || (test.wantErr == nil && err != nil) || (test.wantErr != nil && !errors.Is(err, test.wantErr)) {
				t.Fatalf("mark ready = %v, %v; want %v, %v", ready, err, test.ready, test.wantErr)
			}
			got := repo.uploads["user-1/"+testRequestID]
			want := current
			if test.ready {
				want.State, want.WriteLeaseUntil = UploadReady, time.Time{}
			}
			if got != want {
				t.Fatalf("upload = %#v, want %#v", got, want)
			}
		})
	}
}

type fakeObjectStore struct {
	mu            sync.Mutex
	objects       map[ObjectKey]fakeStoredObject
	existingOnPut *fakeStoredObject
	putErrors     []error
	deleteErrors  []error
	putCalls      []fakePutCall
	openCalls     int
	deleteCalls   []ObjectKey
	putEntered    chan struct{}
	putBlock      chan struct{}
	deleteEntered chan struct{}
	deleteBlock   <-chan struct{}
	putHook       func(PutObject)
}
type fakeStoredObject struct {
	body []byte
	size int64
}
type fakePutCall struct {
	key         ObjectKey
	body        []byte
	size        int64
	digest      [32]byte
	contentType string
}

func (store *fakeObjectStore) Put(ctx context.Context, input PutObject) error {
	body, readErr := io.ReadAll(input.Body)
	store.mu.Lock()
	store.putCalls = append(store.putCalls, fakePutCall{key: input.Key, body: append([]byte(nil), body...), size: input.Size, digest: input.SHA256, contentType: input.ContentType})
	putErr := nextError(&store.putErrors)
	block, entered, hook, existing := store.putBlock, store.putEntered, store.putHook, store.existingOnPut
	store.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if hook != nil {
		hook(input)
	}
	if readErr != nil {
		return readErr
	}
	if putErr != nil {
		if errors.Is(putErr, ErrObjectExists) && existing != nil {
			store.mu.Lock()
			stored := *existing
			stored.body = append([]byte(nil), stored.body...)
			store.objects[input.Key] = stored
			store.mu.Unlock()
		}
		return putErr
	}
	if int64(len(body)) != input.Size || sha256.Sum256(body) != input.SHA256 {
		return ErrObjectIntegrity
	}
	store.mu.Lock()
	store.objects[input.Key] = fakeStoredObject{body: body, size: input.Size}
	store.mu.Unlock()
	return nil
}
func (store *fakeObjectStore) Open(_ context.Context, key ObjectKey) (Object, error) {
	store.mu.Lock()
	store.openCalls++
	stored, ok := store.objects[key]
	store.mu.Unlock()
	if !ok {
		return Object{}, ErrObjectNotFound
	}
	return Object{Body: io.NopCloser(bytes.NewReader(stored.body)), Size: stored.size}, nil
}
func (store *fakeObjectStore) Delete(ctx context.Context, key ObjectKey) error {
	store.mu.Lock()
	store.deleteCalls = append(store.deleteCalls, key)
	deleteErr := nextError(&store.deleteErrors)
	block, entered := store.deleteBlock, store.deleteEntered
	store.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if deleteErr != nil {
		return deleteErr
	}
	store.mu.Lock()
	delete(store.objects, key)
	store.mu.Unlock()
	return nil
}
func (store *fakeObjectStore) putCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.putCalls)
}
func (store *fakeObjectStore) deleteCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.deleteCalls)
}
func (store *fakeObjectStore) putCall(index int) fakePutCall {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.putCalls[index]
}
func (store *fakeObjectStore) openCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.openCalls
}
func (store *fakeObjectStore) objectCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.objects)
}

func TestUploadServiceStoresExactPutAndReplays(t *testing.T) {
	now, body := testClock(), testJPEG(t, 12, 8)
	repo, store := emptyRepositoryAndStore()
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
	pngRepo, pngStore := emptyRepositoryAndStore()
	pngReceipt, pngErr := newUploadService(t, pngRepo, pngStore, UploadConfig{Clock: func() time.Time { return testClock() }}).PrepareImageUpload(context.Background(), "user-1", testReceiver(pngBody))
	pngPut, pngDigest := pngStore.putCall(0), sha256.Sum256(pngBody)
	if pngErr != nil || pngReceipt.ContentType != "image/png" || pngReceipt.ByteSize != int64(len(pngBody)) || pngReceipt.Width != 13 || pngReceipt.Height != 9 || pngReceipt.ImageUploadID == "" || pngPut.contentType != "image/png" || pngPut.size != int64(len(pngBody)) || pngPut.digest != pngDigest || pngPut.key != pngRepo.pendingInputs[0].StorageKey || !bytes.Equal(pngPut.body, pngBody) {
		t.Fatalf("receipt=%+v put=%+v err=%v", pngReceipt, pngPut, pngErr)
	}
}

func TestUploadServiceUsesFreshClockAfterCleanup(t *testing.T) {
	first, second := testClock(), testClock().Add(3*time.Minute)
	clock := advancingTestClock(first, second)
	repo, store := emptyRepositoryAndStore()
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
func TestUploadServiceReconcilesLostMarkReadyResponse(t *testing.T) {
	lost := errors.New("mark-ready response lost")
	repo, store := emptyRepositoryAndStore()
	repo.readyResponses = []repositoryReadyResponse{{ready: false, err: lost, apply: true}}
	receipt, err := newUploadService(t, repo, store, UploadConfig{Clock: func() time.Time { return testClock() }}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if err != nil || receipt.ImageUploadID == "" || repo.findCalls != 2 || len(repo.readyInputs) != 1 {
		t.Fatalf("receipt=%+v err=%v finds=%d ready=%d", receipt, err, repo.findCalls, len(repo.readyInputs))
	}
	upload := repo.uploads["user-1/"+testRequestID]
	if upload.State != UploadReady || store.deleteCount() != 0 {
		t.Fatalf("upload=%+v deletes=%d", upload, store.deleteCount())
	}
}

func TestUploadServiceReconcilesExistingObject(t *testing.T) {
	body := testJPEG(t, 10, 10)
	for _, test := range []struct {
		name       string
		content    []byte
		want       error
		compensate bool
	}{
		{name: "matching", content: body},
		{name: "digest mismatch", content: func() []byte { copyOfBody := append([]byte(nil), body...); copyOfBody[0] ^= 0xff; return copyOfBody }(), want: ErrMediaIntegrity, compensate: true},
	} {
		repo, store := emptyRepositoryAndStore()
		store.putErrors = []error{fmt.Errorf("lost put response: %w", ErrObjectExists)}
		store.existingOnPut = &fakeStoredObject{body: append([]byte(nil), test.content...), size: int64(len(test.content))}
		scratchDir := t.TempDir()
		receipt, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: scratchDir, Clock: func() time.Time { return testClock() }}).PrepareImageUpload(context.Background(), "user-1", testReceiver(body))
		if !errors.Is(err, test.want) || (test.want == nil && receipt.ImageUploadID == "") || store.putCount() != 1 || store.openCount() != 1 {
			t.Fatalf("%s: receipt=%+v err=%v puts=%d opens=%d", test.name, receipt, err, store.putCount(), store.openCount())
		}
		pending := repo.pendingInputs[0]
		if test.compensate {
			if len(repo.markDeletingInputs) != 1 || len(repo.finalizeCalls) != 1 {
				t.Fatalf("%s: deleting=%+v finalizes=%+v", test.name, repo.markDeletingInputs, repo.finalizeCalls)
			}
			if len(store.deleteCalls) != 1 || store.deleteCalls[0] != pending.StorageKey || store.objectCount() != 0 {
				t.Fatalf("%s: delete keys=%v objects=%d", test.name, store.deleteCalls, store.objectCount())
			}
			if repo.finalizeCalls[0].id != pending.ID || !repo.finalizeCalls[0].now.Equal(testClock()) {
				t.Fatalf("%s: finalize=%+v want id=%s now=%s", test.name, repo.finalizeCalls[0], pending.ID, testClock())
			}
			if upload := repo.uploads["user-1/"+testRequestID]; upload.State != UploadExpired {
				t.Fatalf("%s: compensated upload=%+v", test.name, upload)
			}
		} else {
			if len(repo.markDeletingInputs) != 0 || len(store.deleteCalls) != 0 || len(repo.finalizeCalls) != 0 || store.objectCount() != 1 {
				t.Fatalf("%s: unexpected compensation deleting=%+v deletes=%v finalizes=%v objects=%d", test.name, repo.markDeletingInputs, store.deleteCalls, repo.finalizeCalls, store.objectCount())
			}
			if upload := repo.uploads["user-1/"+testRequestID]; upload.State != UploadReady {
				t.Fatalf("%s: reconciled upload=%+v", test.name, upload)
			}
		}
		assertScratchEmpty(t, scratchDir)
	}
}
func TestUploadServicePutFailuresFenceOrClearLease(t *testing.T) {
	for _, test := range []struct {
		name            string
		putErr          error
		markDeletingErr error
		wantErr         error
		wantClear       bool
		wantDelete      bool
		wantFinalize    bool
	}{
		{name: "transient clears lease", putErr: fmt.Errorf("temporary: %w", ErrObjectUnavailable), wantErr: ErrMediaStorageUnavailable, wantClear: true},
		{name: "integrity compensates", putErr: ErrObjectIntegrity, wantErr: ErrMediaIntegrity, wantDelete: true, wantFinalize: true},
		{name: "compensation fence", putErr: ErrObjectIntegrity, markDeletingErr: ErrUploadStateConflict, wantErr: ErrMediaIntegrity, wantClear: false},
	} {
		repo, store := emptyRepositoryAndStore()
		repo.markDeletingErrors, store.putErrors = []error{test.markDeletingErr}, []error{test.putErr}
		first, second, third := testClock(), testClock().Add(time.Minute), testClock().Add(2*time.Minute)
		scratchDir := t.TempDir()
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: scratchDir, Clock: advancingTestClock(first, second, third)}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
		if !errors.Is(err, test.wantErr) || (test.markDeletingErr != nil && !errors.Is(err, test.markDeletingErr)) {
			t.Fatalf("%s: err=%v", test.name, err)
		}
		if len(repo.pendingInputs) != 1 || store.putCount() != 1 || len(repo.readyInputs) != 0 {
			t.Fatalf("%s: counts pending=%d put=%d ready=%d", test.name, len(repo.pendingInputs), store.putCount(), len(repo.readyInputs))
		}
		pending := repo.pendingInputs[0]
		wantLease := LeaseInput{ID: pending.ID, UserID: pending.UserID, UploadRequestID: pending.UploadRequestID, SHA256: pending.SHA256, WriteLeaseUntil: pending.WriteLeaseUntil}
		if (test.wantClear && (len(repo.clearLeaseInputs) != 1 || repo.clearLeaseInputs[0] != wantLease)) || (!test.wantClear && len(repo.clearLeaseInputs) != 0) {
			t.Fatalf("%s: lease clear=%+v pending=%+v", test.name, repo.clearLeaseInputs, pending)
		}
		wantDeleting := test.wantDelete || test.markDeletingErr != nil
		if (len(repo.markDeletingInputs) == 1) != wantDeleting {
			t.Fatalf("%s: deleting=%+v want=%v", test.name, repo.markDeletingInputs, wantDeleting)
		}
		if wantDeleting && repo.markDeletingInputs[0] != wantLease {
			t.Fatalf("%s: deleting fence=%+v want=%+v", test.name, repo.markDeletingInputs[0], wantLease)
		}
		if (store.deleteCount() == 1) != test.wantDelete || (len(repo.finalizeCalls) == 1) != test.wantFinalize {
			t.Fatalf("%s: delete=%d finalize=%d", test.name, store.deleteCount(), len(repo.finalizeCalls))
		}
		if test.wantDelete && (len(store.deleteCalls) != 1 || store.deleteCalls[0] != pending.StorageKey || store.objectCount() != 0) {
			t.Fatalf("%s: delete keys=%v objects=%d", test.name, store.deleteCalls, store.objectCount())
		}
		if test.wantFinalize && (repo.finalizeCalls[0].id != pending.ID || !repo.finalizeCalls[0].now.Equal(third)) {
			t.Fatalf("%s: finalize=%+v want id=%s now=%s", test.name, repo.finalizeCalls[0], pending.ID, third)
		}
		upload := repo.uploads["user-1/"+testRequestID]
		wantState := UploadPending
		if test.wantFinalize {
			wantState = UploadExpired
		}
		if upload.State != wantState {
			t.Fatalf("%s: upload state=%s want=%s", test.name, upload.State, wantState)
		}
		assertScratchEmpty(t, scratchDir)
	}
}

func TestUploadServiceCleanupHonorsCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	repo, store := emptyRepositoryAndStore()
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
		repo, store := emptyRepositoryAndStore()
		repo.expired, repo.claimErrors, repo.finalizeErrors, repo.compactErrors = test.rows, []error{test.claim}, []error{test.final}, []error{test.compact}
		store.deleteErrors = test.deleteErrors
		err := newUploadService(t, repo, store, UploadConfig{CleanupBatch: batch}).CleanupExpired(context.Background(), now)
		if test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) || test.wantCause != nil && !errors.Is(err, test.wantCause) || test.final != nil && !errors.Is(err, test.final) || test.compact != nil && !errors.Is(err, test.compact) {
			t.Fatalf("%s: cleanup=%v want=%v", test.name, err, test.want)
		}
		assertCleanup(t, repo, store, now, batch, test.keys, test.ids, test.states, test.doCompact)
	}
}

func TestUploadServiceSpoolCapIncludesBlockedPutsAndCanceledWaiter(t *testing.T) {
	dir := t.TempDir()
	repo, store := emptyRepositoryAndStore()
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
	repo, store := emptyRepositoryAndStore()
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
		repo, store := emptyRepositoryAndStore()
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
		repo, store := emptyRepositoryAndStore()
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
		repo, store := emptyRepositoryAndStore()
		scratchDir := t.TempDir()
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: scratchDir}).PrepareImageUpload(context.Background(), "user-1", testReceiverWithID(body, requestID))
		if !errors.Is(err, ErrInvalidUploadRequest) {
			t.Fatalf("requestID=%q err=%v", requestID, err)
		}
		assertNoPublication(t, repo, store, scratchDir)
	}
}

func TestPrepareImageUploadReportsScratchErrors(t *testing.T) {
	repo, store := emptyRepositoryAndStore()
	_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: filepath.Join(t.TempDir(), "missing")}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if !errors.Is(err, ErrMediaStorageUnavailable) || !strings.Contains(err.Error(), "create image scratch file") {
		t.Fatalf("create error=%v", err)
	}
	repo, store = emptyRepositoryAndStore()
	dir := t.TempDir()
	var removeErr error
	store.putHook = func(input PutObject) { removeErr = os.Remove(input.Body.(*os.File).Name()) }
	_, err = newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", testReceiver(testJPEG(t, 10, 10)))
	if removeErr != nil || !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("cleanup hook=%v prepare=%v", removeErr, err)
	}
	assertScratchEmpty(t, dir)
}
func emptyRepositoryAndStore() (*fakeUploadRepository, *fakeObjectStore) {
	return &fakeUploadRepository{clock: func() time.Time { return testClock() }, uploads: map[string]Upload{}}, &fakeObjectStore{objects: map[ObjectKey]fakeStoredObject{}}
}
func newUploadService(t *testing.T, repo *fakeUploadRepository, store *fakeObjectStore, config UploadConfig) *UploadService {
	if config.ScratchDir == "" {
		config.ScratchDir = t.TempDir()
	}
	service, err := NewUploadService(repo, store, config)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func testReceiver(body []byte) UploadReceiver { return testReceiverWithID(body, testRequestID) }
func testReceiverWithID(body []byte, requestID string) UploadReceiver {
	return func(_ context.Context, writer io.Writer) (string, error) {
		_, err := writer.Write(body)
		return requestID, err
	}
}

type observedContext struct {
	context.Context
	evaluated chan struct{}
	once      sync.Once
}

func (ctx *observedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.evaluated) })
	return ctx.Context.Done()
}

type prepareResult struct {
	index int
	err   error
}

func waitPrepareResult(t *testing.T, results <-chan prepareResult) prepareResult {
	select {
	case result := <-results:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prepare")
		return prepareResult{}
	}
}
func cleanupRow(id string, key ObjectKey) Upload {
	return Upload{ID: id, StorageKey: key, State: UploadDeleting}
}
func assertCleanup(t *testing.T, repo *fakeUploadRepository, store *fakeObjectStore, now time.Time, limit int, keys []ObjectKey, ids []string, states []UploadState, compact bool) {
	if len(repo.claimCalls) != 1 || repo.claimCalls[0] != (repositoryClaimCall{now: now, limit: limit}) || len(store.deleteCalls) != len(keys) || store.objectCount() != 0 {
		t.Fatalf("deletes=%v objects=%d want keys=%v", store.deleteCalls, store.objectCount(), keys)
	}
	for index, key := range keys {
		if store.deleteCalls[index] != key {
			t.Fatalf("delete[%d]=%q want=%q", index, store.deleteCalls[index], key)
		}
	}
	if len(repo.finalizeCalls) != len(ids) {
		t.Fatalf("finalizes=%v want ids=%v", repo.finalizeCalls, ids)
	}
	for index, id := range ids {
		if repo.finalizeCalls[index].id != id || !repo.finalizeCalls[index].now.Equal(now) {
			t.Fatalf("finalize[%d]=%+v want id=%s now=%s", index, repo.finalizeCalls[index], id, now)
		}
	}
	if len(repo.expired) != len(states) {
		t.Fatalf("states=%v want=%v", repo.expired, states)
	}
	for index, state := range states {
		if repo.expired[index].State != state {
			t.Fatalf("state[%d]=%s want=%s", index, repo.expired[index].State, state)
		}
	}
	if compact {
		if len(repo.compactInputs) != 1 || repo.compactInputs[0] != (repositoryCompactCall{now: now, limit: limit}) {
			t.Fatalf("compact=%v want now=%s limit=%d", repo.compactInputs, now, limit)
		}
	} else if len(repo.compactInputs) != 0 {
		t.Fatalf("unexpected compact=%v", repo.compactInputs)
	}
}
func assertScratchEmpty(t *testing.T, dir string) { assertScratchFiles(t, dir, 0) }
func assertScratchFiles(t *testing.T, dir string, want int) {
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != want {
		t.Fatalf("scratch files=%d err=%v want=%d", len(files), err, want)
	}
}
func assertNoPublication(t *testing.T, repo *fakeUploadRepository, store *fakeObjectStore, scratchDirs ...string) {
	if len(repo.uploads) != 0 || store.putCount() != 0 || store.objectCount() != 0 {
		t.Fatalf("rows=%d puts=%d objects=%d", len(repo.uploads), store.putCount(), store.objectCount())
	}
	for _, dir := range scratchDirs {
		assertScratchEmpty(t, dir)
	}
}
func waitSignal(t *testing.T, signal <-chan struct{}) {
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fake operation")
	}
}

func advancingTestClock(values ...time.Time) func() time.Time {
	return func() time.Time {
		value := values[0]
		if len(values) > 1 {
			values = values[1:]
		}
		return value
	}
}

const testRequestID = "11111111-1111-4111-8111-111111111111"

func testClock() time.Time { return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC) }
func testImage(t *testing.T, width, height int, encode func(io.Writer, image.Image) error) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buffer bytes.Buffer
	if err := encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func testJPEG(t *testing.T, width, height int) []byte {
	return testImage(t, width, height, func(writer io.Writer, img image.Image) error { return jpeg.Encode(writer, img, nil) })
}
func testPNGGray(t *testing.T, width, height int) []byte {
	img := image.NewGray(image.Rect(0, 0, width, height))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
