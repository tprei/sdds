package note

import (
	"context"
	"errors"
	"fmt"

	"github.com/tprei/sdds/services/api/internal/user"
)

// ErrSearchUnavailable means the embedding runtime could not embed the query.
// There is no fallback to lexical-only search: the whole search aborts.
var ErrSearchUnavailable = errors.New("search unavailable")

// SearchStore is the narrow persistence seam HybridSearcher needs: lexical
// retrieval, semantic retrieval, and batch hydration by id. It carries no
// knowledge of SQL, vector bytes, or cosine arithmetic.
type SearchStore interface {
	SearchNotes(ctx context.Context, input SearchInput) ([]Note, error)
	SearchSemantic(ctx context.Context, input SemanticSearchInput) ([]ScoredNote, error)
	FindNotesByID(ctx context.Context, ids []string, viewerUserID user.UserID) ([]Note, error)
}

// HybridSearcher runs FTS5 lexical retrieval and exact-KNN semantic
// retrieval for every search, fuses the two ranked candidate lists with
// reciprocal-rank fusion, and hydrates the result. It has no knowledge of
// either candidate source's internals.
type HybridSearcher struct {
	store    SearchStore
	embedder Embedder
}

func NewHybridSearcher(store SearchStore, embedder Embedder) *HybridSearcher {
	return &HybridSearcher{store: store, embedder: embedder}
}

// Search runs the hybrid retrieval path: lexical candidates, then the query
// embedding, then semantic candidates, then fusion, then hydration. An
// embedder failure aborts the whole search with ErrSearchUnavailable before
// the semantic candidate query ever runs -- there is no fallback to
// lexical-only results.
func (s *HybridSearcher) Search(ctx context.Context, input SearchInput) ([]SearchResult, error) {
	normalized := NormalizeSearchInput(input)
	if problems := ValidateSearchInput(normalized); len(problems) > 0 {
		return nil, fmt.Errorf("hybrid search: invalid input")
	}

	if !HasSearchableTokens(normalized.Query) {
		return []SearchResult{}, nil
	}

	lexicalNotes, err := s.store.SearchNotes(ctx, SearchInput{
		CategorySlug: normalized.CategorySlug,
		Query:        normalized.Query,
		Limit:        SemanticCandidateLimit,
		ViewerUserID: normalized.ViewerUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("hybrid search: lexical candidates: %w", err)
	}

	queryVector, err := s.embedder.EmbedQuery(ctx, normalized.Query)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w: %v", ErrSearchUnavailable, err)
	}

	scoredSemantic, err := s.store.SearchSemantic(ctx, SemanticSearchInput{
		Vector:       queryVector,
		CategorySlug: normalized.CategorySlug,
		Limit:        SemanticCandidateLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("hybrid search: semantic candidates: %w", err)
	}

	lexicalIDs := make([]string, len(lexicalNotes))
	lexicalByID := make(map[string]Note, len(lexicalNotes))
	for i, found := range lexicalNotes {
		lexicalIDs[i] = found.ID
		lexicalByID[found.ID] = found
	}
	semanticIDs := make([]string, len(scoredSemantic))
	for i, scored := range scoredSemantic {
		semanticIDs[i] = scored.NoteID
	}

	fused := FuseSearchCandidates(lexicalIDs, semanticIDs, normalized.Limit)

	missingIDs := make([]string, 0)
	for _, candidate := range fused {
		if _, ok := lexicalByID[candidate.NoteID]; !ok {
			missingIDs = append(missingIDs, candidate.NoteID)
		}
	}
	hydratedByID := make(map[string]Note, len(missingIDs))
	if len(missingIDs) > 0 {
		hydrated, err := s.store.FindNotesByID(ctx, missingIDs, normalized.ViewerUserID)
		if err != nil {
			return nil, fmt.Errorf("hybrid search: hydrate semantic-only candidates: %w", err)
		}
		for _, found := range hydrated {
			hydratedByID[found.ID] = found
		}
	}

	results := make([]SearchResult, 0, len(fused))
	for _, candidate := range fused {
		found, ok := lexicalByID[candidate.NoteID]
		if !ok {
			found, ok = hydratedByID[candidate.NoteID]
		}
		if !ok {
			// The note was deleted between candidate retrieval and
			// hydration; drop it rather than surface a hollow result.
			continue
		}
		results = append(results, SearchResult{Note: found, RetrievalSource: candidate.RetrievalSource})
	}
	return results, nil
}
