package media

import (
	"context"
	"errors"
	"testing"
)

// TestUploadServiceCleanupDrainsOrphanedObjects proves the account-purge
// orphan queue is drained by the existing cleanup sweep: each queued storage
// key has its bucket object deleted and is then forgotten.
func TestUploadServiceCleanupDrainsOrphanedObjects(t *testing.T) {
	repo, store := newUploadRepositoryAndObjectStore()
	repo.orphanedObjects = []ObjectKey{"note-images/a", "note-images/b"}
	store.objects["note-images/a"] = fakeStoredObject{body: []byte("a"), size: 1}
	store.objects["note-images/b"] = fakeStoredObject{body: []byte("b"), size: 1}

	service := newUploadService(t, repo, store, UploadConfig{CleanupBatch: 10})

	if err := service.CleanupExpired(context.Background(), testClock()); err != nil {
		t.Fatalf("cleanup expired: %v", err)
	}

	if store.deleteCount() != 2 {
		t.Fatalf("object deletes = %d, want 2", store.deleteCount())
	}
	if store.objectCount() != 0 {
		t.Fatalf("objects remaining = %d, want 0", store.objectCount())
	}
	if len(repo.orphanForgetCalls) != 2 {
		t.Fatalf("forget calls = %v, want 2", repo.orphanForgetCalls)
	}
	if len(repo.orphanedObjects) != 0 {
		t.Fatalf("queue remaining = %v, want empty", repo.orphanedObjects)
	}
}

// TestUploadServiceCleanupKeepsOrphanOnDeleteFailure proves a failed bucket
// delete leaves the key queued so the next sweep retries it.
func TestUploadServiceCleanupKeepsOrphanOnDeleteFailure(t *testing.T) {
	repo, store := newUploadRepositoryAndObjectStore()
	repo.orphanedObjects = []ObjectKey{"note-images/a"}
	store.objects["note-images/a"] = fakeStoredObject{body: []byte("a"), size: 1}
	store.deleteErrors = []error{ErrMediaStorageUnavailable, ErrMediaStorageUnavailable, ErrMediaStorageUnavailable}

	service := newUploadService(t, repo, store, UploadConfig{CleanupBatch: 10})

	err := service.CleanupExpired(context.Background(), testClock())
	if err == nil {
		t.Fatalf("cleanup expired: want error for failed object delete")
	}
	if !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("cleanup error = %v, want ErrMediaStorageUnavailable", err)
	}
	if len(repo.orphanForgetCalls) != 0 {
		t.Fatalf("forget calls = %v, want none on delete failure", repo.orphanForgetCalls)
	}
}
