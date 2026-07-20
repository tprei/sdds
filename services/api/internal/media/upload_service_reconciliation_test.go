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

func TestUploadServiceReconcilesLostMarkReadyResponse(t *testing.T) {
	lost := errors.New("mark-ready response lost")
	repo, store := newUploadRepositoryAndObjectStore()
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
		repo, store := newUploadRepositoryAndObjectStore()
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
		repo, store := newUploadRepositoryAndObjectStore()
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

func newUploadRepositoryAndObjectStore() (*fakeUploadRepository, *fakeObjectStore) {
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
