package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

const deleteAccountPassword = "secret-password"

// signUpDeleteAccountUser signs up a user and returns the bearer token, the
// user's author id, and the username, for account-deletion handler tests.
func signUpDeleteAccountUser(t *testing.T, router http.Handler, username string) (token, authorID string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username, "password": deleteAccountPassword, "display_name": username,
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/users", string(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var session openapi.AuthSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	return session.Token, session.User.Author.Id
}

func TestDeleteAuthUserRejectsWrongPassword(t *testing.T) {
	router, db := newSQLiteAuthRouter(t)
	token, _ := signUpDeleteAccountUser(t, router, "apagar-conta")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodDelete, "/v1/auth/users/me", token, `{"password":"wrong-password"}`))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidAuth {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidAuth)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users after wrong password = %d, want 1 (nothing deleted)", count)
	}
}

func TestDeleteAuthUserDeletesAccountAndKillsSession(t *testing.T) {
	router, db := newSQLiteAuthRouter(t)
	token, authorID := signUpDeleteAccountUser(t, router, "apagar-conta")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodDelete, "/v1/auth/users/me", token, `{"password":"secret-password"}`))
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204: %s", response.Code, response.Body.String())
	}
	assertNoUserRows(t, db)

	// The credentials no longer resolve a session.
	login := httptest.NewRecorder()
	router.ServeHTTP(login, jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"apagar-conta","password":"secret-password"}`))
	if login.Code == http.StatusCreated {
		t.Fatalf("login after deletion succeeded, want failure")
	}

	// The old bearer token is dead.
	session := httptest.NewRecorder()
	router.ServeHTTP(session, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	if session.Code != http.StatusUnauthorized {
		t.Fatalf("session lookup with dead token = %d, want 401", session.Code)
	}

	// The author profile is gone. The author route sits behind requireAuth, so
	// the dead token would 401 at the middleware before a 404; assert the row is
	// gone directly instead.
	var authorCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM authors WHERE id = ?`, authorID).Scan(&authorCount); err != nil {
		t.Fatalf("count authors: %v", err)
	}
	if authorCount != 0 {
		t.Fatalf("author count after deletion = %d, want 0", authorCount)
	}
}

func TestDeleteAuthUserRequiresAuthentication(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodDelete, "/v1/auth/users/me", "", `{"password":"secret-password"}`))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code != openapi.ErrorCodeUnauthenticated {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeUnauthenticated)
	}
}

func assertNoUserRows(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("users after deletion = %d, want 0", count)
	}
}
