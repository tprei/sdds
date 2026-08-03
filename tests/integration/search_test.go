//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

// TestSearchRuntimeBoundaries proves the Compose-to-runtime FTS search
// boundary: lexical matching, accent folding, strict AND semantics, title-over
// body ranking, category filtering, punctuation handling, and the invalid
// category-slug error path.
func TestSearchRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	waitForReadiness(t, publicClient)

	session := createAuthUser(t, publicClient, openapi.CreateAuthUserJSONRequestBody{
		Username:    fmt.Sprintf("search-%d", time.Now().UnixNano()),
		Password:    "secret-password",
		DisplayName: "Busca Runtime",
	})
	client := newAuthenticatedAPIClient(t, session.Token)

	balcaoRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Café bom",
		Body:            "Tem pao de queijo decente e balcao48 simpatico.",
		CategorySlug:    "food",
		ClientRequestId: "integration-search-balcao",
	}
	balcaoNote := createNote(t, client, balcaoRequest)
	balcaoSearchResults := searchNotes(t, client, "balcao48")
	balcaoMatch := requireLexicalMatch(t, balcaoSearchResults, balcaoNote.Id)
	requireCreatedNote(t, balcaoMatch.Note, balcaoRequest)

	mundialRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Dica de viagem",
		Body:            "Serve para qualquer lugar mundial48.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-search-mundial",
	}
	mundialNote := createNote(t, client, mundialRequest)
	mundialSearchResults := searchNotes(t, client, "mundial48")
	mundialMatch := requireLexicalMatch(t, mundialSearchResults, mundialNote.Id)
	requireCreatedNote(t, mundialMatch.Note, mundialRequest)

	filteredSearchResults := searchNotesByCategory(t, client, "mundial48", "travel")
	filteredMatch := requireLexicalMatch(t, filteredSearchResults, mundialNote.Id)
	requireCreatedNote(t, filteredMatch.Note, mundialRequest)

	emptyFilteredSearchResults := searchNotesByCategory(t, client, "mundial48", "food")
	for _, result := range emptyFilteredSearchResults.Results {
		if result.Note.Id == mundialNote.Id {
			t.Fatalf("travel note leaked into a food-category search: %+v", result.Note)
		}
	}

	emptySearchResults := searchNotes(t, client, "necessaire")
	if len(emptySearchResults.Results) == 0 {
		t.Fatal("semantic search returned no background matches for a non-empty query")
	}
	requireOnlySemanticMatches(t, emptySearchResults, "necessaire")

	accentRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Pão ftsaccent48",
		Body:            "Massa boa.",
		CategorySlug:    "food",
		ClientRequestId: "integration-accent-note",
	}
	accentNote := createNote(t, client, accentRequest)
	accentResults := searchNotes(t, client, "pao ftsaccent48")
	requireLexicalMatch(t, accentResults, accentNote.Id)

	strictBothRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "strictcafe48 strictpao48",
		Body:            "Encontro certo.",
		CategorySlug:    "food",
		ClientRequestId: "integration-strict-both",
	}
	strictBothNote := createNote(t, client, strictBothRequest)
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:           "strictcafe48",
		Body:            "Falta o segundo termo.",
		CategorySlug:    "food",
		ClientRequestId: "integration-strict-cafe",
	})
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:           "strictpao48",
		Body:            "Falta o primeiro termo.",
		CategorySlug:    "food",
		ClientRequestId: "integration-strict-pao",
	})
	strictResults := searchNotes(t, client, "strictcafe48 strictpao48")
	requireLexicalMatch(t, strictResults, strictBothNote.Id)
	requireNeverLexicallyMatched(t, strictResults, "integration-strict-cafe")
	requireNeverLexicallyMatched(t, strictResults, "integration-strict-pao")

	titleRankRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "rankbolo48 roteiro enorme com muitas palavras extras para alongar o titulo e reduzir relevancia sem peso",
		Body:            "Nota mais antiga.",
		CategorySlug:    "food",
		ClientRequestId: "integration-title-rank",
	}
	titleRankNote := createNote(t, client, titleRankRequest)
	bodyRankRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Bolo curto",
		Body:            "rankbolo48.",
		CategorySlug:    "food",
		ClientRequestId: "integration-body-rank",
	}
	bodyRankNote := createNote(t, client, bodyRankRequest)
	rankedResults := searchNotes(t, client, "rankbolo48")
	requireLexicalMatch(t, rankedResults, titleRankNote.Id)
	requireLexicalMatch(t, rankedResults, bodyRankNote.Id)

	categoryFoodRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "catbusca48 comida",
		Body:            "Filtro de categoria.",
		CategorySlug:    "food",
		ClientRequestId: "integration-category-food",
	}
	categoryFoodNote := createNote(t, client, categoryFoodRequest)
	categoryTravelNote := createNote(t, client, openapi.CreateNoteJSONRequestBody{
		Title:           "catbusca48 viagem",
		Body:            "Mesmo termo fora da categoria.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-category-travel",
	})
	categoryResults := searchNotesByCategory(t, client, "catbusca48", "food")
	requireLexicalMatch(t, categoryResults, categoryFoodNote.Id)
	for _, result := range categoryResults.Results {
		if result.Note.CategorySlug != openapi.CategorySlug("food") {
			t.Fatalf("food-category search returned %q", result.Note.CategorySlug)
		}
	}
	categoryTravelResults := searchNotesByCategory(t, client, "catbusca48", "travel")
	requireLexicalMatch(t, categoryTravelResults, categoryTravelNote.Id)
	for _, result := range categoryTravelResults.Results {
		if result.Note.CategorySlug != openapi.CategorySlug("travel") {
			t.Fatalf("travel-category search returned %q", result.Note.CategorySlug)
		}
	}

	globalFirstRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "globalbusca48 primeira",
		Body:            "Aparece na busca global.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-global-first",
	}
	globalFirstNote := createNote(t, client, globalFirstRequest)
	globalSecondRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "globalbusca48 segunda",
		Body:            "Tambem aparece na busca global.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-global-second",
	}
	globalSecondNote := createNote(t, client, globalSecondRequest)
	globalResults := searchNotes(t, client, "globalbusca48")
	requireLexicalMatch(t, globalResults, globalFirstNote.Id)
	requireLexicalMatch(t, globalResults, globalSecondNote.Id)

	puncRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "pontoseguro48",
		Body:            "Pontuacao nao muda a busca.",
		CategorySlug:    "food",
		ClientRequestId: "integration-punctuation",
	}
	puncNote := createNote(t, client, puncRequest)
	puncResults := searchNotes(t, client, "pontoseguro48 ***")
	requireLexicalMatch(t, puncResults, puncNote.Id)

	puncOnlyResults := searchNotes(t, client, "!!! *** ()")
	if len(puncOnlyResults.Results) != 0 {
		t.Fatalf("punctuation-only search note count = %d, want 0", len(puncOnlyResults.Results))
	}

	requireSearchNotesCategoryFilterError(t, client, "comida")
}

// TestSearchAPIHybridRetrieval proves the assembled publish-to-search journey:
// semantic recall, lexical exact matching, and category filtering across both sources.
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

	vocabularyMismatchRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Café da esquina",
		Body:            "Wi-Fi estável, várias tomadas e ninguém reclamou que fiquei duas horas",
		CategorySlug:    "food",
		ClientRequestId: "hybrid-vocab-mismatch-note",
	}
	vocabularyMismatchNote := createNote(t, authed, vocabularyMismatchRequest)

	exactNameRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Padaria Pão Quente",
		Body:            "O melhor bauru da cidade, direto da Padaria Pão Quente.",
		CategorySlug:    "food",
		ClientRequestId: "hybrid-exact-name-note",
	}
	exactNameNote := createNote(t, authed, exactNameRequest)

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
			t.Fatalf("vocabulary-mismatch retrieval source = %q, want semantic", match.RetrievalSource)
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
