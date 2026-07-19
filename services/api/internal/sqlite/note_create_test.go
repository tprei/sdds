package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestNoteStoreAssociatesReadyImageAtomically(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })

	input := imageUploadInput(now, "upload-atomic", "upload-request-atomic", string(systemNoteOwnerUserID), 481234)
	input.ContentType = "image/png"
	input.Width, input.Height = 1200, 900
	input.SHA256 = strings.Repeat("b", 64)
	insertImageUploadRow(t, db, input, "ready", "", nil)

	command := note.CreateInput{
		ClientRequestID: "note-request-atomic", Title: "Atomic image note",
		Body: "The image is attached in the same transaction.", CategorySlug: note.CategorySlugFood,
		PlaceSlug: note.PlaceSlugSaoPaulo, ImageUploadIDs: []string{"upload-atomic"},
	}
	created, err := store.CreateNote(ctx, command)
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	wantTime := normalizeTime(now)
	wantImage := note.Image{
		ID: "upload-atomic", ContentType: "image/png", ByteSize: 481234,
		Width: 1200, Height: 900, Position: 0, CreatedAt: wantTime, UpdatedAt: wantTime,
	}
	if diff := cmp.Diff([]note.Image{wantImage}, created.Images); diff != "" {
		t.Fatalf("created images mismatch (-want +got):\n%s", diff)
	}
	replayed, err := store.CreateNote(ctx, command)
	if err != nil {
		t.Fatalf("replay consumed note: %v", err)
	}
	if replayed.ID != created.ID {
		t.Fatalf("replayed note ID = %q, want %q", replayed.ID, created.ID)
	}
	if diff := cmp.Diff(created.Images, replayed.Images); diff != "" {
		t.Fatalf("replayed images mismatch (-want +got):\n%s", diff)
	}

	var state, consumedNoteID string
	if err := db.QueryRowContext(ctx, `SELECT state, consumed_note_id FROM image_uploads WHERE id = ?`, "upload-atomic").Scan(&state, &consumedNoteID); err != nil {
		t.Fatalf("query consumed upload: %v", err)
	}
	if state != "consumed" || consumedNoteID != created.ID {
		t.Fatalf("upload state = %q/%q, want consumed/%q", state, consumedNoteID, created.ID)
	}
	var noteID, storageKey, contentType, sha256 string
	var byteSize, width, height, position, createdAt, updatedAt int64
	if err := db.QueryRowContext(ctx, `
		SELECT note_id, storage_key, content_type, byte_size, width, height, sha256, position, created_at, updated_at
		FROM note_images WHERE id = ?`, "upload-atomic").Scan(
		&noteID, &storageKey, &contentType, &byteSize, &width, &height, &sha256, &position, &createdAt, &updatedAt,
	); err != nil {
		t.Fatalf("query note image: %v", err)
	}
	if noteID != created.ID || storageKey != "note-images/upload-atomic" || contentType != "image/png" ||
		byteSize != 481234 || width != 1200 || height != 900 || sha256 != strings.Repeat("b", 64) || position != 0 ||
		createdAt != wantTime.UnixMilli() || updatedAt != wantTime.UnixMilli() {
		t.Fatalf("stored note image metadata = %q/%q/%q/%d/%d/%d/%q/%d/%d/%d", noteID, storageKey, contentType, byteSize, width, height, sha256, position, createdAt, updatedAt)
	}
}

func TestNoteStoreReplaysBeforeCatalogChecksAndConflictsChangedCommand(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	input := note.CreateInput{
		ClientRequestID: "note-request-replay",
		Title:           "Replayable note",
		Body:            "The receipt wins over catalog state.",
		CategorySlug:    note.CategorySlugFood,
		PlaceSlug:       note.PlaceSlugSaoPaulo,
	}
	first, err := store.CreateNote(ctx, input)
	if err != nil {
		t.Fatalf("create first note: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE categories SET active = 0 WHERE slug = ?`, note.CategorySlugFood); err != nil {
		t.Fatalf("deactivate category: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE places SET active = 0 WHERE slug = ?`, note.PlaceSlugSaoPaulo); err != nil {
		t.Fatalf("deactivate place: %v", err)
	}
	replayed, err := store.CreateNote(ctx, input)
	if err != nil {
		t.Fatalf("replay after catalog deactivation: %v", err)
	}
	diff := cmp.Diff(first, replayed)
	if replayed.ID != first.ID || diff != "" {
		t.Fatalf("replayed note mismatch (-want +got):\n%s", diff)
	}
	newRequest := input
	newRequest.ClientRequestID = "note-request-after-deactivation"
	_, err = store.CreateNote(ctx, newRequest)
	requireCatalogValidationError(t, err, []note.ValidationProblem{
		{Field: "category_slug", Message: "unknown"},
		{Field: "place_slug", Message: "unknown"},
	})
	input.Title = "Changed command"
	if _, err := store.CreateNote(ctx, input); !errors.Is(err, note.ErrIdempotencyConflict) {
		t.Fatalf("changed command error = %v, want idempotency conflict", err)
	}
}

func TestNoteStoreUploadAvailabilityErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, imageID string
		want          error
		setup         func(*testing.T, *sql.DB)
	}{
		{name: "missing", imageID: "upload-missing", want: note.ErrImageUploadUnavailable},
		{name: "foreign-expired", imageID: "upload-foreign-expired", want: note.ErrImageUploadUnavailable, setup: func(t *testing.T, db *sql.DB) {
			insertImageUploadTestUser(t, db, "other-user")
			seedAssociationUpload(t, db, now.Add(-2*time.Hour), "upload-foreign-expired", "other-user", "ready", "", nil, now.Add(-time.Second))
		}},
		{name: "pending", imageID: "upload-pending", want: note.ErrImageUploadUnavailable, setup: func(t *testing.T, db *sql.DB) {
			lease := now.Add(time.Minute)
			seedAssociationUpload(t, db, now, "upload-pending", string(systemNoteOwnerUserID), "pending", "", &lease, now.Add(time.Hour))
		}},
		{name: "expired", imageID: "upload-expired", want: note.ErrImageUploadExpired, setup: func(t *testing.T, db *sql.DB) {
			seedAssociationUpload(t, db, now.Add(-2*time.Hour), "upload-expired", string(systemNoteOwnerUserID), "ready", "", nil, now.Add(-time.Second))
		}},
		{name: "exact-expiry", imageID: "upload-exact-expiry", want: note.ErrImageUploadExpired, setup: func(t *testing.T, db *sql.DB) {
			seedAssociationUpload(t, db, now.Add(-2*time.Hour), "upload-exact-expiry", string(systemNoteOwnerUserID), "ready", "", nil, now)
		}},
		{name: "one-ms-after-expiry", imageID: "upload-one-ms", setup: func(t *testing.T, db *sql.DB) {
			seedAssociationUpload(t, db, now.Add(-2*time.Hour), "upload-one-ms", string(systemNoteOwnerUserID), "ready", "", nil, now.Add(time.Millisecond))
		}},
		{name: "consumed", imageID: "upload-consumed", want: note.ErrImageUploadUnavailable, setup: func(t *testing.T, db *sql.DB) {
			insertAuthorStoreNote(t, ctx, db, "already-consumed-note", systemNoteOwnerUserID, now.UnixMilli())
			seedAssociationUpload(t, db, now, "upload-consumed", string(systemNoteOwnerUserID), "consumed", "already-consumed-note", nil, now.Add(time.Hour))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openMigratedDatabase(t, ctx)
			store := newTestNoteStore(db, func() time.Time { return now })
			if test.setup != nil {
				test.setup(t, db)
			}
			_, err := store.CreateNote(ctx, note.CreateInput{
				ClientRequestID: "request-" + test.name, Title: "Upload state note",
				Body: "Association must reject unavailable media.", CategorySlug: note.CategorySlugFood,
				PlaceSlug: note.PlaceSlugSaoPaulo, ImageUploadIDs: []string{test.imageID},
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("create error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNoteStoreAssociationRollbackRestoresAllRows(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	insertAuthorStoreNote(t, ctx, db, "blocking-note", systemNoteOwnerUserID, now.UnixMilli())
	insertNoteImageTestRow(t, ctx, db, noteImageRow("upload-rollback", "blocking-note", "image/jpeg", 10, 100, 80, 0, strings.Repeat("c", 64), "note-images/blocking"))
	input := imageUploadInput(now, "upload-rollback", "upload-request-rollback", string(systemNoteOwnerUserID), 10)
	insertImageUploadRow(t, db, input, "ready", "", nil)

	_, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "note-request-rollback",
		Title:           "Atomic rollback",
		Body:            "The existing image ID forces association failure.",
		CategorySlug:    note.CategorySlugFood,
		PlaceSlug:       note.PlaceSlugSaoPaulo,
		ImageUploadIDs:  []string{"upload-rollback"},
	})
	if err == nil {
		t.Fatal("create note error = nil, want association failure")
	}
	var count int
	for _, query := range []string{
		`SELECT COUNT(*) FROM notes WHERE title = 'Atomic rollback'`,
		`SELECT COUNT(*) FROM note_search WHERE title = 'Atomic rollback'`,
		`SELECT COUNT(*) FROM note_create_requests WHERE client_request_id = 'note-request-rollback'`,
	} {
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("query rollback count: %v", err)
		}
		if count != 0 {
			t.Fatalf("rollback query %q count = %d, want 0", query, count)
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_images WHERE id = ? AND note_id = ?`, "upload-rollback", "blocking-note").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback note image count/error = %d/%v, want 1/nil", count, err)
	}
	var state, consumedNoteID string
	if err := db.QueryRowContext(ctx, `SELECT state, COALESCE(consumed_note_id, '') FROM image_uploads WHERE id = ?`, "upload-rollback").Scan(&state, &consumedNoteID); err != nil {
		t.Fatalf("query rollback upload: %v", err)
	}
	if state != "ready" || consumedNoteID != "" {
		t.Fatalf("rollback upload = %q/%q, want ready/empty", state, consumedNoteID)
	}
}

func TestNoteStoreConcurrentIdenticalCreatesConvergeOnOneReceipt(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct{ name, imageID string }{
		{name: "text only"},
		{name: "one image", imageID: "upload-concurrent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			db := openMigratedDatabase(t, ctx)
			store := newTestNoteStore(db, func() time.Time { return now })
			if test.imageID != "" {
				seedAssociationUpload(t, db, now, test.imageID, string(systemNoteOwnerUserID), "ready", "", nil, now.Add(time.Hour))
			}
			input := note.CreateInput{
				ClientRequestID: "note-request-concurrent-" + test.name, Title: "Concurrent note",
				Body: "One transaction wins and the other replays.", CategorySlug: note.CategorySlugFood,
				PlaceSlug: note.PlaceSlugSaoPaulo,
			}
			if test.imageID != "" {
				input.ImageUploadIDs = []string{test.imageID}
			}

			results := createNotesConcurrently(ctx, store, input, input)
			for _, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent create: %v", result.err)
				}
			}
			if len(results) != 2 || results[0].created.ID == "" || results[0].created.ID != results[1].created.ID {
				t.Fatalf("concurrent notes = %#v, want two equal non-empty IDs", results)
			}
			if diff := cmp.Diff(results[0].created, results[1].created); diff != "" {
				t.Fatalf("concurrent note mismatch (-want +got):\n%s", diff)
			}

			wantImages := 0
			if test.imageID != "" {
				wantImages = 1
			}
			assertCommittedCreateCounts(t, ctx, db, wantImages)
			if test.imageID != "" {
				var state, consumedNoteID string
				if err := db.QueryRowContext(ctx, `SELECT state, consumed_note_id FROM image_uploads WHERE id = ?`, test.imageID).Scan(&state, &consumedNoteID); err != nil {
					t.Fatalf("read concurrent upload: %v", err)
				}
				if state != "consumed" || consumedNoteID != results[0].created.ID {
					t.Fatalf("concurrent upload = %q/%q, want consumed/%q", state, consumedNoteID, results[0].created.ID)
				}
			}
		})
	}
}

func TestNoteStoreReconcilesCommittedReceiptPrimaryKeyRace(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	input := note.CreateInput{
		UserID: systemNoteOwnerUserID, ClientRequestID: "note-request-receipt-primary-key", Title: "Receipt primary key",
		Body: "A committed receipt wins after an explicit rollback.", CategorySlug: note.CategorySlugFood,
		PlaceSlug: note.PlaceSlugSaoPaulo,
	}
	winner, err := store.CreateNote(ctx, input)
	if err != nil {
		t.Fatalf("create receipt winner: %v", err)
	}
	insertAuthorStoreNote(t, ctx, db, "receipt-primary-key-loser", systemNoteOwnerUserID, now.UnixMilli())
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin duplicate receipt transaction: %v", err)
	}
	var receiptRace noteCreateReceiptRace
	if err := insertNoteCreateRequest(ctx, tx, input, note.CreateRequestFingerprint(input), "receipt-primary-key-loser", now); !errors.As(err, &receiptRace) {
		t.Fatalf("duplicate receipt error = %v, want noteCreateReceiptRace", err)
	}
	var sqliteErr *modernsqlite.Error
	if !errors.As(receiptRace.insertErr, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
		t.Fatalf("receipt insert error = %v, want PRIMARYKEY constraint", receiptRace.insertErr)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback duplicate receipt transaction: %v", err)
	}
	replayed, err := store.reconcileNoteCreateReceipt(ctx, input, note.CreateRequestFingerprint(input), receiptRace.insertErr)
	if err != nil {
		t.Fatalf("reconcile same receipt: %v", err)
	}
	if replayed.ID != winner.ID {
		t.Fatalf("replayed note ID = %q, want %q", replayed.ID, winner.ID)
	}
	different := input
	different.Title = "Different receipt command"
	if _, err := store.reconcileNoteCreateReceipt(ctx, different, note.CreateRequestFingerprint(different), receiptRace.insertErr); !errors.Is(err, note.ErrIdempotencyConflict) {
		t.Fatalf("reconcile changed receipt error = %v, want idempotency conflict", err)
	}
}

func TestNoteStoreConcurrentRequestsCompeteForReadyImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	const imageID = "upload-competing-requests"
	seedAssociationUpload(t, db, now, imageID, string(systemNoteOwnerUserID), "ready", "", nil, now.Add(time.Hour))
	input := note.CreateInput{
		ClientRequestID: "note-request-competing-a", Title: "Competing image note",
		Body: "Only one request may consume the image.", CategorySlug: note.CategorySlugFood,
		PlaceSlug: note.PlaceSlugSaoPaulo, ImageUploadIDs: []string{imageID},
	}
	other := input
	other.ClientRequestID = "note-request-competing-b"

	var winner noteCreateResult
	successes, unavailable := 0, 0
	for _, result := range createNotesConcurrently(ctx, store, input, other) {
		switch {
		case result.err == nil:
			successes++
			winner = result
		case errors.Is(result.err, note.ErrImageUploadUnavailable):
			unavailable++
		default:
			t.Fatalf("competing create error = %v", result.err)
		}
	}
	if successes != 1 || unavailable != 1 {
		t.Fatalf("competing outcomes success/unavailable = %d/%d, want 1/1", successes, unavailable)
	}
	if images := winner.created.Images; len(images) != 1 || images[0].ID != imageID || images[0].Position != 0 {
		t.Fatalf("winning images = %#v, want %q at position 0", images, imageID)
	}
	var receiptNoteID string
	if err := db.QueryRowContext(ctx, `SELECT note_id FROM note_create_requests WHERE user_id = ? AND client_request_id = ?`, systemNoteOwnerUserID, winner.clientRequestID).Scan(&receiptNoteID); err != nil {
		t.Fatalf("query winning receipt: %v", err)
	}
	if receiptNoteID != winner.created.ID {
		t.Fatalf("receipt note ID = %q, want %q", receiptNoteID, winner.created.ID)
	}
	var state, consumedNoteID string
	if err := db.QueryRowContext(ctx, `SELECT state, consumed_note_id FROM image_uploads WHERE id = ?`, imageID).Scan(&state, &consumedNoteID); err != nil {
		t.Fatalf("query winning upload: %v", err)
	}
	if state != "consumed" || consumedNoteID != winner.created.ID {
		t.Fatalf("winning upload = %q/%q, want consumed/%q", state, consumedNoteID, winner.created.ID)
	}
	assertCommittedCreateCounts(t, ctx, db, 1)
}

func TestNoteStoreReceiptFailureRollsBackCompletedAssociation(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	const imageID = "upload-receipt-rollback"
	seedAssociationUpload(t, db, now, imageID, string(systemNoteOwnerUserID), "ready", "", nil, now.Add(time.Hour))
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER abort_note_create_request BEFORE INSERT ON note_create_requests BEGIN SELECT RAISE(ABORT, 'receipt insert aborted'); END`); err != nil {
		t.Fatalf("create receipt trigger: %v", err)
	}

	_, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "note-request-receipt-rollback",
		Title:           "Receipt rollback",
		Body:            "The receipt failure must roll back every earlier write.",
		CategorySlug:    note.CategorySlugFood,
		PlaceSlug:       note.PlaceSlugSaoPaulo,
		ImageUploadIDs:  []string{imageID},
	})
	if err == nil || !strings.Contains(err.Error(), "receipt insert aborted") {
		t.Fatalf("create error = %v, want receipt trigger error", err)
	}
	var count int
	for _, query := range []string{
		`SELECT COUNT(*) FROM notes WHERE title = 'Receipt rollback'`,
		`SELECT COUNT(*) FROM note_search WHERE title = 'Receipt rollback'`,
		`SELECT COUNT(*) FROM note_create_requests WHERE client_request_id = 'note-request-receipt-rollback'`,
		`SELECT COUNT(*) FROM note_images WHERE id = 'upload-receipt-rollback'`,
	} {
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("query receipt rollback count: %v", err)
		}
		if count != 0 {
			t.Fatalf("receipt rollback query %q count = %d, want 0", query, count)
		}
	}
	var state, consumedNoteID string
	if err := db.QueryRowContext(ctx, `SELECT state, COALESCE(consumed_note_id, '') FROM image_uploads WHERE id = ?`, imageID).Scan(&state, &consumedNoteID); err != nil {
		t.Fatalf("query receipt rollback upload: %v", err)
	}
	if state != "ready" || consumedNoteID != "" {
		t.Fatalf("receipt rollback upload = %q/%q, want ready/empty", state, consumedNoteID)
	}
}

type noteCreateResult struct {
	created         note.Note
	clientRequestID string
	err             error
}

func createNotesConcurrently(ctx context.Context, store *testNoteStore, inputs ...note.CreateInput) []noteCreateResult {
	start := make(chan struct{})
	results := make(chan noteCreateResult, len(inputs))
	var wait sync.WaitGroup
	wait.Add(len(inputs))
	for _, input := range inputs {
		input := input
		go func() {
			defer wait.Done()
			<-start
			created, err := store.CreateNote(ctx, input)
			results <- noteCreateResult{created: created, clientRequestID: input.ClientRequestID, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	created := make([]noteCreateResult, 0, len(inputs))
	for result := range results {
		created = append(created, result)
	}
	return created
}

func assertCommittedCreateCounts(t *testing.T, ctx context.Context, db *sql.DB, wantImages int) {
	t.Helper()
	var receipts, notes, search, images int
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM note_create_requests), (SELECT COUNT(*) FROM notes), (SELECT COUNT(*) FROM note_search), (SELECT COUNT(*) FROM note_images)`).Scan(&receipts, &notes, &search, &images); err != nil {
		t.Fatalf("count committed create rows: %v", err)
	}
	if receipts != 1 || notes != 1 || search != 1 || images != wantImages {
		t.Fatalf("committed counts receipt/note/search/image = %d/%d/%d/%d, want 1/1/1/%d", receipts, notes, search, images, wantImages)
	}
}

func seedAssociationUpload(t *testing.T, db *sql.DB, createdAt time.Time, id, userID, state, consumedNoteID string, lease *time.Time, expiresAt time.Time) {
	t.Helper()
	input := imageUploadInput(createdAt, id, "request-"+id, userID, 10)
	input.ExpiresAt = expiresAt
	insertImageUploadRow(t, db, input, state, consumedNoteID, lease)
}
