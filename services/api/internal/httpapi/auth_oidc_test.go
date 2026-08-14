package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/oidc"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type fakeOIDCVerifier struct {
	verify func(ctx context.Context, provider oidc.Provider, idToken string, nonce string) (oidc.Identity, error)
}

func (verifier fakeOIDCVerifier) Verify(ctx context.Context, provider oidc.Provider, idToken string, nonce string) (oidc.Identity, error) {
	if verifier.verify == nil {
		return oidc.Identity{}, errors.New("oidc verify not implemented")
	}
	return verifier.verify(ctx, provider, idToken, nonce)
}

// newOIDCAuthTestRouter builds a router with an injected verifier, mirroring
// newAuthTestRouter for the provider sign-in endpoint.
func newOIDCAuthTestRouter(t *testing.T, verifier oidc.Verifier, users fakeUserStore, limits AuthLimits) http.Handler {
	t.Helper()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	return newRouter(
		noteHandlers{noteStore: fakeNoteStore{}, notePublisher: fakeNoteStore{}, noteSearcher: fakeNoteStore{}, categoryCatalog: fakeCatalog{}},
		commentHandlers{store: fakeCommentStore{}, notes: fakeNoteStore{}},
		reportHandlers{store: fakeReportStore{}, notes: fakeNoteStore{}, comments: fakeCommentStore{}},
		eventHandlers{store: fakeEventStore{}, limits: newEventRateLimiters(DefaultEventLimits(), func() time.Time { return now }), clock: func() time.Time { return now }},
		authHandlers{
			users:                 users,
			publicAuthors:         users,
			contactChannels:       fakeContactChannelStore{},
			passwordHasher:        authTestPasswordHasher(),
			invalidCredentialHash: authTestCredentialProbeHash(t),
			rateLimiters:          newAuthRateLimiters(limits, func() time.Time { return now }),
			newSessionToken:       func() (string, error) { return "test-token", nil },
			clock:                 func() time.Time { return now },
			oidc:                  verifier,
		},
		mediaHandlers{imageUploads: fakeUploadPreparer{}, attachedImages: fakeAttachedImageReader{}},
		systemHandlers{readiness: fakeReadiness{}},
		newPublicReadRateLimiters(DefaultPublicReadLimits(), func() time.Time { return now }),
	)
}

func oidcTestLimits() AuthLimits {
	limits := authTestLimits()
	limits.OIDCRequestsPerMinute = 1000
	limits.OIDCGlobalRequestsPerMinute = 1000
	return limits
}

func postOIDCSession(router http.Handler, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", body))
	return response
}

func requireOIDCError(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode openapi.ErrorCode) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != wantCode {
		t.Fatalf("code = %s, want %s", body.Code, wantCode)
	}
}

func TestCreateAuthOidcSessionUnavailableWithoutVerifier(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{})
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusServiceUnavailable, openapi.ErrorCodeOidcUnavailable)
}

func TestCreateAuthOidcSessionRejectsInvalidToken(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(context.Context, oidc.Provider, string, string) (oidc.Identity, error) {
		return oidc.Identity{}, oidc.ErrInvalidToken
	}}, fakeUserStore{}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"forged","nonce":"nonce"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusUnauthorized, openapi.ErrorCodeInvalidAuth)
}

func TestCreateAuthOidcSessionUnavailableOnVerifierOutage(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(context.Context, oidc.Provider, string, string) (oidc.Identity, error) {
		return oidc.Identity{}, oidc.ErrUnavailable
	}}, fakeUserStore{}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusServiceUnavailable, openapi.ErrorCodeOidcUnavailable)
}

func TestCreateAuthOidcSessionMapsVerifierFailureToInternal(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(context.Context, oidc.Provider, string, string) (oidc.Identity, error) {
		return oidc.Identity{}, errors.New("unexpected verifier failure")
	}}, fakeUserStore{}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusInternalServerError, openapi.ErrorCodeInternal)
}

func TestCreateAuthOidcSessionRejectsInvalidUsername(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{}, fakeUserStore{}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce","username":"Has Space"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusBadRequest, openapi.ErrorCodeInvalidAuth)
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Fields == nil || len(*body.Fields) != 1 || (*body.Fields)[0].Field != openapi.ValidationFieldUsername {
		t.Fatalf("fields = %+v, want a single username problem", body.Fields)
	}
}

func TestCreateAuthOidcSessionRequiresUsernameWithoutRevealingSubject(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(_ context.Context, provider oidc.Provider, _ string, _ string) (oidc.Identity, error) {
		return oidc.Identity{Provider: provider, Subject: "subject-1"}, nil
	}}, fakeUserStore{
		resolveOIDCIdentity: func(context.Context, user.ResolveOIDCIdentityInput) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrUsernameRequired
		},
	}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusConflict, openapi.ErrorCodeUsernameRequired)
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Fields != nil && len(*body.Fields) > 0 {
		t.Fatalf("fields = %+v, want none: the answer must not describe the subject", body.Fields)
	}
}

func TestCreateAuthOidcSessionReturnsTakenUsername(t *testing.T) {
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(_ context.Context, provider oidc.Provider, _ string, _ string) (oidc.Identity, error) {
		return oidc.Identity{Provider: provider, Subject: "subject-1"}, nil
	}}, fakeUserStore{
		resolveOIDCIdentity: func(context.Context, user.ResolveOIDCIdentityInput) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrUsernameTaken
		},
	}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"token","nonce":"nonce","username":"taken"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	requireOIDCError(t, response, http.StatusConflict, openapi.ErrorCodeUsernameTaken)
}

func TestCreateAuthOidcSessionCreatesSession(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	var gotInput user.ResolveOIDCIdentityInput
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(_ context.Context, provider oidc.Provider, idToken string, nonce string) (oidc.Identity, error) {
		if provider != oidc.ProviderGoogle {
			t.Fatalf("provider = %s, want google", provider)
		}
		if idToken != "id-token" || nonce != "request-nonce" {
			t.Fatalf("token = %q nonce = %q, want the request values", idToken, nonce)
		}
		return oidc.Identity{
			Provider:      provider,
			Subject:       "subject-1",
			Email:         "person@example.com",
			EmailVerified: true,
			DisplayName:   "Person Example",
		}, nil
	}}, fakeUserStore{
		resolveOIDCIdentity: func(_ context.Context, input user.ResolveOIDCIdentityInput) (user.CurrentSession, error) {
			gotInput = input
			return authCurrentSession("ana", "Ana", user.HashSessionToken("test-token"), now.Add(user.SessionLifetime)), nil
		},
	}, oidcTestLimits())
	response := postOIDCSession(router, `{"provider":"google","id_token":"id-token","nonce":"request-nonce","username":"Ana"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), response)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var body openapi.AuthSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Token != "test-token" {
		t.Fatalf("token = %q, want test-token", body.Token)
	}
	if body.User.Username != "ana" {
		t.Fatalf("username = %q, want ana", body.User.Username)
	}
	if gotInput.Provider != "google" || gotInput.Subject != "subject-1" || gotInput.Email != "person@example.com" || !gotInput.EmailVerified {
		t.Fatalf("resolve input = %+v, want the verified google identity", gotInput)
	}
	if gotInput.DisplayName != "Person Example" {
		t.Fatalf("display name = %q, want Person Example", gotInput.DisplayName)
	}
	if gotInput.Username != "ana" {
		t.Fatalf("normalized username = %q, want ana", gotInput.Username)
	}
	if gotInput.TokenHash != user.HashSessionToken("test-token") {
		t.Fatalf("token hash = %q, want the hashed session token", gotInput.TokenHash)
	}
	if !gotInput.ExpiresAt.Equal(now.Add(user.SessionLifetime)) {
		t.Fatalf("expires at = %s, want the session lifetime", gotInput.ExpiresAt)
	}
}

func TestCreateAuthOidcSessionRateLimitsBySource(t *testing.T) {
	limits := oidcTestLimits()
	limits.OIDCRequestsPerMinute = 1
	router := newOIDCAuthTestRouter(t, fakeOIDCVerifier{verify: func(_ context.Context, provider oidc.Provider, _ string, _ string) (oidc.Identity, error) {
		return oidc.Identity{Provider: provider, Subject: "subject-1"}, nil
	}}, fakeUserStore{
		resolveOIDCIdentity: func(context.Context, user.ResolveOIDCIdentityInput) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrInvalidCredentials
		},
	}, limits)

	request := jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", `{"provider":"google","id_token":"token","nonce":"nonce"}`)
	first := httptest.NewRecorder()
	router.ServeHTTP(first, request)
	requireOpenAPIResponse(t, request, first)
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first status = %d, want 401 (token rejected, not throttled)", first.Code)
	}

	retry := httptest.NewRecorder()
	router.ServeHTTP(retry, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", `{"provider":"google","id_token":"token","nonce":"nonce"}`))
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), retry)
	requireOIDCError(t, retry, http.StatusTooManyRequests, openapi.ErrorCodeRateLimited)
	if retry.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is absent")
	}
}
