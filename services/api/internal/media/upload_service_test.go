package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"testing"
	"time"
)

type uploadTestRepository struct {
	rows                          map[string]Upload
	markReadyErr, markDeletingErr error
	cleanup                       []Upload
}
type uploadRepo = uploadTestRepository
type uploadTestStore struct {
	objects               map[ObjectKey][]byte
	putErr, deleteErr     error
	putCalls, deleteCalls int
}
type ctx = context.Context

func (repo *uploadTestRepository) FindByUserRequest(_ context.Context, userID, requestID string) (Upload, error) {
	if row, ok := repo.rows[userID+"/"+requestID]; ok {
		return row, nil
	}
	return Upload{}, ErrUploadNotFound
}
func (repo *uploadTestRepository) BeginPending(_ context.Context, input PendingInput) (Upload, error) {
	key := input.UserID + "/" + input.UploadRequestID
	if row, ok := repo.rows[key]; ok {
		row.WriteLeaseUntil, row.UpdatedAt = input.WriteLeaseUntil, input.UpdatedAt
		repo.rows[key] = row
		return row, nil
	}
	row := Upload{ID: input.ID, UserID: input.UserID, StorageKey: input.StorageKey, UploadRequestID: input.UploadRequestID, State: UploadPending, ContentType: input.ContentType, ByteSize: input.ByteSize, Width: input.Width, Height: input.Height, SHA256: input.SHA256, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt, WriteLeaseUntil: input.WriteLeaseUntil, ExpiresAt: input.ExpiresAt, RequestRetentionUntil: input.RequestRetentionUntil}
	repo.rows[key] = row
	return row, nil
}
func (repo *uploadTestRepository) MarkReady(_ context.Context, _ ReadyInput) (bool, error) {
	if repo.markReadyErr != nil {
		return false, repo.markReadyErr
	}
	for key, row := range repo.rows {
		row.State, row.WriteLeaseUntil = UploadReady, time.Time{}
		repo.rows[key] = row
		return true, nil
	}
	return false, ErrUploadNotFound
}
func (repo *uploadTestRepository) ClearLease(_ context.Context, _ LeaseInput) error  { return nil }
func (r *uploadRepo) MarkDeleting(_ ctx, _ LeaseInput) error                         { return r.markDeletingErr }
func (r *uploadRepo) ClaimExpired(_ ctx, _ time.Time, _ int) ([]Upload, error)       { return r.cleanup, nil }
func (r *uploadRepo) FinalizeExpired(_ context.Context, _ string, _ time.Time) error { return nil }
func (r *uploadRepo) CompactExpired(_ ctx, _ time.Time, _ int) (int64, error)        { return 0, nil }
func (r *uploadRepo) QuotaSnapshot(_ ctx, _ string, _ time.Time) (Quota, error)      { return Quota{}, nil }
func (store *uploadTestStore) Put(_ context.Context, input PutObject) error {
	store.putCalls++
	if store.putErr != nil {
		return store.putErr
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	if int64(len(body)) != input.Size {
		return ErrObjectIntegrity
	}
	store.objects[input.Key] = body
	return nil
}
func (store *uploadTestStore) Open(_ context.Context, _ ObjectKey) (Object, error) {
	return Object{}, ErrObjectNotFound
}
func (store *uploadTestStore) Delete(_ context.Context, key ObjectKey) error {
	store.deleteCalls++
	if store.deleteErr != nil {
		return store.deleteErr
	}
	delete(store.objects, key)
	return nil
}
func TestUploadServiceSuccessAndReplay(t *testing.T) {
	now := testClock()
	for _, tc := range []struct {
		mime string
		body []byte
	}{{"image/jpeg", testJPEG(t, 12, 8)}, {"image/png", testPNG(t, 12, 8)}} {
		repo, store := emptyRepoStore()
		receipt, err := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }}).Prepare(context.Background(), "user-1", testReceiver(tc.body))
		if err != nil || receipt.ContentType != tc.mime || receipt.ByteSize != int64(len(tc.body)) || receipt.Width != 12 || receipt.Height != 8 || len(store.objects) != 1 {
			t.Fatalf("receipt=%+v err=%v objects=%d", receipt, err, len(store.objects))
		}
	}
	body := testJPEG(t, 10, 10)
	repo, store := emptyRepoStore()
	service := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }})
	first, err := service.Prepare(context.Background(), "user-1", testReceiver(body))
	if err != nil || first.ImageUploadID == "" {
		t.Fatalf("first receipt=%+v err=%v", first, err)
	}
	puts := store.putCalls
	receipt, err := service.Prepare(context.Background(), "user-1", testReceiver(body))
	if err != nil || receipt.ImageUploadID != first.ImageUploadID || store.putCalls != puts {
		t.Fatalf("replay receipt=%+v err=%v puts=%d", receipt, err, store.putCalls)
	}
	if _, err := service.Prepare(context.Background(), "user-1", testReceiver(testPNG(t, 10, 10))); !errors.Is(err, ErrUploadIdempotencyConflict) {
		t.Fatalf("conflict=%v", err)
	}
}
func TestUploadServiceRejectsMediaBoundaries(t *testing.T) {
	now := testClock()
	for _, tc := range []struct {
		body []byte
		want error
	}{{nil, ErrInvalidMedia}, {[]byte("not an image"), ErrInvalidMedia}, {[]byte("<svg></svg>"), ErrUnsupportedMediaType}} {
		repo, store := emptyRepoStore()
		_, err := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }}).Prepare(context.Background(), "user-1", testReceiver(tc.body))
		if !errors.Is(err, tc.want) || store.putCalls != 0 || len(repo.rows) != 0 {
			t.Fatalf("err=%v puts=%d rows=%d", err, store.putCalls, len(repo.rows))
		}
	}
	repo, store := emptyRepoStore()
	_, err := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }}).Prepare(context.Background(), "user-1", testReceiver(testPNG(t, MaxImageWidth+1, 1)))
	if !errors.Is(err, ErrMediaDimensions) {
		t.Fatalf("dimensions err=%v", err)
	}
}
func TestUploadServiceSpoolCapBoundsConcurrentReceivers(t *testing.T) {
	now := testClock()
	dir := t.TempDir()
	repo, store := emptyRepoStore()
	service := testService(t, repo, store, UploadConfig{ScratchDir: dir, Clock: func() time.Time { return now }})
	entered, release, done := make(chan struct{}, 3), make(chan struct{}), make(chan error, 3)
	for range 3 {
		go func() {
			_, err := service.Prepare(context.Background(), "user-1", func(_ context.Context, writer io.Writer) (string, error) {
				if _, err := writer.Write([]byte("x")); err != nil {
					return testRequestID, err
				}
				entered <- struct{}{}
				<-release
				return "invalid-request-id", nil
			})
			done <- err
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("fewer than two receivers reached the spool")
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	if len(files) != 2 {
		close(release)
		t.Fatalf("scratch files=%d", len(files))
	}
	select {
	case <-entered:
		close(release)
		t.Fatal("third receiver bypassed cap")
	default:
	}
	close(release)
	for range 3 {
		if err := <-done; !errors.Is(err, ErrInvalidUploadRequest) {
			t.Fatalf("receiver err=%v", err)
		}
	}
	assertNoPublication(t, repo, store, dir)
}
func TestUploadServiceReceiverFailuresPublishNothing(t *testing.T) {
	now, body := testClock(), testJPEG(t, 10, 10)
	callbackErr := errors.New("receiver rejected trailing data")
	oversize := bytes.Repeat([]byte{'x'}, int(MaxEncodedImageSize)+1)
	var cancel context.CancelFunc
	cases := []struct {
		name    string
		receive UploadReceiver
		want    error
	}{
		{"callback", func(_ context.Context, writer io.Writer) (string, error) {
			_, _ = writer.Write(body)
			return "not-a-request-id", callbackErr
		}, callbackErr},
		{"oversize", func(_ context.Context, writer io.Writer) (string, error) {
			_, _ = writer.Write(oversize)
			return testRequestID, nil
		}, ErrMediaTooLarge},
		{"cancel", func(_ context.Context, writer io.Writer) (string, error) {
			_, err := writer.Write(body)
			cancel()
			return testRequestID, err
		}, context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			repo, store := emptyRepoStore()
			ctx, stop := context.WithCancel(context.Background())
			cancel = stop
			_, err := testService(t, repo, store, UploadConfig{ScratchDir: dir, Clock: func() time.Time { return now }}).Prepare(ctx, "user-1", tc.receive)
			stop()
			if !errors.Is(err, tc.want) || (tc.name == "callback" && errors.Is(err, ErrInvalidUploadRequest)) {
				t.Fatalf("err=%v", err)
			}
			assertNoPublication(t, repo, store, dir)
		})
	}
}
func TestUploadServicePersistenceFencingAndCleanup(t *testing.T) {
	now, b := testClock(), testJPEG(t, 10, 10)
	repo, store := emptyRepoStore()
	repo.markDeletingErr, store.putErr = ErrUploadStateConflict, ErrObjectIntegrity
	if _, err := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }}).Prepare(context.Background(), "user-1", testReceiver(b)); !errors.Is(err, ErrUploadStateConflict) || store.deleteCalls != 0 {
		t.Fatalf("fence err=%v deletes=%d", err, store.deleteCalls)
	}
	repo.cleanup, store.deleteErr = []Upload{{ID: "expired", StorageKey: "note-images/expired", State: UploadDeleting}}, ErrObjectUnavailable
	if err := testService(t, repo, store, UploadConfig{Clock: func() time.Time { return now }}).CleanupExpired(context.Background(), now); !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("cleanup=%v", err)
	}
}

const testRequestID = "11111111-1111-4111-8111-111111111111"

func testClock() time.Time { return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC) }
func emptyRepoStore() (*uploadTestRepository, *uploadTestStore) {
	return &uploadTestRepository{rows: map[string]Upload{}}, &uploadTestStore{objects: map[ObjectKey][]byte{}}
}
func testService(t *testing.T, repo *uploadTestRepository, store *uploadTestStore, config UploadConfig) *UploadService {
	service, err := NewUploadService(repo, store, config)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func testReceiver(body []byte) UploadReceiver {
	return func(_ context.Context, writer io.Writer) (string, error) {
		_, err := writer.Write(body)
		return testRequestID, err
	}
}
func assertNoPublication(t *testing.T, repo *uploadTestRepository, store *uploadTestStore, dir string) {
	if len(repo.rows) != 0 || store.putCalls != 0 || len(store.objects) != 0 {
		t.Fatalf("rows=%d puts=%d objects=%d", len(repo.rows), store.putCalls, len(store.objects))
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatalf("scratch files=%d", len(files))
	}
}
func testImage(t *testing.T, w, h int, encode func(io.Writer, image.Image) error) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buffer bytes.Buffer
	if err := encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func testJPEG(t *testing.T, width, height int) []byte {
	return testImage(t, width, height, func(w io.Writer, img image.Image) error { return jpeg.Encode(w, img, nil) })
}
func testPNG(t *testing.T, width, height int) []byte { return testImage(t, width, height, png.Encode) }
