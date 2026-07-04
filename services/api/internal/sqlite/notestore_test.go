package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestNoteStoreCreatesAndListsRecentNotes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
	}
	index := 0
	store := newNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	first, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	})
	if err != nil {
		t.Fatalf("create first note: %v", err)
	}

	second, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Necessaire de viagem",
		Body:         "Cabe tudo e não vaza.",
		CategorySlug: "viagem",
		CitySlug:     "rio-de-janeiro",
	})
	if err != nil {
		t.Fatalf("create second note: %v", err)
	}

	found, err := store.ListRecentNotes(ctx, 10)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("note count = %d, want 2", len(found))
	}
	gotIDs := []string{found[0].ID, found[1].ID}
	wantIDs := []string{second.ID, first.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("recent note ids mismatch (-want +got):\n%s", diff)
	}
	if found[0].CreatedAt != times[1] {
		t.Fatalf("created_at = %s, want %s", found[0].CreatedAt, times[1])
	}
}

func TestNoteStoreRespectsRecentLimit(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	store := newNoteStore(db, func() time.Time {
		return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	})

	for _, title := range []string{"Primeira nota", "Segunda nota"} {
		if _, err := store.CreateNote(ctx, note.CreateInput{
			Title:        title,
			Body:         "Um corpo de nota.",
			CategorySlug: "achadinhos",
			CitySlug:     "lisboa",
		}); err != nil {
			t.Fatalf("create note %s: %v", title, err)
		}
	}

	found, err := store.ListRecentNotes(ctx, 1)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("note count = %d, want 1", len(found))
	}
}

func TestNoteStoreListsFractionalSecondNotesInRecentOrder(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 0, 0, 100_000_000, time.UTC),
	}
	index := 0
	store := newNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	older, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Nota exata",
		Body:         "Criada no segundo exato.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	})
	if err != nil {
		t.Fatalf("create older note: %v", err)
	}

	newer, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Nota fracionada",
		Body:         "Criada um pouco depois.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	})
	if err != nil {
		t.Fatalf("create newer note: %v", err)
	}

	found, err := store.ListRecentNotes(ctx, 10)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("note count = %d, want 2", len(found))
	}
	gotIDs := []string{found[0].ID, found[1].ID}
	wantIDs := []string{newer.ID, older.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("recent note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreStoresUnixMillisecondTimestamps(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	now := time.Date(2026, 7, 2, 12, 0, 0, 123_456_789, time.UTC)
	store := newNoteStore(db, func() time.Time {
		return now
	})

	created, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: note.CategorySlugComida,
		CitySlug:     note.CitySlugSaoPaulo,
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	var storedCreatedAt int64
	var storedUpdatedAt int64
	if err := db.QueryRowContext(ctx, `SELECT created_at, updated_at FROM notes WHERE id = ?`, created.ID).Scan(&storedCreatedAt, &storedUpdatedAt); err != nil {
		t.Fatalf("query stored timestamps: %v", err)
	}
	gotStoredTimestamps := []int64{storedCreatedAt, storedUpdatedAt}
	wantStoredTimestamps := []int64{now.UnixMilli(), now.UnixMilli()}
	if diff := cmp.Diff(wantStoredTimestamps, gotStoredTimestamps); diff != "" {
		t.Fatalf("stored timestamps mismatch (-want +got):\n%s", diff)
	}
	if created.CreatedAt != time.UnixMilli(now.UnixMilli()).UTC() {
		t.Fatalf("created.CreatedAt = %s, want %s", created.CreatedAt, time.UnixMilli(now.UnixMilli()).UTC())
	}
}

func TestNoteStoreRejectsUnknownCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	store := NewNoteStore(db)
	_, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Produto bom",
		Body:         "Funcionou bem.",
		CategorySlug: "qualquer-coisa",
		CitySlug:     "sao-paulo",
	})
	if err == nil {
		t.Fatal("create note error = nil, want foreign key error")
	}
	if !strings.Contains(err.Error(), "constraint failed") {
		t.Fatalf("create note error = %q, want constraint failure", err)
	}
}

func TestNoteStoreRejectsUnknownCity(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)
	defer db.Close()

	store := NewNoteStore(db)
	_, err := store.CreateNote(ctx, note.CreateInput{
		Title:        "Produto bom",
		Body:         "Funcionou bem.",
		CategorySlug: "achadinhos",
		CitySlug:     "qualquer-lugar",
	})
	if err == nil {
		t.Fatal("create note error = nil, want foreign key error")
	}
	if !strings.Contains(err.Error(), "constraint failed") {
		t.Fatalf("create note error = %q, want constraint failure", err)
	}
}

func openMigratedDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		db.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}
