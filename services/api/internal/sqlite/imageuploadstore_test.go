package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tprei/sdds/services/api/internal/media"
	"strings"
	"testing"
	"time"
)

func TestImageUploadMigrationCreatesTableIndexesAndConstraints(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	for _, name := range []string{"image_uploads", "image_uploads_cleanup_idx", "image_uploads_user_idx"} {
		kind := "table"
		if strings.HasSuffix(name, "idx") {
			kind = "index"
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, kind, name).Scan(&count); err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("%s count = %d, want 1", name, count)
		}
	}
	insertImageUploadTestUser(t, db, "upload-user")
	valid := imageUploadInput(time.UnixMilli(1_000), "upload-1", "request-1", "upload-user", 10)
	insertImageUploadRow(t, db, valid, string(media.UploadPending), "", nil)
	for name, statement := range map[string]string{
		"invalid state":        `UPDATE image_uploads SET state = 'unknown' WHERE id = 'upload-1'`,
		"invalid content type": `UPDATE image_uploads SET content_type = 'text/plain' WHERE id = 'upload-1'`,
		"invalid digest":       `UPDATE image_uploads SET sha256 = 'BAD' WHERE id = 'upload-1'`,
		"note on non-consumed": `UPDATE image_uploads SET state = 'ready', consumed_note_id = 'missing-note' WHERE id = 'upload-1'`,
	} {
		if _, err := db.ExecContext(ctx, statement); err == nil {
			t.Fatalf("%s constraint error = nil", name)
		}
	}
}
func TestImageUploadStoreReplayLeaseAndConditionalReady(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertImageUploadTestUser(t, db, "upload-user")
	now := time.UnixMilli(1_000_000).UTC()
	clock := now
	store := newImageUploadStore(db, func() time.Time { return clock })
	input := imageUploadInput(now, "upload-1", "request-1", "upload-user", 100)
	created, err := store.BeginPending(ctx, input)
	if err != nil {
		t.Fatalf("begin pending: %v", err)
	}
	if created.State != media.UploadPending || created.ID != input.ID || created.StorageKey != input.StorageKey {
		t.Fatalf("created upload = %#v", created)
	}
	found, err := store.FindByUserRequest(ctx, input.UserID, input.UploadRequestID)
	if err != nil {
		t.Fatalf("find replay: %v", err)
	}
	if _, err := store.BeginPending(ctx, input); !errors.Is(err, media.ErrUploadInProgress) {
		t.Fatalf("active lease error = %v, want in-progress", err)
	}
	stale := media.LeaseInput{ID: created.ID, UserID: "wrong-user", UploadRequestID: created.UploadRequestID, SHA256: created.SHA256, WriteLeaseUntil: created.WriteLeaseUntil}
	for name, transition := range map[string]func(context.Context, media.LeaseInput) error{"clear": store.ClearLease, "delete": store.MarkDeleting} {
		if err := transition(ctx, stale); !errors.Is(err, media.ErrUploadStateConflict) {
			t.Fatalf("stale %s error = %v", name, err)
		}
	}
	past := input
	past.WriteLeaseUntil = now
	if _, err := store.BeginPending(ctx, past); !errors.Is(err, media.ErrInvalidUploadRequest) {
		t.Fatalf("past lease error = %v", err)
	}
	clock = clock.Add(media.UploadLeaseDuration + time.Second)
	retry := input
	retry.WriteLeaseUntil = clock.Add(media.UploadLeaseDuration)
	reclaimed, err := store.BeginPending(ctx, retry)
	if err != nil {
		t.Fatalf("reclaim pending: %v", err)
	}
	if reclaimed.ID != created.ID || reclaimed.StorageKey != created.StorageKey || !reclaimed.WriteLeaseUntil.After(clock) {
		t.Fatalf("reclaimed upload = %#v", reclaimed)
	}
	readyInput := media.ReadyInput{ID: reclaimed.ID, UserID: reclaimed.UserID, UploadRequestID: reclaimed.UploadRequestID, SHA256: reclaimed.SHA256, WriteLeaseUntil: reclaimed.WriteLeaseUntil}
	ready, err := store.MarkReady(ctx, readyInput)
	if err != nil || !ready {
		t.Fatalf("mark ready = %v, %v", ready, err)
	}
	ready, err = store.MarkReady(ctx, readyInput)
	if err != nil || ready {
		t.Fatalf("stale mark ready = %v, %v", ready, err)
	}
	found, err = store.FindByUserRequest(ctx, input.UserID, input.UploadRequestID)
	if err != nil {
		t.Fatalf("find ready replay: %v", err)
	}
	if found.State != media.UploadReady || !found.WriteLeaseUntil.IsZero() {
		t.Fatalf("ready upload = %#v", found)
	}
	if _, err := store.BeginPending(ctx, media.PendingInput{UserID: input.UserID, UploadRequestID: input.UploadRequestID, ContentType: input.ContentType, ByteSize: input.ByteSize, Width: input.Width, Height: input.Height, SHA256: strings.Repeat("b", 64)}); !errors.Is(err, media.ErrUploadIdempotencyConflict) {
		t.Fatalf("different content error = %v, want idempotency conflict", err)
	}
}
func TestImageUploadStoreQuotaBoundaries(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertImageUploadTestUser(t, db, "quota-user")
	insertImageUploadTestUser(t, db, "bytes-user")
	now := time.UnixMilli(2_000_000).UTC()
	store := newImageUploadStore(db, func() time.Time { return now })
	for index := 0; index < media.MaxLiveUploads; index++ {
		input := imageUploadInput(now, "count-"+string(rune('a'+index)), "count-request-"+string(rune('a'+index)), "quota-user", 1)
		if _, err := store.BeginPending(ctx, input); err != nil {
			t.Fatalf("insert live upload %d: %v", index, err)
		}
	}
	if _, err := store.BeginPending(ctx, imageUploadInput(now, "count-over", "count-request-over", "quota-user", 1)); !errors.Is(err, media.ErrUploadQuotaExceeded) {
		t.Fatalf("user quota error = %v, want quota exceeded", err)
	}
	if _, err := store.BeginPending(ctx, imageUploadInput(now, "bytes-over", "bytes-request-over", "bytes-user", 1)); err != nil {
		t.Fatalf("insert global quota row: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE image_uploads SET byte_size = ? WHERE id = 'bytes-over'`, media.MaxLiveBytes); err != nil {
		t.Fatalf("set global quota boundary: %v", err)
	}
	quota, err := store.QuotaSnapshot(ctx, "bytes-user", now)
	if err != nil {
		t.Fatalf("quota snapshot: %v", err)
	}
	wantGlobalBytes := media.MaxLiveBytes + media.MaxLiveUploads
	if quota.GlobalBytes != wantGlobalBytes {
		t.Fatalf("global bytes = %d, want %d", quota.GlobalBytes, wantGlobalBytes)
	}
	if _, err := store.BeginPending(ctx, imageUploadInput(now, "bytes-over-2", "bytes-request-over-2", "other-user", 1)); !errors.Is(err, media.ErrUploadQuotaExceeded) {
		t.Fatalf("global quota error = %v, want quota exceeded", err)
	}
}

func TestImageUploadStoreQuotaCountsUsableRows(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertImageUploadTestUser(t, db, "quota-user")
	now := time.UnixMilli(4_000_000).UTC()
	store := newImageUploadStore(db, func() time.Time { return now })

	rows := []struct {
		id        string
		createdAt time.Time
		state     media.UploadState
		byteSize  int64
		lease     *time.Time
	}{
		{id: "pending-at-now", createdAt: now.Add(-media.UploadTTL), state: media.UploadPending, byteSize: 10},
		{id: "ready-expired", createdAt: now.Add(-media.UploadTTL - time.Millisecond), state: media.UploadReady, byteSize: 20},
		{id: "pending-live", createdAt: now, state: media.UploadPending, byteSize: 30},
		{id: "ready-live", createdAt: now, state: media.UploadReady, byteSize: 40},
		{id: "deleting-expired", createdAt: now.Add(-media.UploadTTL - time.Millisecond), state: media.UploadDeleting, byteSize: 50},
	}
	for _, row := range rows {
		input := imageUploadInput(row.createdAt, row.id, row.id+"-request", "quota-user", row.byteSize)
		insertImageUploadRow(t, db, input, string(row.state), "", row.lease)
	}

	beforeBoundary, err := store.QuotaSnapshot(ctx, "quota-user", now.Add(-time.Millisecond))
	if err != nil {
		t.Fatalf("quota before expiration boundary: %v", err)
	}
	if beforeBoundary.UserCount != 4 || beforeBoundary.GlobalBytes != 130 {
		t.Fatalf("quota before expiration boundary = %#v, want count 4 and bytes 130", beforeBoundary)
	}
	atBoundary, err := store.QuotaSnapshot(ctx, "quota-user", now)
	if err != nil {
		t.Fatalf("quota at expiration boundary: %v", err)
	}
	if atBoundary.UserCount != 3 || atBoundary.GlobalBytes != 120 {
		t.Fatalf("quota at expiration boundary = %#v, want count 3 and bytes 120", atBoundary)
	}

	if _, err := store.BeginPending(ctx, imageUploadInput(now, "quota-after-expiry", "quota-after-expiry-request", "quota-user", 1)); err != nil {
		t.Fatalf("begin upload after expired rows: %v", err)
	}
	afterExpired, err := store.QuotaSnapshot(ctx, "quota-user", now)
	if err != nil {
		t.Fatalf("quota after expired rows: %v", err)
	}
	if afterExpired.UserCount != 4 || afterExpired.GlobalBytes != 121 {
		t.Fatalf("quota after expired rows = %#v, want count 4 and bytes 121", afterExpired)
	}

	deleting := imageUploadInput(now, "deleting-live", "deleting-live-request", "quota-user", 60)
	insertImageUploadRow(t, db, deleting, string(media.UploadDeleting), "", nil)
	if _, err := store.BeginPending(ctx, imageUploadInput(now, "quota-at-limit", "quota-at-limit-request", "quota-user", 1)); !errors.Is(err, media.ErrUploadQuotaExceeded) {
		t.Fatalf("quota at live row limit error = %v, want quota exceeded", err)
	}
}
func TestImageUploadStoreCleanupClaimFinalizeAndRetention(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertImageUploadTestUser(t, db, "cleanup-user")
	if _, err := db.Exec(`INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "cleanup-note", "cleanup-user", "Cleanup note", "body", "food", "sao-paulo", 0, 0); err != nil {
		t.Fatalf("insert cleanup note: %v", err)
	}
	now := time.UnixMilli(3_000_000).UTC()
	insertImageUploadRow(t, db, imageUploadInput(now.Add(-2*time.Hour), "expired-pending", "cleanup-request-1", "cleanup-user", 10), string(media.UploadPending), "", nil)
	insertImageUploadRow(t, db, imageUploadInput(now.Add(-time.Hour), "expired-ready", "cleanup-request-2", "cleanup-user", 11), string(media.UploadReady), "", nil)
	insertImageUploadRow(t, db, imageUploadInput(now.Add(-time.Hour), "expired-consumed", "cleanup-request-3", "cleanup-user", 12), string(media.UploadConsumed), "cleanup-note", nil)
	deleting := imageUploadInput(now, "deleting-retry", "cleanup-request-4", "cleanup-user", 13)
	deleting.UpdatedAt = now.Add(-2 * time.Millisecond)
	deletingLease := now.Add(-time.Millisecond)
	insertImageUploadRow(t, db, deleting, string(media.UploadDeleting), "", &deletingLease)
	if _, err := db.Exec(`UPDATE image_uploads SET expires_at = ? WHERE id IN ('expired-pending', 'expired-ready', 'expired-consumed')`, now.Add(-time.Minute).UnixMilli()); err != nil {
		t.Fatalf("age cleanup rows: %v", err)
	}
	store := newImageUploadStore(db, func() time.Time { return now })
	claimed, err := store.ClaimExpired(ctx, now, 1)
	if err != nil {
		t.Fatalf("claim expired: %v", err)
	}
	if len(claimed) != 1 || claimed[0].State != media.UploadDeleting || claimed[0].ID != "expired-pending" {
		t.Fatalf("claimed uploads = %#v", claimed)
	}
	if err := store.FinalizeExpired(ctx, claimed[0].ID, now); err != nil {
		t.Fatalf("finalize expired: %v", err)
	}
	finalized, err := store.FindByUserRequest(ctx, "cleanup-user", "cleanup-request-1")
	if err != nil {
		t.Fatalf("find finalized: %v", err)
	}
	if finalized.State != media.UploadExpired || !finalized.WriteLeaseUntil.IsZero() {
		t.Fatalf("finalized upload = %#v", finalized)
	}
	claimedRemaining, err := store.ClaimExpired(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim remaining expired: %v", err)
	}
	for _, upload := range claimedRemaining {
		if err := store.FinalizeExpired(ctx, upload.ID, now); err != nil {
			t.Fatalf("finalize remaining expired: %v", err)
		}
	}
	if quota, err := store.QuotaSnapshot(ctx, "cleanup-user", now); err != nil || quota.UserCount != 0 || quota.GlobalBytes != 0 {
		t.Fatalf("quota after cleanup = %#v, %v", quota, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = 'cleanup-note'`); err != nil {
		t.Fatalf("delete consumed note: %v", err)
	}
	consumed, err := store.FindByUserRequest(ctx, "cleanup-user", "cleanup-request-3")
	if err != nil || consumed.State != media.UploadConsumed || consumed.ConsumedNoteID != "" {
		t.Fatalf("consumed receipt after note deletion = %#v, %v", consumed, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE image_uploads SET request_retention_until = ? WHERE id IN ('expired-pending', 'expired-ready', 'expired-consumed')`, now.Add(-time.Second).UnixMilli()); err != nil {
		t.Fatalf("age retention tombstones: %v", err)
	}
	removed, err := store.CompactExpired(ctx, now, 1)
	if err != nil || removed != 1 {
		t.Fatalf("compact one tombstone = %d, %v", removed, err)
	}
	removed, err = store.CompactExpired(ctx, now, 10)
	if err != nil || removed != 2 {
		t.Fatalf("compact remaining tombstones = %d, %v", removed, err)
	}
	if _, err := store.FindByUserRequest(ctx, "cleanup-user", "cleanup-request-3"); !errors.Is(err, media.ErrUploadNotFound) {
		t.Fatalf("consumed row should compact after retention: %v", err)
	}
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
