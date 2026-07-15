package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

const noteImageInsertSQL = `INSERT INTO note_images (
	id, note_id, storage_key, content_type, byte_size, width, height,
	sha256, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const noteImageTestTimestamp int64 = 1782993600000

func TestNoteImagesMigrationAllowsOrderedRowsAndRejectsInvalidMetadata(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "image-schema-note", authorStoreUserID, noteImageTestTimestamp)

	insertNoteImageTestRow(t, ctx, db, noteImageRow("image-one", "image-schema-note", "image/jpeg", 10, 100, 80, 0, strings.Repeat("a", 64)))
	insertNoteImageTestRow(t, ctx, db, noteImageRow("image-two", "image-schema-note", "image/png", 20, 200, 160, 1, strings.Repeat("b", 64)))

	invalidRows := []struct {
		name string
		row  noteImageTestRow
	}{
		{name: "duplicate position", row: noteImageRow("image-duplicate-position", "image-schema-note", "image/jpeg", 10, 100, 80, 0, strings.Repeat("c", 64))},
		{name: "duplicate storage key", row: noteImageRow("image-duplicate-storage", "image-schema-note", "image/jpeg", 10, 100, 80, 2, strings.Repeat("c", 64), "note-images/image-one")},
		{name: "invalid content type", row: noteImageRow("image-invalid-content-type", "image-schema-note", "text/plain", 10, 100, 80, 2, strings.Repeat("c", 64))},
		{name: "invalid size", row: noteImageRow("image-invalid-size", "image-schema-note", "image/jpeg", 0, 100, 80, 2, strings.Repeat("c", 64))},
		{name: "invalid dimensions", row: noteImageRow("image-invalid-dimensions", "image-schema-note", "image/jpeg", 10, 0, 80, 2, strings.Repeat("c", 64))},
		{name: "invalid checksum", row: noteImageRow("image-invalid-checksum", "image-schema-note", "image/jpeg", 10, 100, 80, 2, "not-a-sha256")},
		{name: "invalid position", row: noteImageRow("image-invalid-position", "image-schema-note", "image/jpeg", 10, 100, 80, -1, strings.Repeat("c", 64))},
	}
	for _, test := range invalidRows {
		t.Run(test.name, func(t *testing.T) {
			if err := insertNoteImageTestRowErr(ctx, db, test.row); err == nil {
				t.Fatal("insert note image error = nil, want constraint error")
			}
		})
	}
}

func TestNoteImagesMigrationRejectsUnknownNotes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	if err := insertNoteImageTestRowErr(ctx, db, noteImageRow("image-missing-note", "missing-note", "image/jpeg", 10, 100, 80, 0, strings.Repeat("a", 64))); err == nil {
		t.Fatal("insert image for missing note error = nil, want foreign key error")
	}
}

func TestNoteImagesMigrationRejectsInvalidStorageClasses(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "image-storage-class-note", authorStoreUserID, noteImageTestTimestamp)

	tests := []struct {
		name   string
		column string
		value  any
	}{
		{name: "id blob", column: "id", value: []byte("image-id")},
		{name: "storage key blob", column: "storage_key", value: []byte("note-images/blob")},
		{name: "content type blob", column: "content_type", value: []byte("image/jpeg")},
		{name: "sha256 blob", column: "sha256", value: []byte(strings.Repeat("a", 64))},
		{name: "byte size text", column: "byte_size", value: "10px"},
		{name: "byte size real", column: "byte_size", value: 10.5},
		{name: "byte size blob", column: "byte_size", value: []byte("10")},
		{name: "width text", column: "width", value: "100px"},
		{name: "width real", column: "width", value: 100.5},
		{name: "width blob", column: "width", value: []byte("100")},
		{name: "height text", column: "height", value: "80px"},
		{name: "height real", column: "height", value: 80.5},
		{name: "height blob", column: "height", value: []byte("80")},
		{name: "position text", column: "position", value: "2px"},
		{name: "position real", column: "position", value: 2.5},
		{name: "position blob", column: "position", value: []byte("2")},
		{name: "created at text", column: "created_at", value: "not-a-timestamp"},
		{name: "created at real", column: "created_at", value: 1782993600000.5},
		{name: "created at blob", column: "created_at", value: []byte("1782993600000")},
		{name: "updated at text", column: "updated_at", value: "not-a-timestamp"},
		{name: "updated at real", column: "updated_at", value: 1782993600000.5},
		{name: "updated at blob", column: "updated_at", value: []byte("1782993600000")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := insertNoteImageStorageClassTestRowErr(ctx, db, "image-invalid-"+strings.ReplaceAll(test.name, " ", "-"), test.column, test.value)
			if err == nil {
				t.Fatalf("insert invalid %s error = nil, want constraint error", test.column)
			}
		})
	}
}

func TestNoteStoreHydratesOrderedImagesForEveryReadPath(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Imagem de comida",
		Body:         "Um lugar com fotos.",
		CategorySlug: note.CategorySlugFood,
		PlaceSlug:    note.PlaceSlugSaoPaulo,
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	insertNoteImageTestRow(t, ctx, db, noteImageRowAt("image-position-one", created.ID, "image/png", 20, 200, 160, 1, strings.Repeat("b", 64), 1782993601000))
	insertNoteImageTestRow(t, ctx, db, noteImageRowAt("image-position-zero", created.ID, "image/jpeg", 10, 100, 80, 0, strings.Repeat("a", 64), noteImageTestTimestamp))

	wantImages := []note.Image{
		{
			ID:          "image-position-zero",
			ContentType: "image/jpeg",
			ByteSize:    10,
			Width:       100,
			Height:      80,
			Position:    0,
			CreatedAt:   time.UnixMilli(noteImageTestTimestamp).UTC(),
			UpdatedAt:   time.UnixMilli(noteImageTestTimestamp).UTC(),
		},
		{
			ID:          "image-position-one",
			ContentType: "image/png",
			ByteSize:    20,
			Width:       200,
			Height:      160,
			Position:    1,
			CreatedAt:   time.UnixMilli(1782993601000).UTC(),
			UpdatedAt:   time.UnixMilli(1782993601000).UTC(),
		},
	}

	reads := []struct {
		label string
		read  func() ([]note.Note, error)
	}{
		{"list recent notes", func() ([]note.Note, error) { return store.ListRecentNotes(ctx, note.ListInput{Limit: 10}) }},
		{"list category notes", func() ([]note.Note, error) {
			return store.ListRecentNotes(ctx, note.ListInput{CategorySlug: note.CategorySlugFood, Limit: 10})
		}},
		{"search notes", func() ([]note.Note, error) {
			return store.SearchNotes(ctx, note.SearchInput{Query: "fotos", Limit: 10})
		}},
	}
	for _, test := range reads {
		found, err := test.read()
		if err != nil {
			t.Fatalf("%s: %v", test.label, err)
		}
		assertNoteImages(t, found, created.ID, wantImages)
	}

	found, err := store.FindNote(ctx, created.ID)
	if err != nil {
		t.Fatalf("find note: %v", err)
	}
	if diff := cmp.Diff(wantImages, found.Images); diff != "" {
		t.Fatalf("detail images mismatch (-want +got):\n%s", diff)
	}

	authorPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{AuthorID: systemNoteOwnerAuthorID, Limit: 10})
	if err != nil {
		t.Fatalf("list author notes: %v", err)
	}
	if len(authorPage.Notes) != 1 {
		t.Fatalf("author note count = %d, want 1", len(authorPage.Notes))
	}
	if diff := cmp.Diff(wantImages, authorPage.Notes[0].Note.Images); diff != "" {
		t.Fatalf("author note images mismatch (-want +got):\n%s", diff)
	}

	textOnly, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Nota sem imagem",
		Body:         "Sem mídia.",
		CategorySlug: note.CategorySlugFood,
		PlaceSlug:    note.PlaceSlugSaoPaulo,
	})
	if err != nil {
		t.Fatalf("create text-only note: %v", err)
	}
	if textOnly.Images == nil {
		t.Fatal("created text-only images = nil, want non-nil empty slice")
	}
	if len(textOnly.Images) != 0 {
		t.Fatalf("created text-only image count = %d, want 0", len(textOnly.Images))
	}
}

func TestNoteStoreHydratesAuthorNotesWithoutChangingPagination(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "author-image-newest", authorStoreUserID, 3000)
	insertAuthorStoreNote(t, ctx, db, "author-image-middle", authorStoreUserID, 2000)
	insertAuthorStoreNote(t, ctx, db, "author-text-oldest", authorStoreUserID, 1000)
	insertNoteImageTestRow(t, ctx, db, noteImageRow("author-image-newest-media", "author-image-newest", "image/jpeg", 5, 10, 20, 0, strings.Repeat("d", 64)))
	insertNoteImageTestRow(t, ctx, db, noteImageRow("author-image-middle-media", "author-image-middle", "image/png", 6, 11, 21, 0, strings.Repeat("e", 64)))

	store := NewNoteStore(db)
	firstPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{AuthorID: authorStoreAuthorID, Limit: 2})
	if err != nil {
		t.Fatalf("list first author page: %v", err)
	}
	if !firstPage.HasMore || len(firstPage.Notes) != 2 {
		t.Fatalf("first author page = %#v, want two notes and HasMore", firstPage)
	}
	for index, id := range []string{
		"author-image-newest-media",
		"author-image-middle-media",
	} {
		images := firstPage.Notes[index].Note.Images
		if len(images) != 1 || images[0].ID != id {
			t.Fatalf("note %s images = %#v", id, images)
		}
	}

	secondPage, err := store.ListAuthorNotes(ctx, note.AuthorNotesInput{
		AuthorID: authorStoreAuthorID,
		Limit:    2,
		After:    &firstPage.Notes[1].Position,
	})
	if err != nil {
		t.Fatalf("list second author page: %v", err)
	}
	if secondPage.HasMore || len(secondPage.Notes) != 1 {
		t.Fatalf("second author page = %#v, want one note without HasMore", secondPage)
	}
	if secondPage.Notes[0].Note.Images == nil || len(secondPage.Notes[0].Note.Images) != 0 {
		t.Fatalf("oldest note images = %#v, want non-nil empty", secondPage.Notes[0].Note.Images)
	}
}

type noteImageTestRow struct {
	ID          string
	NoteID      string
	StorageKey  string
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
	SHA256      string
	Position    int
	CreatedAt   int64
	UpdatedAt   int64
}

func noteImageRow(id, noteID, contentType string, byteSize int64, width, height, position int, sha256 string, storageKey ...string) noteImageTestRow {
	row := noteImageTestRow{ID: id, NoteID: noteID, StorageKey: "note-images/" + id, ContentType: contentType, ByteSize: byteSize, Width: width, Height: height, SHA256: sha256, Position: position, CreatedAt: noteImageTestTimestamp, UpdatedAt: noteImageTestTimestamp}
	if len(storageKey) > 0 {
		row.StorageKey = storageKey[0]
	}
	return row
}

func noteImageRowAt(id, noteID, contentType string, byteSize int64, width, height, position int, sha256 string, createdAt int64) noteImageTestRow {
	row := noteImageRow(id, noteID, contentType, byteSize, width, height, position, sha256)
	row.CreatedAt, row.UpdatedAt = createdAt, createdAt
	return row
}

func insertNoteImageTestRow(t *testing.T, ctx context.Context, db *sql.DB, row noteImageTestRow) {
	if err := insertNoteImageTestRowErr(ctx, db, row); err != nil {
		t.Fatalf("insert note image %s: %v", row.ID, err)
	}
}

func insertNoteImageTestRowErr(ctx context.Context, db *sql.DB, row noteImageTestRow) error {
	_, err := db.ExecContext(ctx, noteImageInsertSQL, row.ID, row.NoteID, row.StorageKey, row.ContentType, row.ByteSize, row.Width, row.Height, row.SHA256, row.Position, row.CreatedAt, row.UpdatedAt)
	return err
}

func insertNoteImageStorageClassTestRowErr(ctx context.Context, db *sql.DB, id, column string, value any) error {
	values := []any{id, "image-storage-class-note", "note-images/" + id, "image/jpeg", int64(10), 100, 80, strings.Repeat("a", 64), 2, noteImageTestTimestamp, noteImageTestTimestamp}
	index, ok := map[string]int{
		"id": 0, "storage_key": 2, "content_type": 3, "byte_size": 4, "width": 5,
		"height": 6, "sha256": 7, "position": 8, "created_at": 9, "updated_at": 10,
	}[column]
	if !ok {
		return fmt.Errorf("unknown note image column %q", column)
	}
	values[index] = value
	_, err := db.ExecContext(ctx, noteImageInsertSQL, values...)
	return err
}

func assertNoteImages(t *testing.T, notes []note.Note, noteID string, want []note.Image) {
	for _, found := range notes {
		if found.ID != noteID {
			continue
		}
		if diff := cmp.Diff(want, found.Images); diff != "" {
			t.Fatalf("note %s images mismatch (-want +got):\n%s", noteID, diff)
		}
		return
	}
	t.Fatalf("note %s not found in %#v", noteID, notes)
}
