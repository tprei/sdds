package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestNoteStoreCreatesAndListsRecentNotes(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	first, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create first note: %v", err)
	}

	second, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Necessaire de viagem",
		Body:         "Cabe tudo e não vaza.",
		CategorySlug: "travel",
		PlaceSlug:    "rio-de-janeiro"}))
	if err != nil {
		t.Fatalf("create second note: %v", err)
	}

	found, err := store.ListRecentNotes(ctx, note.ListInput{Limit: 10})
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

func TestNoteStoreListsRecentNotesByCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 2, 0, 0, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	olderFood, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create food note: %v", err)
	}

	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Necessaire de viagem",
		Body:         "Cabe tudo e não vaza.",
		CategorySlug: "travel",
		PlaceSlug:    "rio-de-janeiro"})); err != nil {
		t.Fatalf("create travel note: %v", err)
	}

	newerFood, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Padaria boa",
		Body:         "Tem bolo simples.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create newer food note: %v", err)
	}

	found, err := store.ListRecentNotes(ctx, note.ListInput{
		CategorySlug: "food",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("list food notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{newerFood.ID, olderFood.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("recent note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreFindsNoteByID(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newTestNoteStore(db, func() time.Time {
		return now
	})

	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.FindNote(ctx, created.ID)
	if err != nil {
		t.Fatalf("find note: %v", err)
	}

	if diff := cmp.Diff(created, found); diff != "" {
		t.Fatalf("found note mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreFindsUnknownNoteAsNotFound(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	_, err := store.FindNote(ctx, "missing-note")
	if !errors.Is(err, note.ErrNoteNotFound) {
		t.Fatalf("find note error = %v, want ErrNoteNotFound", err)
	}
}

func TestNoteStoreSearchesNoteTitles(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "pão",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{created.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchesNoteBodies(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Lugar bom",
		Body:         "Tem brigadeiro decente.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "brigadeiro",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{created.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchesNotesByCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 2, 0, 0, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	olderFood, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Balcao simpatico.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create food note: %v", err)
	}

	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Balcao simpatico.",
		CategorySlug: "travel",
		PlaceSlug:    "rio-de-janeiro"})); err != nil {
		t.Fatalf("create travel note: %v", err)
	}

	newerFood, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Balcao simpatico.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create newer food note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		CategorySlug: "food",
		Query:        "balcao",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("search food notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{newerFood.ID, olderFood.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchReturnsEmptyResults(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"})); err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "necessaire",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("search note count = %d, want 0", len(found))
	}
}

func TestNoteStoreSearchRanksTitleMatchesAheadOfBodyMatches(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	titleMatch, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Brigadeiro roteiro enorme com muitas palavras extras para alongar o titulo e reduzir relevancia sem peso",
		Body:         "Docinho antigo.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create title match: %v", err)
	}

	bodyMatch, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Mesa curta",
		Body:         "Brigadeiro.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create body match: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "brigadeiro",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{titleMatch.ID, bodyMatch.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchRequiresEveryToken(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	bothTokenMatch, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Cafe com pao",
		Body:         "Padaria boa.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create both-token note: %v", err)
	}

	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Cafe honesto",
		Body:         "Abre cedo.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"})); err != nil {
		t.Fatalf("create cafe-only note: %v", err)
	}

	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Pao bom",
		Body:         "Sai quente.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"})); err != nil {
		t.Fatalf("create pao-only note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "cafe pao",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{bothTokenMatch.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchReturnsEmptyForPunctuationOnlyQuery(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café com pão de queijo",
		Body:         "Bom para trabalhar de manhã.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"})); err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "!!! *** ()",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("search note count = %d, want 0", len(found))
	}
}

func TestNoteStoreSearchOrdersTiesByRecency(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 1, 0, 0, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	older, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Um achado de bairro.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create older note: %v", err)
	}

	newer, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Um achado de bairro.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create newer note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "café",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{newer.ID, older.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchMatchesAccentedText(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "cafe",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{created.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}
}

func TestNoteStoreSearchIgnoresFTSOperatorsFromUserInput(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, time.Now)
	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Restaurante brasileiro",
		Body:         "Barato em Dublin 12.",
		CategorySlug: "food",
		PlaceSlug:    "lisboa"}))
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	found, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "restaurante *",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}

	gotIDs := noteIDs(found)
	wantIDs := []string{created.ID}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("search note ids mismatch (-want +got):\n%s", diff)
	}

	foundWithOR, err := store.SearchNotes(ctx, note.SearchInput{
		Query: "restaurante OR barato",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search notes with OR: %v", err)
	}
	if len(foundWithOR) != 0 {
		t.Fatalf("search note count with OR = %d, want 0", len(foundWithOR))
	}
}

func TestNoteStoreRespectsRecentLimit(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := newTestNoteStore(db, func() time.Time {
		return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	})

	for _, title := range []string{"Primeira nota", "Segunda nota"} {
		if _, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: title,
			Body:         "Um corpo de nota.",
			CategorySlug: "finds",
			PlaceSlug:    "lisboa"})); err != nil {
			t.Fatalf("create note %s: %v", title, err)
		}
	}

	found, err := store.ListRecentNotes(ctx, note.ListInput{Limit: 1})
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

	times := []time.Time{
		time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 12, 0, 0, 100_000_000, time.UTC),
	}
	index := 0
	store := newTestNoteStore(db, func() time.Time {
		current := times[index]
		index++
		return current
	})

	older, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Nota exata",
		Body:         "Criada no segundo exato.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create older note: %v", err)
	}

	newer, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Nota fracionada",
		Body:         "Criada um pouco depois.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo"}))
	if err != nil {
		t.Fatalf("create newer note: %v", err)
	}

	found, err := store.ListRecentNotes(ctx, note.ListInput{Limit: 10})
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

	now := time.Date(2026, 7, 2, 12, 0, 0, 123_456_789, time.UTC)
	store := newTestNoteStore(db, func() time.Time {
		return now
	})

	created, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: note.CategorySlugFood,
		PlaceSlug:    note.PlaceSlugSaoPaulo}))
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

func TestNoteStoreRejectsBothUnknownCatalogValues(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := NewNoteStore(db)
	_, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
		Title:        "Produto bom",
		Body:         "Funcionou bem.",
		CategorySlug: "qualquer-categoria",
		PlaceSlug:    "qualquer-lugar",
	}))
	requireCatalogValidationError(t, err, []note.ValidationProblem{
		{Field: "category_slug", Message: "unknown"},
		{Field: "place_slug", Message: "unknown"},
	})
	if !errors.Is(err, note.ErrCategoryNotFound) {
		t.Fatalf("create note error = %v, want category not found", err)
	}
	if !errors.Is(err, note.ErrPlaceNotFound) {
		t.Fatalf("create note error = %v, want place not found", err)
	}
}

func TestNoteStoreRejectsUnknownCategory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := NewNoteStore(db)
	_, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Produto bom",
		Body:         "Funcionou bem.",
		CategorySlug: "qualquer-coisa",
		PlaceSlug:    "sao-paulo"}))
	requireCatalogValidationError(t, err, []note.ValidationProblem{
		{Field: "category_slug", Message: "unknown"},
	})
}

func TestNoteStoreRejectsUnknownPlace(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDatabase(t, ctx)

	store := NewNoteStore(db)
	_, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{Title: "Produto bom",
		Body:         "Funcionou bem.",
		CategorySlug: "finds",
		PlaceSlug:    "qualquer-lugar"}))
	requireCatalogValidationError(t, err, []note.ValidationProblem{
		{Field: "place_slug", Message: "unknown"},
	})
}

func TestNoteStoreRejectsInactiveCatalogValues(t *testing.T) {
	tests := []struct {
		name         string
		updateSQL    string
		categorySlug note.CategorySlug
		placeSlug    note.PlaceSlug
		wantField    string
	}{
		{
			name:         "inactive category",
			updateSQL:    `UPDATE categories SET active = 0 WHERE slug = ?`,
			categorySlug: "food",
			placeSlug:    "sao-paulo",
			wantField:    "category_slug",
		},
		{
			name:         "inactive place",
			updateSQL:    `UPDATE places SET active = 0 WHERE slug = ?`,
			categorySlug: "food",
			placeSlug:    "sao-paulo",
			wantField:    "place_slug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := openMigratedDatabase(t, ctx)
			slug := tt.categorySlug
			if tt.wantField == "place_slug" {
				slug = note.CategorySlug(tt.placeSlug)
			}
			if _, err := db.ExecContext(ctx, tt.updateSQL, string(slug)); err != nil {
				t.Fatalf("deactivate catalog value: %v", err)
			}

			store := NewNoteStore(db)
			_, err := store.CreateNote(ctx, testCreateInput(note.CreateInput{
				Title:        "Produto bom",
				Body:         "Funcionou bem.",
				CategorySlug: tt.categorySlug,
				PlaceSlug:    tt.placeSlug,
			}))
			requireCatalogValidationError(t, err, []note.ValidationProblem{
				{Field: tt.wantField, Message: "unknown"},
			})
		})
	}
}

func requireCatalogValidationError(t *testing.T, err error, want []note.ValidationProblem) {
	t.Helper()

	if err == nil {
		t.Fatal("create note error = nil, want catalog validation error")
	}
	var catalogErr *note.CatalogValidationError
	if !errors.As(err, &catalogErr) {
		t.Fatalf("create note error = %v, want CatalogValidationError", err)
	}
	if diff := cmp.Diff(want, catalogErr.Problems); diff != "" {
		t.Fatalf("catalog validation problems mismatch (-want +got):\n%s", diff)
	}
}

func noteIDs(notes []note.Note) []string {
	ids := make([]string, 0, len(notes))
	for _, found := range notes {
		ids = append(ids, found.ID)
	}
	return ids
}

type testNoteStore struct {
	*NoteStore
}

func newTestNoteStore(db *sql.DB, clock func() time.Time) *testNoteStore {
	if _, err := db.Exec(`
		INSERT INTO users (id, state, created_at, updated_at)
		VALUES (?, 'active', 0, 0)
		ON CONFLICT (id) DO NOTHING`, systemNoteOwnerUserID); err != nil {
		panic(err)
	}
	if _, err := db.Exec(`
		INSERT INTO authors (id, user_id, display_name, created_at, updated_at)
		VALUES (?, ?, 'sdds', 0, 0)
		ON CONFLICT (id) DO NOTHING`, systemNoteOwnerAuthorID, systemNoteOwnerUserID); err != nil {
		panic(err)
	}
	return &testNoteStore{NoteStore: newNoteStore(db, clock)}
}

func (store *testNoteStore) CreateNote(ctx context.Context, input note.CreateInput) (note.Note, error) {
	input.UserID = systemNoteOwnerUserID
	return store.NoteStore.CreateNote(ctx, input)
}

func openMigratedDatabase(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}
