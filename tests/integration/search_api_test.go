//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestSearchAPIHybridRetrieval proves the assembled publish-to-search
// journey: a query sharing no lexical tokens with a note's text still finds
// it through semantic retrieval, an exact business name or token still finds
// its note through lexical retrieval, and category filtering behaves
// consistently across both candidate sources.
func TestSearchAPIHybridRetrieval(t *testing.T) {
	client := newAPIClient(t)
	waitForReadiness(t, client)
	username := fmt.Sprintf("hybrid-%d", time.Now().UnixNano())
	session := createAuthUser(t, client, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    "secret-password",
		DisplayName: "Hybrid Search",
	})
	authed := newAuthenticatedAPIClient(t, session.Token)

	// The note shares zero tokens with the query, so FTS5-only search could
	// never find it.
	vocabularyMismatchRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Café da esquina",
		Body:            "Wi-Fi estável, várias tomadas e ninguém reclamou que fiquei duas horas",
		CategorySlug:    "food",
		ClientRequestId: "hybrid-vocab-mismatch-note",
	}
	vocabularyMismatchNote := createNote(t, authed, vocabularyMismatchRequest)

	// An exact, rare business-name token that must remain findable lexically
	// alongside semantic recall.
	exactNameRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Padaria Pão Quente",
		Body:            "O melhor bauru da cidade, direto da Padaria Pão Quente.",
		CategorySlug:    "food",
		ClientRequestId: "hybrid-exact-name-note",
	}
	exactNameNote := createNote(t, authed, exactNameRequest)

	// A note that is semantically close to the vocabulary-mismatch query but
	// lives in a different category, to prove category filtering is applied
	// consistently to the semantic candidate source too.
	semanticNearOtherCategoryRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Espaço de coworking",
		Body:            "Internet rápida, tomadas em todas as mesas, ambiente tranquilo para notebook.",
		CategorySlug:    "travel",
		ClientRequestId: "hybrid-semantic-other-category-note",
	}
	semanticNearOtherCategoryNote := createNote(t, authed, semanticNearOtherCategoryRequest)

	t.Run("vocabulary mismatch finds the note semantically", func(t *testing.T) {
		results := searchNotes(t, authed, "lugar bom pra trabalhar de notebook")
		match := requireSearchResultByID(t, results, vocabularyMismatchNote.Id)
		if match.RetrievalSource != openapi.Semantic {
			t.Fatalf("vocabulary-mismatch retrieval source = %q, want semantic (the query shares no lexical tokens with the note)", match.RetrievalSource)
		}
		if string(results.SearchVersion) != "hybrid-serafim100m-fts5-v1" {
			t.Fatalf("search version = %q, want hybrid-serafim100m-fts5-v1", results.SearchVersion)
		}
	})

	t.Run("exact business name still matches lexically", func(t *testing.T) {
		results := searchNotes(t, authed, "Padaria Pão Quente")
		requireLexicalMatch(t, results, exactNameNote.Id)
	})

	t.Run("category filter excludes a semantically-near note in another category", func(t *testing.T) {
		results := searchNotesByCategory(t, authed, "lugar bom pra trabalhar de notebook", "travel")
		for _, result := range results.Results {
			if result.Note.Id == vocabularyMismatchNote.Id {
				t.Fatalf("food-category note leaked into a travel-category search: %+v", result.Note)
			}
		}
		match := requireSearchResultByID(t, results, semanticNearOtherCategoryNote.Id)
		if match.RetrievalSource != openapi.Semantic && match.RetrievalSource != openapi.Hybrid {
			t.Fatalf("in-category note retrieval source = %q, want semantic or hybrid", match.RetrievalSource)
		}
	})
}
