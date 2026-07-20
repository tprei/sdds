package sqlite

import (
	"context"
	"errors"
	"github.com/tprei/sdds/services/api/internal/media"
	"strings"
	"testing"
	"time"
)

type imageUploadAttempt struct {
	upload media.Upload
	err    error
}

func TestImageUploadMigrationCreatesTableIndexesAndConstraints(t *testing.T) {
	fixture := newImageUploadStoreFixture(t, time.UnixMilli(1_000), "upload-user")
	ctx := fixture.ctx
	db := fixture.db
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
	valid := imageUploadInput(time.UnixMilli(1_000), "upload-1", "request-1", "upload-user", 10)
	insertImageUploadRow(t, db, valid, string(media.UploadPending), "", nil)
	duplicate := imageUploadInput(time.UnixMilli(1_000), "upload-2", "request-2", "upload-user", 10)
	insertImageUploadRow(t, db, duplicate, string(media.UploadPending), "", nil)
	insertImageUploadTestNote(t, db, "constraint-note", "upload-user")
	for name, statement := range map[string]string{
		"invalid state":             `UPDATE image_uploads SET state = 'unknown' WHERE id = 'upload-1'`,
		"invalid content type":      `UPDATE image_uploads SET content_type = 'text/plain' WHERE id = 'upload-1'`,
		"invalid digest":            `UPDATE image_uploads SET sha256 = 'BAD' WHERE id = 'upload-1'`,
		"note on non-consumed":      `UPDATE image_uploads SET state = 'ready', consumed_note_id = 'constraint-note' WHERE id = 'upload-1'`,
		"duplicate storage key":     `UPDATE image_uploads SET storage_key = 'note-images/upload-1' WHERE id = 'upload-2'`,
		"duplicate owner request":   `UPDATE image_uploads SET upload_request_id = 'request-1' WHERE id = 'upload-2'`,
		"non-positive bytes":        `UPDATE image_uploads SET byte_size = 0 WHERE id = 'upload-1'`,
		"non-positive width":        `UPDATE image_uploads SET width = 0 WHERE id = 'upload-1'`,
		"non-positive height":       `UPDATE image_uploads SET height = 0 WHERE id = 'upload-1'`,
		"invalid created timestamp": `UPDATE image_uploads SET created_at = 'invalid' WHERE id = 'upload-1'`,
		"invalid updated timestamp": `UPDATE image_uploads SET updated_at = 'invalid' WHERE id = 'upload-1'`,
		"lease at updated":          `UPDATE image_uploads SET write_lease_until = updated_at WHERE id = 'upload-1'`,
		"expiry at creation":        `UPDATE image_uploads SET expires_at = created_at WHERE id = 'upload-1'`,
		"retention at expiry":       `UPDATE image_uploads SET request_retention_until = expires_at WHERE id = 'upload-1'`,
		"empty request":             `UPDATE image_uploads SET upload_request_id = '' WHERE id = 'upload-1'`,
		"overlong request":          `UPDATE image_uploads SET upload_request_id = printf('%129s', '') WHERE id = 'upload-1'`,
		"unknown user":              `UPDATE image_uploads SET user_id = 'missing-user' WHERE id = 'upload-1'`,
	} {
		if _, err := db.ExecContext(ctx, statement); err == nil {
			t.Fatalf("%s constraint error = nil", name)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE image_uploads SET state = 'consumed', consumed_note_id = NULL WHERE id = 'upload-1'`); err != nil {
		t.Fatalf("consumed row with null note error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE image_uploads SET state = 'pending' WHERE id = 'upload-1'`); err != nil {
		t.Fatalf("restore non-consumed row error = %v", err)
	}
}
func TestImageUploadStoreReplayLeaseAndConditionalReady(t *testing.T) {
	fixture := newImageUploadStoreFixture(t, time.UnixMilli(1_000_000).UTC(), "upload-user")
	ctx := fixture.ctx
	now := fixture.now
	store := fixture.store
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
	fixture.now = fixture.now.Add(media.UploadLeaseDuration + time.Second)
	retry := input
	retry.WriteLeaseUntil = fixture.now.Add(media.UploadLeaseDuration)
	reclaimed, err := store.BeginPending(ctx, retry)
	if err != nil {
		t.Fatalf("reclaim pending: %v", err)
	}
	if reclaimed.ID != created.ID || reclaimed.StorageKey != created.StorageKey || !reclaimed.WriteLeaseUntil.After(fixture.now) {
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
	fixture := newImageUploadStoreFixture(t, time.UnixMilli(2_000_000).UTC(), "quota-user", "bytes-user")
	ctx := fixture.ctx
	db := fixture.db
	now := fixture.now
	store := fixture.store
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
	fixture := newImageUploadStoreFixture(t, time.UnixMilli(4_000_000).UTC(), "quota-user")
	ctx := fixture.ctx
	db := fixture.db
	now := fixture.now
	store := fixture.store

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
	fixture := newImageUploadStoreFixture(t, time.UnixMilli(3_000_000).UTC(), "cleanup-user")
	ctx := fixture.ctx
	db := fixture.db
	now := fixture.now
	if _, err := db.Exec(`INSERT INTO notes (id, user_id, title, body, category_slug, place_slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "cleanup-note", "cleanup-user", "Cleanup note", "body", "food", "sao-paulo", 0, 0); err != nil {
		t.Fatalf("insert cleanup note: %v", err)
	}
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
	store := fixture.store
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
func TestImageUploadStoreConcurrentClaimsPreserveAtomicLimits(t *testing.T) {
	t.Run("identical request", func(t *testing.T) {
		now := time.UnixMilli(5_000_000).UTC()
		fixture := newImageUploadStoreFixture(t, now, "concurrent-user")
		inputs := []media.PendingInput{imageUploadInput(now, "concurrent-a", "concurrent-request", "concurrent-user", 10), imageUploadInput(now, "concurrent-b", "concurrent-request", "concurrent-user", 10)}
		start := make(chan struct{})
		results := make(chan imageUploadAttempt, len(inputs))
		for _, input := range inputs {
			input := input
			go func() {
				<-start
				upload, err := fixture.store.BeginPending(fixture.ctx, input)
				results <- imageUploadAttempt{upload: upload, err: err}
			}()
		}
		close(start)
		var (
			inProgress int
			winner     media.Upload
		)
		for range inputs {
			result := <-results
			switch {
			case result.err == nil:
				winner = result.upload
			case errors.Is(result.err, media.ErrUploadInProgress):
				inProgress++
			default:
				t.Fatalf("concurrent identical request error = %v", result.err)
			}
		}
		if inProgress != 1 {
			t.Fatalf("concurrent identical request in-progress outcomes = %d", inProgress)
		}
		found, err := fixture.store.FindByUserRequest(fixture.ctx, "concurrent-user", "concurrent-request")
		if err != nil {
			t.Fatalf("find concurrent winner: %v", err)
		}
		if found.ID != winner.ID || found.StorageKey != winner.StorageKey {
			t.Fatalf("persisted winner ID/key = %q/%q, want %q/%q", found.ID, found.StorageKey, winner.ID, winner.StorageKey)
		}
	})
	t.Run("quota limit", func(t *testing.T) {
		now := time.UnixMilli(6_000_000).UTC()
		fixture := newImageUploadStoreFixture(t, now, "quota-concurrent-user")
		for index := range media.MaxLiveUploads - 1 {
			id := "quota-seed-" + string(rune('a'+index))
			insertImageUploadRow(t, fixture.db, imageUploadInput(now, id, id+"-request", "quota-concurrent-user", 1), string(media.UploadPending), "", nil)
		}
		inputs := []media.PendingInput{imageUploadInput(now, "quota-attempt-a", "quota-request-a", "quota-concurrent-user", 1), imageUploadInput(now, "quota-attempt-b", "quota-request-b", "quota-concurrent-user", 1)}
		start := make(chan struct{})
		results := make(chan error, len(inputs))
		for _, input := range inputs {
			input := input
			go func() {
				<-start
				_, err := fixture.store.BeginPending(fixture.ctx, input)
				results <- err
			}()
		}
		close(start)
		var quotaExceeded int
		for range inputs {
			switch err := <-results; {
			case err == nil:
			case errors.Is(err, media.ErrUploadQuotaExceeded):
				quotaExceeded++
			default:
				t.Fatalf("concurrent quota error = %v", err)
			}
		}
		if quotaExceeded != 1 {
			t.Fatalf("concurrent quota outcomes = %d quota exceeded", quotaExceeded)
		}
		quota, err := fixture.store.QuotaSnapshot(fixture.ctx, "quota-concurrent-user", now)
		if err != nil || quota.UserCount != media.MaxLiveUploads {
			t.Fatalf("concurrent quota = %#v, %v; want user count %d", quota, err, media.MaxLiveUploads)
		}
	})
}

func TestImageUploadStoreConcurrentReclaimKeepsSingleLease(t *testing.T) {
	now := time.UnixMilli(7_000_000).UTC()
	fixture := newImageUploadStoreFixture(t, now, "reclaim-user")
	input := imageUploadInput(now.Add(-2*time.Millisecond), "reclaim-pending", "reclaim-request", "reclaim-user", 10)
	expiredLease := now.Add(-time.Millisecond)
	insertImageUploadRow(t, fixture.db, input, string(media.UploadPending), "", &expiredLease)

	inputs := []media.PendingInput{input, input}
	start := make(chan struct{})
	results := make(chan imageUploadAttempt, len(inputs))
	for _, input := range inputs {
		input := input
		go func() {
			<-start
			upload, err := fixture.store.BeginPending(fixture.ctx, input)
			results <- imageUploadAttempt{upload: upload, err: err}
		}()
	}
	close(start)

	var (
		inProgress int
		winner     media.Upload
	)
	for range inputs {
		result := <-results
		switch {
		case result.err == nil:
			winner = result.upload
		case errors.Is(result.err, media.ErrUploadInProgress):
			inProgress++
		default:
			t.Fatalf("concurrent reclaim error = %v", result.err)
		}
	}
	if inProgress != 1 {
		t.Fatalf("concurrent reclaim in-progress outcomes = %d", inProgress)
	}

	found, err := fixture.store.FindByUserRequest(fixture.ctx, input.UserID, input.UploadRequestID)
	if err != nil {
		t.Fatalf("find reclaimed upload: %v", err)
	}
	if found.ID != winner.ID || found.State != media.UploadPending || !found.WriteLeaseUntil.Equal(winner.WriteLeaseUntil) {
		t.Fatalf("reclaimed upload = %#v, winner = %#v", found, winner)
	}
}

func TestImageUploadStoreConcurrentCleanupClaimKeepsSingleLease(t *testing.T) {
	now := time.UnixMilli(8_000_000).UTC()
	fixture := newImageUploadStoreFixture(t, now, "cleanup-claim-user")
	expired := imageUploadInput(now.Add(-media.UploadTTL-time.Millisecond), "cleanup-claim", "cleanup-claim-request", "cleanup-claim-user", 10)
	insertImageUploadRow(t, fixture.db, expired, string(media.UploadPending), "", nil)

	type claimAttempt struct {
		uploads []media.Upload
		err     error
	}
	const attempts = 2
	start := make(chan struct{})
	results := make(chan claimAttempt, attempts)
	for range attempts {
		go func() {
			<-start
			uploads, err := fixture.store.ClaimExpired(fixture.ctx, now, 1)
			results <- claimAttempt{uploads: uploads, err: err}
		}()
	}
	close(start)

	var winner media.Upload
	claims := 0
	for range attempts {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent cleanup claim: %v", result.err)
		}
		switch len(result.uploads) {
		case 0:
		case 1:
			claims++
			winner = result.uploads[0]
		default:
			t.Fatalf("cleanup claim count = %d, want at most one", len(result.uploads))
		}
	}
	if claims != 1 {
		t.Fatalf("concurrent cleanup claim outcomes = %d", claims)
	}

	found, err := fixture.store.FindByUserRequest(fixture.ctx, expired.UserID, expired.UploadRequestID)
	if err != nil {
		t.Fatalf("find cleanup claim: %v", err)
	}
	if found.ID != winner.ID || found.State != media.UploadDeleting || !found.WriteLeaseUntil.After(now) {
		t.Fatalf("cleanup claimed upload = %#v, winner = %#v", found, winner)
	}
}
