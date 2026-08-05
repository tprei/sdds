package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

// newPublicReadRateLimitRouter builds a router whose public-read routes run
// under the supplied limits and frozen clock, so the rate-limit behaviour is
// deterministic. Writes still resolve through the authenticated group.
func newPublicReadRateLimitRouter(t *testing.T, limits PublicReadLimits, clock func() time.Time) http.Handler {
	t.Helper()
	now := clock()
	markedNote := note.Note{
		ID:           exampleNoteID,
		Title:        "Café bom",
		Body:         "Tem pao de queijo decente.",
		CategorySlug: note.CategorySlugFood,
		Author:       note.AuthorSummary{ID: publicReadAuthorID, DisplayName: "Thiago"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	store := fakeNoteStore{
		listNotes: func(context.Context, note.ListInput) ([]note.Note, error) {
			return []note.Note{markedNote}, nil
		},
		findNote: func(context.Context, string, user.UserID) (note.Note, error) {
			return markedNote, nil
		},
	}
	users := authenticatedFakeUserStore(fakeUserStore{
		findPublicAuthor: func(context.Context, author.AuthorID) (author.PublicAuthor, error) {
			return author.PublicAuthor{ID: publicReadAuthorID, DisplayName: "Thiago"}, nil
		},
	})
	hasher := authTestPasswordHasher()
	return newRouter(
		noteHandlers{noteStore: store, notePublisher: store, noteSearcher: store, authorNoteStore: store, usefulStore: store, categoryCatalog: fakeCatalog{}},
		commentHandlers{store: fakeCommentStore{}, notes: store},
		reportHandlers{store: fakeReportStore{}, notes: store, comments: fakeCommentStore{}},
		eventHandlers{store: fakeEventStore{}, limits: newEventRateLimiters(DefaultEventLimits(), clock), clock: clock},
		authHandlers{
			users:                 users,
			publicAuthors:         users,
			contactChannels:       fakeContactChannelStore{},
			passwordHasher:        hasher,
			invalidCredentialHash: mustInvalidCredentialHash(hasher),
			rateLimiters:          newAuthRateLimiters(DefaultAuthLimits(), clock),
			newSessionToken:       user.NewSessionToken,
			clock:                 clock,
		},
		mediaHandlers{imageUploads: fakeUploadPreparer{}, attachedImages: fakeAttachedImageReader{}},
		systemHandlers{readiness: fakeReadiness{}},
		newPublicReadRateLimiters(limits, clock),
	)
}

func publicReadLimitRequest(remoteAddr string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)
	request.RemoteAddr = remoteAddr
	return request
}

func requireRetryAfter(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	retryAfter := response.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("missing Retry-After header on rate-limited response")
	}
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Fatalf("Retry-After = %q: %v", retryAfter, err)
	}
	if seconds < 1 || seconds > 60 {
		t.Fatalf("Retry-After = %d, want a value in [1, 60]", seconds)
	}
}

func TestPublicReadRateLimitRejectsThirdRequestFromSameSource(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 2,
		GlobalRequestsPerMinute: 1000,
	}, func() time.Time { return now })

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, publicReadLimitRequest("203.0.113.7:41234"))
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, response.Code, http.StatusOK)
		}
	}

	throttled := httptest.NewRecorder()
	router.ServeHTTP(throttled, publicReadLimitRequest("203.0.113.7:41234"))
	if throttled.Code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want %d", throttled.Code, http.StatusTooManyRequests)
	}
	requireErrorCode(t, throttled, openapi.ErrorCodeRateLimited)
	requireRetryAfter(t, throttled)
}

func TestPublicReadRateLimitKeysPerSource(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 2,
		GlobalRequestsPerMinute: 1000,
	}, func() time.Time { return now })

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, publicReadLimitRequest("203.0.113.7:41234"))
		if response.Code != http.StatusOK {
			t.Fatalf("throttled source request %d status = %d, want %d", i+1, response.Code, http.StatusOK)
		}
	}

	other := httptest.NewRecorder()
	router.ServeHTTP(other, publicReadLimitRequest("198.51.100.9:41234"))
	if other.Code != http.StatusOK {
		t.Fatalf("fresh source status = %d, want %d (per-IP keying must not share the counter)", other.Code, http.StatusOK)
	}
}

func TestPublicReadGlobalLimitBlocksEverySource(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 1000,
		GlobalRequestsPerMinute: 1,
	}, func() time.Time { return now })

	first := httptest.NewRecorder()
	router.ServeHTTP(first, publicReadLimitRequest("203.0.113.7:41234"))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, publicReadLimitRequest("198.51.100.9:41234"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d (global limit must block even a fresh source)", second.Code, http.StatusTooManyRequests)
	}
}

func TestPublicReadRateLimitRecoverAfterWindow(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	current := now
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 2,
		GlobalRequestsPerMinute: 1000,
	}, func() time.Time { return current })

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, publicReadLimitRequest("203.0.113.7:41234"))
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, response.Code, http.StatusOK)
		}
	}

	throttled := httptest.NewRecorder()
	router.ServeHTTP(throttled, publicReadLimitRequest("203.0.113.7:41234"))
	if throttled.Code != http.StatusTooManyRequests {
		t.Fatalf("pre-recovery status = %d, want %d", throttled.Code, http.StatusTooManyRequests)
	}

	current = now.Add(time.Minute)
	recovered := httptest.NewRecorder()
	router.ServeHTTP(recovered, publicReadLimitRequest("203.0.113.7:41234"))
	if recovered.Code != http.StatusOK {
		t.Fatalf("post-recovery status = %d, want %d", recovered.Code, http.StatusOK)
	}
}

func TestPublicReadRateLimitAlsoThrottlesAuthenticatedCallers(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 2,
		GlobalRequestsPerMinute: 1000,
	}, func() time.Time { return now })

	doAuthenticated := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := publicReadLimitRequest("203.0.113.7:41234")
		request.Header.Set("Authorization", "Bearer current-token")
		router.ServeHTTP(response, request)
		return response
	}

	for i := 0; i < 2; i++ {
		if response := doAuthenticated(); response.Code != http.StatusOK {
			t.Fatalf("authenticated request %d status = %d, want %d", i+1, response.Code, http.StatusOK)
		}
	}
	if response := doAuthenticated(); response.Code != http.StatusTooManyRequests {
		t.Fatalf("authenticated third request status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}

func TestPublicReadRateLimitDoesNotApplyToWrites(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	router := newPublicReadRateLimitRouter(t, PublicReadLimits{
		SourceRequestsPerMinute: 2,
		GlobalRequestsPerMinute: 1000,
	}, func() time.Time { return now })

	// Exhaust the public-read source limit.
	for i := 0; i < 3; i++ {
		router.ServeHTTP(httptest.NewRecorder(), publicReadLimitRequest("203.0.113.7:41234"))
	}

	// A write route has no token, so it returns 401 unauthenticated, never 429.
	writeResponse := httptest.NewRecorder()
	writeRequest := jsonRequest(http.MethodPost, "/v1/notes", `{"title":"x","body":"y","category_slug":"food"}`)
	writeRequest.RemoteAddr = "203.0.113.7:41234"
	router.ServeHTTP(writeResponse, writeRequest)

	if writeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("write status = %d, want %d (writes must not be subject to the public-read limiter)", writeResponse.Code, http.StatusUnauthorized)
	}
	requireErrorCode(t, writeResponse, openapi.ErrorCodeUnauthenticated)
}
