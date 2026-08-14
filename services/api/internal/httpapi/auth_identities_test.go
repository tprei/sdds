package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

func currentSessionIdentities(t *testing.T, router http.Handler) openapi.CurrentUser {
	t.Helper()
	response := httptest.NewRecorder()
	request := authRequest(http.MethodGet, "/v1/auth/session", "current-token", "")
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusOK {
		t.Fatalf("get session status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var session openapi.CurrentSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return session.User
}

func TestCurrentSessionListsLoginIdentities(t *testing.T) {
	router := newAuthTestRouter(t, authenticatedFakeUserStore(fakeUserStore{
		listLoginIdentities: func(context.Context, user.UserID) ([]user.LoginIdentitySummary, error) {
			return []user.LoginIdentitySummary{
				{ID: "identity-password", Kind: user.LoginIdentityKindPassword, Provider: user.LoginIdentityProviderLocal},
				{ID: "identity-google", Kind: user.LoginIdentityKindOIDC, Provider: user.LoginIdentityProviderGoogle},
			}, nil
		},
	}))
	current := currentSessionIdentities(t, router)
	if len(current.Identities) != 2 {
		t.Fatalf("identities = %+v, want two entries", current.Identities)
	}
	password := current.Identities[0]
	if password.Id != "identity-password" || password.Kind != openapi.LoginIdentityKindPassword || password.Provider != openapi.LoginIdentityProviderLocal {
		t.Fatalf("password identity = %+v", password)
	}
	google := current.Identities[1]
	if google.Id != "identity-google" || google.Kind != openapi.LoginIdentityKindOidc || google.Provider != openapi.LoginIdentityProviderGoogle {
		t.Fatalf("google identity = %+v", google)
	}
}

func TestCurrentSessionFailsClosedWhenIdentitiesAreUnreadable(t *testing.T) {
	router := newAuthTestRouter(t, authenticatedFakeUserStore(fakeUserStore{
		listLoginIdentities: func(context.Context, user.UserID) ([]user.LoginIdentitySummary, error) {
			return nil, context.DeadlineExceeded
		},
	}))
	response := httptest.NewRecorder()
	request := authRequest(http.MethodGet, "/v1/auth/session", "current-token", "")
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	requireOIDCError(t, response, http.StatusInternalServerError, openapi.ErrorCodeInternal)
}

func deleteIdentity(router http.Handler, identityID string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/auth/identities/"+identityID, nil)
	request.Header.Set("Authorization", "Bearer current-token")
	router.ServeHTTP(response, request)
	return response
}

func TestDeleteAuthIdentityRequiresAuthentication(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/auth/identities/identity-1", nil)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestDeleteAuthIdentityDisconnects(t *testing.T) {
	var gotID user.LoginIdentityID
	router := newAuthTestRouter(t, authenticatedFakeUserStore(fakeUserStore{
		deleteLoginIdentity: func(_ context.Context, userID user.UserID, identityID user.LoginIdentityID) error {
			if userID != "user-id-thiago" {
				t.Fatalf("user id = %s, want the session user", userID)
			}
			gotID = identityID
			return nil
		},
	}))
	response := deleteIdentity(router, "identity-google")
	requireOpenAPIResponse(t, httptest.NewRequest(http.MethodDelete, "/v1/auth/identities/identity-google", nil), response)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if gotID != "identity-google" {
		t.Fatalf("identity id = %s, want identity-google", gotID)
	}
}

func TestDeleteAuthIdentityRefusesLastSignInMethod(t *testing.T) {
	router := newAuthTestRouter(t, authenticatedFakeUserStore(fakeUserStore{
		deleteLoginIdentity: func(context.Context, user.UserID, user.LoginIdentityID) error {
			return user.ErrLastLoginIdentity
		},
	}))
	response := deleteIdentity(router, "identity-password")
	requireOpenAPIResponse(t, httptest.NewRequest(http.MethodDelete, "/v1/auth/identities/identity-password", nil), response)
	requireOIDCError(t, response, http.StatusConflict, openapi.ErrorCodeLastSignInMethod)
}

func TestDeleteAuthIdentityHidesForeignAndMissingIdentities(t *testing.T) {
	router := newAuthTestRouter(t, authenticatedFakeUserStore(fakeUserStore{
		deleteLoginIdentity: func(context.Context, user.UserID, user.LoginIdentityID) error {
			return user.ErrLoginIdentityNotFound
		},
	}))
	for _, identityID := range []string{"identity-foreign", "00000000-0000-0000-0000-000000000000"} {
		response := deleteIdentity(router, identityID)
		requireOpenAPIResponse(t, httptest.NewRequest(http.MethodDelete, "/v1/auth/identities/"+identityID, nil), response)
		requireOIDCError(t, response, http.StatusNotFound, openapi.ErrorCodeNotFound)
	}
}
