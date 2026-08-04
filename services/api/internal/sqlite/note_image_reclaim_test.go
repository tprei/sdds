package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/note"
)

// TestDeleteNoteMakesConsumedUploadClaimable verifies that deleting a note
// transitions its consumed image uploads into the retryable "deleting" state
// the retention sweep already claims, instead of orphaning their object bytes.
func TestDeleteNoteMakesConsumedUploadClaimable(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.UnixMilli(9_000_000).UTC()
	store := newTestNoteStore(db, func() time.Time { return now })
	uploadStore := newImageUploadStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "reclaim-delete", Title: "Nota com imagem",
		Body: "A imagem anexada volta pra limpeza.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	upload := imageUploadInput(now, "consumed-image", "reclaim-request", string(systemNoteOwnerUserID), 10)
	insertImageUploadRow(t, db, upload, string(media.UploadConsumed), created.ID, nil)

	if err := store.DeleteNote(ctx, created.ID, systemNoteOwnerUserID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	var state string
	var lease any
	var consumedNoteID any
	if err := db.QueryRowContext(ctx,
		`SELECT state, write_lease_until, consumed_note_id FROM image_uploads WHERE id = ?`,
		upload.ID,
	).Scan(&state, &lease, &consumedNoteID); err != nil {
		t.Fatalf("read upload row: %v", err)
	}
	if state != string(media.UploadDeleting) {
		t.Fatalf("upload state = %q, want %q", state, media.UploadDeleting)
	}
	if lease != nil {
		t.Fatalf("upload write_lease_until = %v, want nil", lease)
	}
	if consumedNoteID != nil {
		t.Fatalf("upload consumed_note_id = %v, want nil", consumedNoteID)
	}

	claimed, err := uploadStore.ClaimExpired(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim expired: %v", err)
	}
	found := false
	for _, c := range claimed {
		if c.ID == upload.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("detached upload %q was not claimed by the retention sweep", upload.ID)
	}
}

// TestDeleteNoteLeavesUnrelatedConsumedUploadAlone verifies that deleting a
// note does not disturb image uploads consumed by a different note.
func TestDeleteNoteLeavesUnrelatedConsumedUploadAlone(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.UnixMilli(9_000_000).UTC()
	store := newTestNoteStore(db, func() time.Time { return now })
	uploadStore := newImageUploadStore(db, func() time.Time { return now })

	deleted, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "reclaim-delete-target", Title: "Nota excluída",
		Body: "Some.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create deleted note: %v", err)
	}
	other, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "reclaim-keep-target", Title: "Nota mantida",
		Body: "Fica.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create kept note: %v", err)
	}

	unrelated := imageUploadInput(now, "unrelated-image", "keep-request", string(systemNoteOwnerUserID), 10)
	insertImageUploadRow(t, db, unrelated, string(media.UploadConsumed), other.ID, nil)

	if err := store.DeleteNote(ctx, deleted.ID, systemNoteOwnerUserID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	var state string
	var consumedNoteID string
	if err := db.QueryRowContext(ctx,
		`SELECT state, consumed_note_id FROM image_uploads WHERE id = ?`,
		unrelated.ID,
	).Scan(&state, &consumedNoteID); err != nil {
		t.Fatalf("read unrelated upload row: %v", err)
	}
	if state != string(media.UploadConsumed) {
		t.Fatalf("unrelated upload state = %q, want %q", state, media.UploadConsumed)
	}
	if consumedNoteID != other.ID {
		t.Fatalf("unrelated upload consumed_note_id = %q, want %q", consumedNoteID, other.ID)
	}

	claimed, err := uploadStore.ClaimExpired(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim expired: %v", err)
	}
	for _, c := range claimed {
		if c.ID == unrelated.ID {
			t.Fatalf("unrelated upload %q should not be claimed", unrelated.ID)
		}
	}
}
