package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
)

type imageUploadStoreFixture struct {
	ctx   context.Context
	db    *sql.DB
	now   time.Time
	store *ImageUploadStore
}

func newImageUploadStoreFixture(t *testing.T, now time.Time, userIDs ...string) *imageUploadStoreFixture {
	t.Helper()
	fixture := &imageUploadStoreFixture{ctx: context.Background(), now: now}
	fixture.db = openMigratedDatabase(t, fixture.ctx)
	for _, userID := range userIDs {
		insertImageUploadTestUser(t, fixture.db, userID)
	}
	fixture.store = newImageUploadStore(fixture.db, func() time.Time { return fixture.now })
	return fixture
}

func imageUploadInput(now time.Time, id, requestID, userID string, size int64) media.PendingInput {
	return media.PendingInput{
		ID:                    id,
		UserID:                userID,
		StorageKey:            media.ObjectKey("note-images/" + id),
		UploadRequestID:       requestID,
		ContentType:           "image/jpeg",
		ByteSize:              size,
		Width:                 100,
		Height:                100,
		SHA256:                strings.Repeat("a", 64),
		CreatedAt:             now,
		UpdatedAt:             now,
		WriteLeaseUntil:       now.Add(media.UploadLeaseDuration),
		ExpiresAt:             now.Add(media.UploadTTL),
		RequestRetentionUntil: now.Add(media.UploadRequestRetention),
	}
}

func insertImageUploadTestUser(t *testing.T, db *sql.DB, id string) {
	if _, err := db.Exec(`INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, id); err != nil {
		t.Fatalf("insert upload user: %v", err)
	}
}
func insertImageUploadTestNote(t *testing.T, db *sql.DB, id, userID string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, "Upload note", "body", "food", "sao-paulo", 0, 0); err != nil {
		t.Fatalf("insert upload note: %v", err)
	}
}

func insertImageUploadRow(t *testing.T, db *sql.DB, input media.PendingInput, state, consumedNoteID string, leaseUntil *time.Time) {
	var lease any
	if leaseUntil != nil {
		lease = leaseUntil.UnixMilli()
	}
	if _, err := db.Exec(`
		INSERT INTO image_uploads (id, user_id, storage_key, upload_request_id, state, consumed_note_id, content_type, byte_size, width, height, sha256, created_at, updated_at, write_lease_until, expires_at, request_retention_until)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.UserID, input.StorageKey, input.UploadRequestID, state, consumedNoteID,
		input.ContentType, input.ByteSize, input.Width, input.Height, input.SHA256,
		input.CreatedAt.UnixMilli(), input.UpdatedAt.UnixMilli(), lease, input.ExpiresAt.UnixMilli(), input.RequestRetentionUntil.UnixMilli()); err != nil {
		t.Fatalf("insert image upload row: %v", err)
	}
}
