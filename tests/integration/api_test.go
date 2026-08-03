//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/openapi"
)

func TestAPIRuntimeBoundaries(t *testing.T) {
	publicClient := newAPIClient(t)
	client := publicClient

	waitForReadiness(t, client)

	health, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	requireStatus(t, "GET /healthz", health.StatusCode(), http.StatusNoContent, health.Body)

	username := fmt.Sprintf("thiago-%d", time.Now().UnixNano())
	displayName := "Thiago Integração"
	password := "secret-password"
	createdSession := createAuthUser(t, client, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	})
	requireAuthSession(t, createdSession, username, displayName)
	requireDuplicateAuthUser(t, client, openapi.CreateAuthUserJSONRequestBody{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	})
	sessionClient := newAuthenticatedAPIClient(t, createdSession.Token)
	currentSession := getAuthSession(t, sessionClient)
	requireCurrentSession(t, currentSession, createdSession)
	deleteAuthSession(t, sessionClient)
	requireUnauthenticatedAuthSession(t, sessionClient)
	requireInvalidAuthSession(t, client, username, "wrong-password")
	loggedInSession := createAuthSession(t, client, openapi.CreateAuthSessionJSONRequestBody{
		Username: username,
		Password: password,
	})
	requireAuthSession(t, loggedInSession, username, displayName)
	requireCurrentSession(t, getAuthSession(t, newAuthenticatedAPIClient(t, loggedInSession.Token)), loggedInSession)
	client = newAuthenticatedAPIClient(t, loggedInSession.Token)
	requireCatalogs(t, client)

	initialNotes := listNotes(t, client)
	if len(initialNotes.Notes) != 0 {
		t.Fatalf("initial note count = %d, want 0", len(initialNotes.Notes))
	}

	request := openapi.CreateNoteJSONRequestBody{
		Title:           "Café bom",
		Body:            "Tem pao de queijo decente e balcao simpatico.",
		CategorySlug:    "food",
		ClientRequestId: "integration-created-note",
	}
	created := createNote(t, client, request)
	requireCreatedNote(t, created, request)

	travelRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "Dica de viagem",
		Body:            "Serve para qualquer lugar mundial.",
		CategorySlug:    "travel",
		ClientRequestId: "integration-travel-note",
	}
	travelNote := createNote(t, client, travelRequest)
	requireCreatedNote(t, travelNote, travelRequest)

	updatedNotes := listNotes(t, client)
	if len(updatedNotes.Notes) != 2 {
		t.Fatalf("updated note count = %d, want 2", len(updatedNotes.Notes))
	}
	requireListedNote(t, updatedNotes, created.Id, request)
	requireListedNote(t, updatedNotes, travelNote.Id, travelRequest)

	foodNotes := listNotesByCategory(t, client, "food")
	if len(foodNotes.Notes) != 1 {
		t.Fatalf("food note count = %d, want 1", len(foodNotes.Notes))
	}
	requireListedNote(t, foodNotes, created.Id, request)

	travelNotes := listNotesByCategory(t, client, "travel")
	if len(travelNotes.Notes) != 1 {
		t.Fatalf("travel note count = %d, want 1", len(travelNotes.Notes))
	}
	requireListedNote(t, travelNotes, travelNote.Id, travelRequest)

	fetched := getNote(t, client, created.Id)
	requireCreatedNote(t, fetched, request)
	if fetched.Id != created.Id {
		t.Fatalf("fetched note id = %q, want %q", fetched.Id, created.Id)
	}
	if fetched.CreatedAt != created.CreatedAt {
		t.Fatalf("fetched created_at = %d, want %d", fetched.CreatedAt, created.CreatedAt)
	}
	if fetched.UpdatedAt != created.UpdatedAt {
		t.Fatalf("fetched updated_at = %d, want %d", fetched.UpdatedAt, created.UpdatedAt)
	}

	fetchedTravelNote := getNote(t, client, travelNote.Id)
	requireCreatedNote(t, fetchedTravelNote, travelRequest)
	if fetchedTravelNote.Id != travelNote.Id {
		t.Fatalf("fetched travel note id = %q, want %q", fetchedTravelNote.Id, travelNote.Id)
	}

	searchResults := searchNotes(t, client, "balcao")
	searchMatch := requireLexicalMatch(t, searchResults, created.Id)
	requireCreatedNote(t, searchMatch.Note, request)

	travelSearchResults := searchNotes(t, client, "mundial")
	travelSearchMatch := requireLexicalMatch(t, travelSearchResults, travelNote.Id)
	requireCreatedNote(t, travelSearchMatch.Note, travelRequest)

	filteredSearchResults := searchNotesByCategory(t, client, "mundial", "travel")
	filteredMatch := requireLexicalMatch(t, filteredSearchResults, travelNote.Id)
	requireCreatedNote(t, filteredMatch.Note, travelRequest)

	emptyFilteredSearchResults := searchNotesByCategory(t, client, "mundial", "food")
	for _, result := range emptyFilteredSearchResults.Results {
		if result.Note.Id == travelNote.Id {
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
	createNote(t, client, openapi.CreateNoteJSONRequestBody{
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

	punctuationRequest := openapi.CreateNoteJSONRequestBody{
		Title:           "pontoseguro48",
		Body:            "Pontuacao nao muda a busca.",
		CategorySlug:    "food",
		ClientRequestId: "integration-punctuation",
	}
	punctuationNote := createNote(t, client, punctuationRequest)
	punctuationResults := searchNotes(t, client, "pontoseguro48 ***")
	requireLexicalMatch(t, punctuationResults, punctuationNote.Id)

	punctuationOnlyResults := searchNotes(t, client, "!!! *** ()")
	if len(punctuationOnlyResults.Results) != 0 {
		t.Fatalf("punctuation-only search note count = %d, want 0", len(punctuationOnlyResults.Results))
	}

	requireListNotesCategoryFilterError(t, client, "comida")
	requireSearchNotesCategoryFilterError(t, client, "comida")
	requireMediaAPIRuntimeBoundaries(t, publicClient, client, loggedInSession.User.Author)
}
