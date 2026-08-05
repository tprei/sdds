package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
)

func TestOrphanedMediaObjectsClaimAndForget(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newImageUploadStore(db, func() time.Time { return time.UnixMilli(0) })

	keys := []string{"note-images/a", "note-images/b", "note-images/c"}
	for i, key := range keys {
		if _, err := db.ExecContext(ctx, `INSERT INTO orphaned_media_objects (storage_key, orphaned_at) VALUES (?, ?)`, key, i); err != nil {
			t.Fatalf("insert orphan %s: %v", key, err)
		}
	}

	// Claim is bounded and ordered oldest-first by (orphaned_at, storage_key).
	claimed, err := store.ClaimOrphanedObjects(ctx, 2)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 2 || string(claimed[0]) != "note-images/a" || string(claimed[1]) != "note-images/b" {
		t.Fatalf("claimed = %v, want [note-images/a note-images/b]", claimed)
	}

	// A claimed key remains claimable until it is forgotten.
	again, err := store.ClaimOrphanedObjects(ctx, 2)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if len(again) != 2 {
		t.Fatalf("reclaim returned %d keys, want 2", len(again))
	}

	if err := store.ForgetOrphanedObject(ctx, media.ObjectKey("note-images/a")); err != nil {
		t.Fatalf("forget: %v", err)
	}
	remaining, err := store.ClaimOrphanedObjects(ctx, 10)
	if err != nil {
		t.Fatalf("claim remaining: %v", err)
	}
	got := make(map[string]bool, len(remaining))
	for _, key := range remaining {
		got[string(key)] = true
	}
	if got["note-images/a"] || !got["note-images/b"] || !got["note-images/c"] {
		t.Fatalf("remaining = %v, want only b and c", got)
	}
}

func TestOrphanedMediaObjectsClaimEmptyQueue(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newImageUploadStore(db, func() time.Time { return time.UnixMilli(0) })

	claimed, err := store.ClaimOrphanedObjects(ctx, 10)
	if err != nil {
		t.Fatalf("claim empty: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("claim empty = %v, want empty", claimed)
	}
}

func TestOrphanedMediaObjectsForgetMissingKey(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newImageUploadStore(db, func() time.Time { return time.UnixMilli(0) })

	if err := store.ForgetOrphanedObject(ctx, media.ObjectKey("never-queued")); err != nil {
		t.Fatalf("forget missing key: unexpected error %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM orphaned_media_objects`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("count after forget missing = %d, want 0", count)
	}
}
