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
		`SELECT COUNT(*) FROM note_images WHERE note_id IN (SELECT id FROM notes WHERE title = 'Atomic rollback')`,
	} {
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("query rollback count: %v", err)
		}
		if count != 0 {
			t.Fatalf("rollback query %q count = %d, want 0", query, count)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })
	input := note.CreateInput{
		ClientRequestID: "note-request-concurrent", Title: "Concurrent note",
		Body: "One transaction wins and the other replays.", CategorySlug: note.CategorySlugFood,
		PlaceSlug: note.PlaceSlugSaoPaulo,
	}
	type result struct {
		note note.Note
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			<-start
			created, err := store.CreateNote(ctx, input)
			results <- result{note: created, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var IDs []string
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent create: %v", result.err)
		}
		IDs = append(IDs, result.note.ID)
	}
	if len(IDs) != 2 || IDs[0] == "" || IDs[0] != IDs[1] {
		t.Fatalf("concurrent note IDs = %#v, want two equal non-empty IDs", IDs)
	}
	var receiptCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_create_requests WHERE client_request_id = ?`, input.ClientRequestID).Scan(&receiptCount); err != nil {
		t.Fatalf("count concurrent receipts: %v", err)
	}
	var noteCount, searchCount int
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM notes), (SELECT COUNT(*) FROM note_search)`).Scan(&noteCount, &searchCount); err != nil {
		t.Fatalf("count committed note/search rows: %v", err)
	}
	if receiptCount != 1 || noteCount != 1 || searchCount != 1 {
		t.Fatalf("committed counts receipt/note/search = %d/%d/%d, want 1/1/1", receiptCount, noteCount, searchCount)
	}
}

func TestNoteCreateFingerprintFramesFieldsAndOrder(t *testing.T) {
	base := note.CreateInput{
		Title: "Title", Body: "Body", CategorySlug: note.CategorySlugFood, PlaceSlug: note.PlaceSlugSaoPaulo,
		ImageUploadIDs: []string{"upload-a", "upload-b"},
	}
	tests := []struct {
		name   string
		mutate func(*note.CreateInput)
	}{
		{"title", func(input *note.CreateInput) { input.Title = "Other title" }},
		{"body", func(input *note.CreateInput) { input.Body = "Other body" }},
		{"category", func(input *note.CreateInput) { input.CategorySlug = note.CategorySlugTravel }},
		{"place", func(input *note.CreateInput) { input.PlaceSlug = note.PlaceSlugRioDeJaneiro }},
		{"image value", func(input *note.CreateInput) { input.ImageUploadIDs[0] = "upload-c" }},
		{"image order", func(input *note.CreateInput) {
			input.ImageUploadIDs[0], input.ImageUploadIDs[1] = input.ImageUploadIDs[1], input.ImageUploadIDs[0]
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := base
			mutated.ImageUploadIDs = append([]string(nil), base.ImageUploadIDs...)
			test.mutate(&mutated)
			if noteCreateFingerprint(base) == noteCreateFingerprint(mutated) {
				t.Fatalf("%s mutation produced the same fingerprint", test.name)
			}
		})
	}
	boundary := note.CreateInput{Title: "ab", Body: "c", CategorySlug: note.CategorySlugFood, ImageUploadIDs: []string{"upload-a"}}
	if noteCreateFingerprint(boundary) == noteCreateFingerprint(note.CreateInput{Title: "a", Body: "bc", CategorySlug: note.CategorySlugFood, ImageUploadIDs: []string{"upload-a"}}) {
		t.Fatal("length-framed title/body boundary collision")
	}
	withNil := base
	withNil.ImageUploadIDs = nil
	withEmpty := base
	withEmpty.ImageUploadIDs = []string{}
	if noteCreateFingerprint(withNil) != noteCreateFingerprint(withEmpty) {
		t.Fatal("nil and empty upload ID lists should have the same fingerprint")
	}
}

func seedAssociationUpload(t *testing.T, db *sql.DB, createdAt time.Time, id, userID, state, consumedNoteID string, lease *time.Time, expiresAt time.Time) {
	t.Helper()
	input := imageUploadInput(createdAt, id, "request-"+id, userID, 10)
	input.ExpiresAt = expiresAt
	insertImageUploadRow(t, db, input, state, consumedNoteID, lease)
}
