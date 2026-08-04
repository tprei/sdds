package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestNoteStoreUpdateNoteReindexesLexicalSearch(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "lexical-edit", Title: "Pão de queijo zuzubom",
		Body: "Busca por zuzubom encontra essa nota.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	updated, err := store.UpdateNote(ctx, note.UpdateInput{
		NoteID: created.ID, UserID: systemNoteOwnerUserID,
		Title: "Misto quente delicioso", Body: "Busca por delicioso encontra essa nota.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.Title != "Misto quente delicioso" {
		t.Fatalf("updated title = %q", updated.Title)
	}

	oldMatches, err := store.SearchNotes(ctx, note.SearchInput{Query: "zuzubom", Limit: 10, ViewerUserID: systemNoteOwnerUserID})
	if err != nil {
		t.Fatalf("search old token: %v", err)
	}
	for _, n := range oldMatches {
		if n.ID == created.ID {
			t.Fatalf("old token still matches the edited note")
		}
	}

	newMatches, err := store.SearchNotes(ctx, note.SearchInput{Query: "delicioso", Limit: 10, ViewerUserID: systemNoteOwnerUserID})
	if err != nil {
		t.Fatalf("search new token: %v", err)
	}
	found := false
	for _, n := range newMatches {
		if n.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("new token did not match the edited note")
	}
}

func TestNoteStoreUpdateNoteReplacesEmbedding(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "semantic-edit", Title: "Vetor que muda",
		Body: "A semântica segue o novo vetor após a edição.", CategorySlug: note.CategorySlugFood,
		Embedding: testEmbeddingWithVector(testVectorAt(0, 1)),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	if _, err := store.UpdateNote(ctx, note.UpdateInput{
		NoteID: created.ID, UserID: systemNoteOwnerUserID,
		Title: "Vetor que muda", Body: "A semântica segue o novo vetor após a edição.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbeddingWithVector(testVectorAt(1, 0)),
	}); err != nil {
		t.Fatalf("update note: %v", err)
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM note_embeddings WHERE note_id = ?`, created.ID).Scan(&rowCount); err != nil {
		t.Fatalf("count embeddings: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("note_embeddings rows = %d, want 1", rowCount)
	}

	scored, err := store.SearchSemantic(ctx, note.SemanticSearchInput{Vector: testVectorAt(1, 0), Limit: 10})
	if err != nil {
		t.Fatalf("search semantic: %v", err)
	}
	if len(scored) == 0 || scored[0].NoteID != created.ID {
		t.Fatalf("semantic results = %+v, want edited note first", scored)
	}
}

func TestNoteStoreUpdateNoteRejectsNonAuthor(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "owner-edit", Title: "Nota do dono",
		Body: "Só o dono pode editar.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	_, err = store.UpdateNote(ctx, note.UpdateInput{
		NoteID: created.ID, UserID: "not-the-owner",
		Title: "Tentativa alheia", Body: "Não deveria funcionar.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if !errors.Is(err, note.ErrNoteForbidden) {
		t.Fatalf("update non-owner error = %v, want ErrNoteForbidden", err)
	}

	var title string
	if err := db.QueryRowContext(ctx, `SELECT title FROM notes WHERE id = ?`, created.ID).Scan(&title); err != nil {
		t.Fatalf("read note title: %v", err)
	}
	if title != "Nota do dono" {
		t.Fatalf("note title = %q, want unchanged", title)
	}
}

func TestNoteStoreUpdateNoteBumpsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, func() time.Time { return now })

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "timestamp-edit", Title: "Carimbo de tempo",
		Body: "O updated_at avança quando edito.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	later := now.Add(time.Hour)
	store = newTestNoteStore(db, func() time.Time { return later })
	if _, err := store.UpdateNote(ctx, note.UpdateInput{
		NoteID: created.ID, UserID: systemNoteOwnerUserID,
		Title: "Carimbo de tempo editado", Body: "O updated_at avança quando edito.",
		CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	}); err != nil {
		t.Fatalf("update note: %v", err)
	}

	var createdAt, updatedAt int64
	if err := db.QueryRowContext(ctx, `SELECT created_at, updated_at FROM notes WHERE id = ?`, created.ID).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatalf("read timestamps: %v", err)
	}
	if updatedAt <= createdAt {
		t.Fatalf("updated_at = %d, created_at = %d, want updated_at > created_at", updatedAt, createdAt)
	}
}

func TestNoteStoreDeleteNoteRemovesSearchRowAndCascades(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "cascade-delete", Title: "Nota com filhos",
		Body: "Comentários, útil e imagem somem junto.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	otherUser := user.UserID("00000000-0000-7000-8000-000000000099")
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, state, created_at, updated_at) VALUES (?, 'active', 0, 0)`, otherUser); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)`, "comment-root", created.ID, otherUser, "comentário raiz", 0); err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_comments (id, note_id, user_id, body, created_at, parent_comment_id) VALUES (?, ?, ?, ?, ?, ?)`, "comment-reply", created.ID, otherUser, "resposta", 0, "comment-root"); err != nil {
		t.Fatalf("insert reply: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_useful_reactions (note_id, user_id, created_at) VALUES (?, ?, ?)`, created.ID, otherUser, 0); err != nil {
		t.Fatalf("insert useful: %v", err)
	}
	if _, err := db.ExecContext(ctx, noteImageInsertSQL, "image-child", created.ID, "note-images/child", "image/jpeg", 10, 100, 80, strings.Repeat("a", 64), 0, 0, 0); err != nil {
		t.Fatalf("insert note image: %v", err)
	}

	if err := store.DeleteNote(ctx, created.ID, systemNoteOwnerUserID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	for _, query := range []string{
		`SELECT COUNT(*) FROM note_search WHERE note_id = '` + created.ID + `'`,
		`SELECT COUNT(*) FROM note_comments WHERE note_id = '` + created.ID + `'`,
		`SELECT COUNT(*) FROM note_useful_reactions WHERE note_id = '` + created.ID + `'`,
		`SELECT COUNT(*) FROM note_images WHERE note_id = '` + created.ID + `'`,
		`SELECT COUNT(*) FROM note_embeddings WHERE note_id = '` + created.ID + `'`,
	} {
		var count int
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if count != 0 {
			t.Fatalf("query %q count = %d, want 0", query, count)
		}
	}

	if _, err := store.FindNote(ctx, created.ID, systemNoteOwnerUserID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Fatalf("find deleted note error = %v, want ErrNoteNotFound", err)
	}
}

func TestNoteStoreDeleteNoteRejectsNonAuthorAndUnknownID(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	store := newTestNoteStore(db, time.Now)

	created, err := store.CreateNote(ctx, note.CreateInput{
		ClientRequestID: "forbidden-delete", Title: "Não apaga alheia",
		Body: "Outro usuário não consegue excluir.", CategorySlug: note.CategorySlugFood, Embedding: testEmbedding(),
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := store.DeleteNote(ctx, created.ID, "not-the-owner"); !errors.Is(err, note.ErrNoteForbidden) {
		t.Fatalf("delete non-owner error = %v, want ErrNoteForbidden", err)
	}
	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notes WHERE id = ?`, created.ID).Scan(&rowCount); err != nil {
		t.Fatalf("count notes after forbidden delete: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("notes after forbidden delete = %d, want 1", rowCount)
	}

	if err := store.DeleteNote(ctx, "never-existed", systemNoteOwnerUserID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Fatalf("delete unknown note error = %v, want ErrNoteNotFound", err)
	}
}
