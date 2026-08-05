package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

var errDeleteUserStoreUnavailable = errors.New("store unavailable")

func deleteAuthUserRouter(t *testing.T, users fakeUserStore) http.Handler {
	t.Helper()
	return newAuthTestRouter(t, authenticatedFakeUserStore(users))
}

func TestDeleteAuthUserRejectsMissingBearer(t *testing.T) {
	deleteCalled := false
	router := deleteAuthUserRouter(t, fakeUserStore{
		findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
			t.Fatal("find password login called without a bearer")
			return user.PasswordLogin{}, nil
		},
		deleteUser: func(context.Context, user.UserID, time.Time) error {
			deleteCalled = true
			return nil
		},
	})

	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodDelete, "/v1/auth/users/me", `{"password":"secret-password"}`)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if deleteCalled {
		t.Fatal("delete user was called without authentication")
	}
}

func TestDeleteAuthUserRejectsWrongPassword(t *testing.T) {
	deleteCalled := false
	router := deleteAuthUserRouter(t, fakeUserStore{
		findPasswordLogin: func(_ context.Context, username string) (user.PasswordLogin, error) {
			if username != "thiago" {
				t.Fatalf("find password login username = %q, want thiago", username)
			}
			return user.PasswordLogin{SecretHash: authTestPasswordHash(t, "secret-password")}, nil
		},
		deleteUser: func(context.Context, user.UserID, time.Time) error {
			deleteCalled = true
			return nil
		},
	})

	response := httptest.NewRecorder()
	request := authRequest(http.MethodDelete, "/v1/auth/users/me", "current-token", `{"password":"wrong-password"}`)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeForbidden {
		t.Fatalf("code = %q, want %q", body.Code, openapi.ErrorCodeForbidden)
	}
	if deleteCalled {
		t.Fatal("delete user was called with a wrong password")
	}
}

func TestDeleteAuthUserDeletesAccountOnCorrectPassword(t *testing.T) {
	var capturedUserID user.UserID
	var capturedAt time.Time
	router := deleteAuthUserRouter(t, fakeUserStore{
		findPasswordLogin: func(_ context.Context, username string) (user.PasswordLogin, error) {
			if username != "thiago" {
				t.Fatalf("find password login username = %q, want thiago", username)
			}
			return user.PasswordLogin{SecretHash: authTestPasswordHash(t, "secret-password")}, nil
		},
		deleteUser: func(_ context.Context, userID user.UserID, deletedAt time.Time) error {
			capturedUserID = userID
			capturedAt = deletedAt
			return nil
		},
	})

	response := httptest.NewRecorder()
	request := authRequest(http.MethodDelete, "/v1/auth/users/me", "current-token", `{"password":"secret-password"}`)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if capturedUserID != "user-id-thiago" {
		t.Fatalf("deleted user id = %q, want user-id-thiago", capturedUserID)
	}
	wantAt := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	if !capturedAt.Equal(wantAt) {
		t.Fatalf("deleted at = %s, want %s", capturedAt, wantAt)
	}
}

func TestDeleteAuthUserSurfacesStoreError(t *testing.T) {
	router := deleteAuthUserRouter(t, fakeUserStore{
		findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
			return user.PasswordLogin{SecretHash: authTestPasswordHash(t, "secret-password")}, nil
		},
		deleteUser: func(context.Context, user.UserID, time.Time) error {
			return errDeleteUserStoreUnavailable
		},
	})

	response := httptest.NewRecorder()
	request := authRequest(http.MethodDelete, "/v1/auth/users/me", "current-token", `{"password":"secret-password"}`)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInternal {
		t.Fatalf("code = %q, want %q", body.Code, openapi.ErrorCodeInternal)
	}
}

func TestDeleteAuthUserRejectsOversizedBody(t *testing.T) {
	router := deleteAuthUserRouter(t, fakeUserStore{
		findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
			t.Fatal("find password login called for an oversized body")
			return user.PasswordLogin{}, nil
		},
	})

	response := httptest.NewRecorder()
	oversized := `{"password":"` + strings.Repeat("a", int(maxAuthRequestBytes)) + `"}`
	request := authRequest(http.MethodDelete, "/v1/auth/users/me", "current-token", oversized)
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeRequestTooLarge)
	}
}
