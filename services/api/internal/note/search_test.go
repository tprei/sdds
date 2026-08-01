package note

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/user"
)

type stubSearchStore struct {
	searchNotes    func(ctx context.Context, input SearchInput) ([]Note, error)
	searchSemantic func(ctx context.Context, input SemanticSearchInput) ([]ScoredNote, error)
	findNotesByID  func(ctx context.Context, ids []string, viewerUserID user.UserID) ([]Note, error)
}

func (stub stubSearchStore) SearchNotes(ctx context.Context, input SearchInput) ([]Note, error) {
	if stub.searchNotes == nil {
		return nil, errors.New("search notes not implemented")
	}
	return stub.searchNotes(ctx, input)
}

func (stub stubSearchStore) SearchSemantic(ctx context.Context, input SemanticSearchInput) ([]ScoredNote, error) {
	if stub.searchSemantic == nil {
		return nil, errors.New("search semantic not implemented")
	}
	return stub.searchSemantic(ctx, input)
}

func (stub stubSearchStore) FindNotesByID(ctx context.Context, ids []string, viewerUserID user.UserID) ([]Note, error) {
	if stub.findNotesByID == nil {
		return nil, errors.New("find notes by id not implemented")
	}
	return stub.findNotesByID(ctx, ids, viewerUserID)
}

// queryableEmbedder lets tests in this file override EmbedQuery without
// reusing stubEmbedder (declared in publish_test.go), since that stub always
// errors on EmbedQuery by design.
type queryableEmbedder struct {
	embedQuery func(ctx context.Context, text string) ([]float32, error)
}

func (q queryableEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return q.embedQuery(ctx, text)
}

func (q queryableEmbedder) EmbedPassages(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("unexpected embed passages")
}

func TestHybridSearcherReturnsSearchUnavailableOnEmbedderFailure(t *testing.T) {
	embedderErr := errors.New("sidecar unreachable")
	semanticCalled := false
	store := stubSearchStore{
		searchNotes: func(context.Context, SearchInput) ([]Note, error) {
			return []Note{}, nil
		},
		searchSemantic: func(context.Context, SemanticSearchInput) ([]ScoredNote, error) {
			semanticCalled = true
			return nil, nil
		},
	}
	embedder := queryableEmbedder{embedQuery: func(context.Context, string) ([]float32, error) {
		return nil, embedderErr
	}}

	searcher := NewHybridSearcher(store, embedder)
	_, err := searcher.Search(context.Background(), SearchInput{Query: "cafe", ViewerUserID: user.UserID("user-1")})
	if !errors.Is(err, ErrSearchUnavailable) {
		t.Fatalf("search error = %v, want ErrSearchUnavailable", err)
	}
	if semanticCalled {
		t.Fatal("SearchSemantic was called despite embedder failure")
	}
}

func TestHybridSearcherPassesCategorySlugToBothSources(t *testing.T) {
	var gotLexicalCategory, gotSemanticCategory CategorySlug
	store := stubSearchStore{
		searchNotes: func(_ context.Context, input SearchInput) ([]Note, error) {
			gotLexicalCategory = input.CategorySlug
			return []Note{}, nil
		},
		searchSemantic: func(_ context.Context, input SemanticSearchInput) ([]ScoredNote, error) {
			gotSemanticCategory = input.CategorySlug
			return []ScoredNote{}, nil
		},
	}
	embedder := queryableEmbedder{embedQuery: func(context.Context, string) ([]float32, error) {
		return make([]float32, EmbeddingDimension), nil
	}}

	searcher := NewHybridSearcher(store, embedder)
	_, err := searcher.Search(context.Background(), SearchInput{
		Query:        "cafe",
		CategorySlug: CategorySlug("food"),
		ViewerUserID: user.UserID("user-1"),
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotLexicalCategory != CategorySlug("food") {
		t.Fatalf("lexical category = %q, want food", gotLexicalCategory)
	}
	if gotSemanticCategory != CategorySlug("food") {
		t.Fatalf("semantic category = %q, want food", gotSemanticCategory)
	}
}

func TestHybridSearcherHydratesSemanticOnlyCandidates(t *testing.T) {
	lexicalNote := Note{ID: "lexical-note", Title: "Lexical"}
	semanticOnlyNote := Note{ID: "semantic-only-note", Title: "Semantic"}

	var hydrateRequestedIDs []string
	store := stubSearchStore{
		searchNotes: func(context.Context, SearchInput) ([]Note, error) {
			return []Note{lexicalNote}, nil
		},
		searchSemantic: func(context.Context, SemanticSearchInput) ([]ScoredNote, error) {
			return []ScoredNote{{NoteID: "semantic-only-note", Score: 0.9}}, nil
		},
		findNotesByID: func(_ context.Context, ids []string, _ user.UserID) ([]Note, error) {
			hydrateRequestedIDs = ids
			return []Note{semanticOnlyNote}, nil
		},
	}
	embedder := queryableEmbedder{embedQuery: func(context.Context, string) ([]float32, error) {
		return make([]float32, EmbeddingDimension), nil
	}}

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), SearchInput{Query: "cafe", ViewerUserID: user.UserID("user-1")})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if diff := cmp.Diff([]string{"semantic-only-note"}, hydrateRequestedIDs); diff != "" {
		t.Fatalf("hydrate request mismatch (-want +got):\n%s", diff)
	}

	found := map[string]bool{}
	for _, result := range results {
		found[result.Note.ID] = true
	}
	if !found["lexical-note"] || !found["semantic-only-note"] {
		t.Fatalf("results missing expected notes: %+v", results)
	}
}

func TestHybridSearcherDropsFusedCandidateHydrationCannotResolve(t *testing.T) {
	store := stubSearchStore{
		searchNotes: func(context.Context, SearchInput) ([]Note, error) {
			return []Note{}, nil
		},
		searchSemantic: func(context.Context, SemanticSearchInput) ([]ScoredNote, error) {
			return []ScoredNote{{NoteID: "deleted-note", Score: 0.9}}, nil
		},
		findNotesByID: func(context.Context, []string, user.UserID) ([]Note, error) {
			// The note was deleted between candidate retrieval and hydration.
			return []Note{}, nil
		},
	}
	embedder := queryableEmbedder{embedQuery: func(context.Context, string) ([]float32, error) {
		return make([]float32, EmbeddingDimension), nil
	}}

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), SearchInput{Query: "cafe", ViewerUserID: user.UserID("user-1")})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %+v, want none (unresolvable candidate dropped)", results)
	}
}

func TestHybridSearcherSkipsBothSourcesForQueryWithNoSearchableTokens(t *testing.T) {
	lexicalCalled := false
	semanticCalled := false
	embedderCalled := false
	store := stubSearchStore{
		searchNotes: func(context.Context, SearchInput) ([]Note, error) {
			lexicalCalled = true
			return []Note{}, nil
		},
		searchSemantic: func(context.Context, SemanticSearchInput) ([]ScoredNote, error) {
			semanticCalled = true
			return nil, nil
		},
	}
	embedder := queryableEmbedder{embedQuery: func(context.Context, string) ([]float32, error) {
		embedderCalled = true
		return nil, errors.New("unexpected embed query")
	}}

	searcher := NewHybridSearcher(store, embedder)
	results, err := searcher.Search(context.Background(), SearchInput{Query: "!!! *** ()", ViewerUserID: user.UserID("user-1")})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %+v, want none for a query with no letters or digits", results)
	}
	if lexicalCalled || semanticCalled || embedderCalled {
		t.Fatalf("lexical/semantic/embedder called = %v/%v/%v, want none called for an unsearchable query", lexicalCalled, semanticCalled, embedderCalled)
	}
}
