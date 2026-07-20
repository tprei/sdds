package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/media"
)

func TestFindAttachedImageReturnsCommittedMetadata(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertAuthorStoreUser(t, ctx, db, authorStoreUserID, authorStoreAuthorID, "Marina Alves")
	insertAuthorStoreNote(t, ctx, db, "image-metadata-note", authorStoreUserID, noteImageTestTimestamp)
	digest := strings.Repeat("ab", 32)
	insertNoteImageTestRow(t, ctx, db, noteImageRow("image-metadata", "image-metadata-note", "image/jpeg", 42, 100, 80, 0, digest, "note-images/opaque-image"))

	got, err := NewNoteStore(db).FindAttachedImage(ctx, "image-metadata")
	if err != nil {
		t.Fatalf("FindAttachedImage error = %v", err)
	}
	wantDigest := [32]byte{}
	for index := range wantDigest {
		wantDigest[index] = 0xab
	}
	want := media.ImageMetadata{
		ID:          "image-metadata",
		StorageKey:  media.ObjectKey("note-images/opaque-image"),
		ContentType: "image/jpeg",
		ByteSize:    42,
		SHA256:      wantDigest,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("metadata mismatch (-want +got):\n%s", diff)
	}
}

func TestFindAttachedImageReturnsNotFoundForUnknownOrUnattachedID(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	insertImageUploadTestUser(t, db, "image-upload-user")
	now := noteImageTestTimestamp
	for _, test := range []struct {
		id    string
		state string
	}{
		{id: "public-pending-image", state: "pending"},
		{id: "public-ready-image", state: "ready"},
	} {
		input := imageUploadInput(time.UnixMilli(now), test.id, test.id+"-request", "image-upload-user", 42)
		insertImageUploadRow(t, db, input, test.state, "", nil)
	}

	store := NewNoteStore(db)
	for _, id := range []string{"unknown-image", "public-pending-image", "public-ready-image"} {
		_, err := store.FindAttachedImage(ctx, id)
		if !errors.Is(err, media.ErrImageNotFound) {
			t.Fatalf("FindAttachedImage(%q) error = %v, want %v", id, err, media.ErrImageNotFound)
		}
	}
}
