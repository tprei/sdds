package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
)

// recordingObjectStore is an in-memory ObjectStore that records every Delete.
// It is local to this test because media's fakeObjectStore lives in package
// media and is not importable here.
type recordingObjectStore struct {
	deleted []media.ObjectKey
	err     error
}

func (store *recordingObjectStore) Put(context.Context, media.PutObject) error { return nil }
func (store *recordingObjectStore) Open(context.Context, media.ObjectKey) (media.Object, error) {
	return media.Object{}, nil
}
func (store *recordingObjectStore) Delete(_ context.Context, key media.ObjectKey) error {
	if store.err != nil {
		return store.err
	}
	store.deleted = append(store.deleted, key)
	return nil
}

func assertUploadState(t *testing.T, ctx context.Context, db *sql.DB, id, want string) {
	t.Helper()
	var state string
	if err := db.QueryRowContext(ctx, `SELECT state FROM image_uploads WHERE id = ?`, id).Scan(&state); err != nil {
		t.Fatalf("read upload state: %v", err)
	}
	if state != want {
		t.Fatalf("upload %q state = %q, want %q", id, state, want)
	}
}

// TestDeletedNoteImageBytesAreReclaimedAndCompacted proves the full reclaim
// path for a deleted note's image: DeleteNote detaches the consumed upload into
// the retryable "deleting" state, CleanupExpired deletes the object bytes and
// finalizes the row to "expired", and a later sweep compacts the metadata once
// its retention window passes.
func TestDeletedNoteImageBytesAreReclaimedAndCompacted(t *testing.T) {
	ctx := context.Background()
	now := time.UnixMilli(9_000_000).UTC()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, func() time.Time { return now })
	uploadStore := newImageUploadStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "lifecycle-reclaim", Title: "Nota com imagem",
		Body: "A imagem anexada deve ser coletada.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	upload := imageUploadInput(now, "lifecycle-image", "lifecycle-request", string(systemNoteOwnerUserID), 10)
	insertImageUploadRow(t, db, upload, string(media.UploadConsumed), created.ID, nil)

	if err := store.DeleteNote(ctx, created.ID, systemNoteOwnerUserID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	objectStore := &recordingObjectStore{}
	service, err := media.NewUploadService(uploadStore, objectStore, media.UploadConfig{})
	if err != nil {
		t.Fatalf("create upload service: %v", err)
	}

	if err := service.CleanupExpired(ctx, now); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	if len(objectStore.deleted) != 1 || objectStore.deleted[0] != upload.StorageKey {
		t.Fatalf("deleted keys = %v, want [%s]", objectStore.deleted, upload.StorageKey)
	}
	assertUploadState(t, ctx, db, upload.ID, string(media.UploadExpired))

	// Advance past request_retention_until so CompactExpired drops the row.
	var retentionUntil int64
	if err := db.QueryRowContext(ctx, `SELECT request_retention_until FROM image_uploads WHERE id = ?`, upload.ID).Scan(&retentionUntil); err != nil {
		t.Fatalf("read retention until: %v", err)
	}
	later := time.UnixMilli(retentionUntil).Add(time.Second)
	if err := service.CleanupExpired(ctx, later); err != nil {
		t.Fatalf("compact sweep: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM image_uploads WHERE id = ?`, upload.ID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("image_uploads count after compaction = %d, want 0", count)
	}
}

// TestDeletedNoteImageBytesSurviveATransientStoreFailure proves that a
// transient object-store error does not lose the upload: the object is not
// finalized to "expired", so a later sweep that succeeds still reclaims it.
func TestDeletedNoteImageBytesSurviveATransientStoreFailure(t *testing.T) {
	ctx := context.Background()
	now := time.UnixMilli(9_000_000).UTC()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, func() time.Time { return now })
	uploadStore := newImageUploadStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "lifecycle-transient", Title: "Nota com imagem",
		Body: "Falha transitoria nao pode perder o objeto.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	upload := imageUploadInput(now, "transient-image", "transient-request", string(systemNoteOwnerUserID), 10)
	insertImageUploadRow(t, db, upload, string(media.UploadConsumed), created.ID, nil)

	if err := store.DeleteNote(ctx, created.ID, systemNoteOwnerUserID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	objectStore := &recordingObjectStore{err: media.ErrMediaStorageUnavailable}
	service, err := media.NewUploadService(uploadStore, objectStore, media.UploadConfig{})
	if err != nil {
		t.Fatalf("create upload service: %v", err)
	}

	if err := service.CleanupExpired(ctx, now); !errors.Is(err, media.ErrMediaStorageUnavailable) {
		t.Fatalf("first sweep error = %v, want ErrMediaStorageUnavailable", err)
	}
	// The object was never deleted and the row stays "deleting" so it is not lost.
	if len(objectStore.deleted) != 0 {
		t.Fatalf("deleted keys = %v, want none on failure", objectStore.deleted)
	}
	assertUploadState(t, ctx, db, upload.ID, string(media.UploadDeleting))

	// The failed claim left a lease; let it expire, clear the fault, and sweep again.
	var leaseUntil int64
	if err := db.QueryRowContext(ctx, `SELECT write_lease_until FROM image_uploads WHERE id = ?`, upload.ID).Scan(&leaseUntil); err != nil {
		t.Fatalf("read lease until: %v", err)
	}
	retryAt := time.UnixMilli(leaseUntil).Add(time.Second)
	objectStore.err = nil
	if err := service.CleanupExpired(ctx, retryAt); err != nil {
		t.Fatalf("retry sweep: %v", err)
	}
	if len(objectStore.deleted) != 1 || objectStore.deleted[0] != upload.StorageKey {
		t.Fatalf("deleted keys after retry = %v, want [%s]", objectStore.deleted, upload.StorageKey)
	}
	assertUploadState(t, ctx, db, upload.ID, string(media.UploadExpired))
}
