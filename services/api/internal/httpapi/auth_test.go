package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/author"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestCreateAuthUserReturnsSession(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	hasher := authTestPasswordHasher()
	router := newRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			if input.Username != "thiago" {
				t.Fatalf("username = %q, want thiago", input.Username)
			}
			if input.DisplayName != "Thiago" {
				t.Fatalf("display name = %q, want Thiago", input.DisplayName)
			}
			verified, err := hasher.Verify("secret-password", input.SecretHash)
			if err != nil {
				t.Fatalf("verify secret hash: %v", err)
			}
			if !verified {
				t.Fatal("secret hash did not verify password")
			}
			if input.TokenHash != user.HashSessionToken("created-token") {
				t.Fatalf("token hash = %q, want hash of created token", input.TokenHash)
			}
			if input.ExpiresAt != now.Add(user.SessionLifetime) {
				t.Fatalf("expires at = %s, want %s", input.ExpiresAt, now.Add(user.SessionLifetime))
			}
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, hasher, authTestCredentialProbeHash(t), func() (string, error) {
		return "created-token", nil
	}, func() time.Time { return now }, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})

	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":" Thiago ","password":"secret-password","display_name":" Thiago "}`)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	var body openapi.AuthSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := openapi.AuthSessionResponse{
		Token:     "created-token",
		ExpiresAt: now.Add(user.SessionLifetime).UnixMilli(),
		User: openapi.CurrentUser{
			Id:       "user-id-thiago",
			Username: "thiago",
			Author: openapi.AuthorSummary{
				Id:          "author-id-thiago",
				DisplayName: "Thiago",
			},
		},
	}
	if diff := cmp.Diff(want, body); diff != "" {
		t.Fatalf("response mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateAuthUserReturnsUsernameTaken(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{
		createPasswordUser: func(context.Context, user.CreatePasswordUserInput) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrUsernameTaken
		},
	})
	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeUsernameTaken {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeUsernameTaken)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{{
		Field: openapi.ValidationFieldUsername,
		Code:  openapi.ValidationProblemCodeTaken,
	}})
}

func TestCreateAuthUserRejectsValidationProblems(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{
		createPasswordUser: func(context.Context, user.CreatePasswordUserInput) (user.CurrentSession, error) {
			t.Fatal("CreatePasswordUser should not be called")
			return user.CurrentSession{}, nil
		},
	})
	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"té","password":"short","display_name":""}`)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidAuth {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidAuth)
	}
	requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{
		{Field: openapi.ValidationFieldUsername, Code: openapi.ValidationProblemCodeTooShort},
		{Field: openapi.ValidationFieldPassword, Code: openapi.ValidationProblemCodeTooShort},
		{Field: openapi.ValidationFieldDisplayName, Code: openapi.ValidationProblemCodeRequired},
	})
}

func TestCreateAuthSessionReturnsSession(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	hasher := authTestPasswordHasher()
	secretHash, err := hasher.Hash("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	router := newRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		findPasswordLogin: func(_ context.Context, normalizedUsername string) (user.PasswordLogin, error) {
			if normalizedUsername != "thiago" {
				t.Fatalf("normalized username = %q, want thiago", normalizedUsername)
			}
			return user.PasswordLogin{
				User:       user.User{ID: "user-id-thiago", State: user.UserStateActive},
				Author:     user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
				Username:   "thiago",
				SecretHash: secretHash,
			}, nil
		},
		createSession: func(_ context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
			if input.UserID != "user-id-thiago" {
				t.Fatalf("user id = %q, want user-id-thiago", input.UserID)
			}
			if input.TokenHash != user.HashSessionToken("login-token") {
				t.Fatalf("token hash = %q, want hash of login token", input.TokenHash)
			}
			return authCurrentSession("thiago", "Thiago", input.TokenHash, input.ExpiresAt), nil
		},
	}, hasher, authTestCredentialProbeHash(t), func() (string, error) {
		return "login-token", nil
	}, func() time.Time { return now }, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})
	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":" Thiago ","password":"secret-password"}`)

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	var body openapi.AuthSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Token != "login-token" {
		t.Fatalf("token = %q, want login-token", body.Token)
	}
	if body.User.Username != "thiago" {
		t.Fatalf("username = %q, want thiago", body.User.Username)
	}
}

func TestCreateAuthSessionHidesInvalidCredentialDetails(t *testing.T) {
	tests := []struct {
		name  string
		store fakeUserStore
	}{
		{
			name: "missing username",
			store: fakeUserStore{
				findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
					return user.PasswordLogin{}, user.ErrInvalidCredentials
				},
			},
		},
		{
			name: "wrong password",
			store: fakeUserStore{
				findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
					secretHash, err := authTestPasswordHasher().Hash("actual-password")
					if err != nil {
						t.Fatalf("hash password: %v", err)
					}
					return user.PasswordLogin{
						User:       user.User{ID: "user-id-thiago", State: user.UserStateActive},
						Author:     user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
						Username:   "thiago",
						SecretHash: secretHash,
					}, nil
				},
			},
		},
		{
			name: "disabled user",
			store: fakeUserStore{
				findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
					secretHash, err := authTestPasswordHasher().Hash("secret-password")
					if err != nil {
						t.Fatalf("hash password: %v", err)
					}
					return user.PasswordLogin{
						User:       user.User{ID: "user-id-thiago", State: user.UserStateDisabled},
						Author:     user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
						Username:   "thiago",
						SecretHash: secretHash,
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newAuthTestRouter(t, tt.store)
			response := httptest.NewRecorder()
			request := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`)

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeInvalidAuth {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeInvalidAuth)
			}
			if body.Fields != nil {
				t.Fatalf("fields = %#v, want nil", *body.Fields)
			}
		})
	}
}

func TestCreateAuthSessionVerifiesPasswordBeforeRejectingCredentials(t *testing.T) {
	tests := []struct {
		name     string
		login    user.PasswordLogin
		err      error
		wantHash func(secretHash string, probeHash string) string
	}{
		{
			name: "missing username",
			err:  user.ErrInvalidCredentials,
			wantHash: func(_ string, probeHash string) string {
				return probeHash
			},
		},
		{
			name: "wrong password",
			login: user.PasswordLogin{
				User:     user.User{ID: "user-id-thiago", State: user.UserStateActive},
				Author:   user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
				Username: "thiago",
			},
			wantHash: func(secretHash string, _ string) string {
				return secretHash
			},
		},
		{
			name: "disabled user",
			login: user.PasswordLogin{
				User:     user.User{ID: "user-id-thiago", State: user.UserStateDisabled},
				Author:   user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
				Username: "thiago",
			},
			wantHash: func(_ string, probeHash string) string {
				return probeHash
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretHash := authTestPasswordHash(t, "actual-password")
			probeHash := authTestPasswordHash(t, "invalid-credential-probe")
			login := tt.login
			login.SecretHash = secretHash
			hasher := &recordingPasswordHasher{delegate: authTestPasswordHasher()}
			router := newRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
				findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
					return login, tt.err
				},
				createSession: func(context.Context, user.CreateSessionInput) (user.CurrentSession, error) {
					t.Fatal("CreateSession should not be called")
					return user.CurrentSession{}, nil
				},
			}, hasher, probeHash, func() (string, error) {
				return "unused-token", nil
			}, func() time.Time {
				return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
			}, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})

			response := httptest.NewRecorder()
			request := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`)

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			if len(hasher.verifyCalls) != 1 {
				t.Fatalf("verify calls = %d, want 1", len(hasher.verifyCalls))
			}
			got := hasher.verifyCalls[0]
			if got.password != "secret-password" {
				t.Fatalf("verified password = %q, want secret-password", got.password)
			}
			if got.encoded != tt.wantHash(secretHash, probeHash) {
				t.Fatalf("verified hash = %q, want expected rejection hash", got.encoded)
			}
		})
	}
}

func TestGetAuthSessionReturnsCurrentSession(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	tokenHash := user.HashSessionToken("current-token")
	router := newRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		findCurrentSession: func(_ context.Context, gotTokenHash string, gotNow time.Time) (user.CurrentSession, error) {
			if gotTokenHash != tokenHash {
				t.Fatalf("token hash = %q, want %q", gotTokenHash, tokenHash)
			}
			if gotNow != now {
				t.Fatalf("now = %s, want %s", gotNow, now)
			}
			return authCurrentSession("thiago", "Thiago", tokenHash, now.Add(user.SessionLifetime)), nil
		},
	}, authTestPasswordHasher(), authTestCredentialProbeHash(t), func() (string, error) {
		return "unused-token", nil
	}, func() time.Time { return now }, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	request.Header.Set("Authorization", "Bearer current-token")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body openapi.CurrentSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.Id != "user-id-thiago" {
		t.Fatalf("user id = %q, want user-id-thiago", body.User.Id)
	}
}

func TestGetAuthSessionAcceptsCaseInsensitiveBearerScheme(t *testing.T) {
	tokenHash := user.HashSessionToken("current-token")
	router := newAuthTestRouter(t, fakeUserStore{
		findCurrentSession: func(_ context.Context, gotTokenHash string, _ time.Time) (user.CurrentSession, error) {
			if gotTokenHash != tokenHash {
				t.Fatalf("token hash = %q, want %q", gotTokenHash, tokenHash)
			}
			now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
			return authCurrentSession("thiago", "Thiago", tokenHash, now.Add(user.SessionLifetime)), nil
		},
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	request.Header.Set("Authorization", "bEaReR current-token")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestAuthBoundaryRejectsUnauthenticatedSessions(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		storeError    error
	}{
		{name: "missing header"},
		{name: "malformed header", authorization: "Token abc"},
		{name: "unknown token", authorization: "Bearer missing", storeError: user.ErrSessionNotFound},
		{name: "expired", authorization: "Bearer expired", storeError: user.ErrSessionExpired},
		{name: "revoked", authorization: "Bearer revoked", storeError: user.ErrSessionRevoked},
		{name: "disabled user", authorization: "Bearer disabled", storeError: user.ErrUserDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newAuthTestRouter(t, fakeUserStore{
				findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
					if tt.storeError == nil {
						t.Fatal("FindCurrentSession should not be called")
					}
					return user.CurrentSession{}, tt.storeError
				},
			})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != openapi.ErrorCodeUnauthenticated {
				t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeUnauthenticated)
			}
		})
	}
}

func TestDeleteAuthSessionRevokesCurrentSession(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	current := authCurrentSession("thiago", "Thiago", user.HashSessionToken("current-token"), now.Add(user.SessionLifetime))
	router := newRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{
		findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
			return current, nil
		},
		revokeSession: func(_ context.Context, sessionID user.SessionID, revokedAt time.Time) error {
			if sessionID != current.Session.ID {
				t.Fatalf("session id = %q, want %q", sessionID, current.Session.ID)
			}
			if revokedAt != now {
				t.Fatalf("revoked at = %s, want %s", revokedAt, now)
			}
			return nil
		},
	}, authTestPasswordHasher(), authTestCredentialProbeHash(t), func() (string, error) {
		return "unused-token", nil
	}, func() time.Time { return now }, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/v1/auth/session", nil)
	request.Header.Set("Authorization", "Bearer current-token")

	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestAuthRequestsRejectOversizedBodies(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{})

	for _, path := range []string{"/v1/auth/users", "/v1/auth/sessions"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := jsonRequest(http.MethodPost, path, `{"username":"thiago","password":"`+strings.Repeat("a", int(maxAuthRequestBytes))+`","display_name":"Thiago"}`)

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
		})
	}
}

func TestAuthSessionRoutesRejectRequestBodies(t *testing.T) {
	tokenHash := user.HashSessionToken("current-token")
	router := newAuthTestRouter(t, fakeUserStore{
		findCurrentSession: func(_ context.Context, gotTokenHash string, now time.Time) (user.CurrentSession, error) {
			if gotTokenHash != tokenHash {
				t.Fatalf("token hash = %q, want %q", gotTokenHash, tokenHash)
			}
			return authCurrentSession("thiago", "Thiago", tokenHash, now.Add(user.SessionLifetime)), nil
		},
	})

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/v1/auth/session"},
		{method: http.MethodDelete, path: "/v1/auth/session"},
	} {
		t.Run(tt.method, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"unexpected":true}`))
			request.Header.Set("Authorization", "Bearer current-token")
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(response, request)
			requireOpenAPIResponse(t, request, response)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestAuthPreflightAllowsAuthorizationAndDelete(t *testing.T) {
	router := newAuthTestRouter(t, fakeUserStore{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/v1/auth/session", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8081")
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	request.Header.Set("Access-Control-Request-Headers", "Authorization")

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Headers") != corsAllowedHeaders {
		t.Fatalf("allow headers = %q, want %q", response.Header().Get("Access-Control-Allow-Headers"), corsAllowedHeaders)
	}
	if response.Header().Get("Access-Control-Allow-Methods") != corsAllowedMethods {
		t.Fatalf("allow methods = %q, want %q", response.Header().Get("Access-Control-Allow-Methods"), corsAllowedMethods)
	}
}

func newAuthTestRouter(t *testing.T, users fakeUserStore) http.Handler {
	t.Helper()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	return newRouter(fakeNoteStore{}, fakeCatalog{}, users, authTestPasswordHasher(), authTestCredentialProbeHash(t), func() (string, error) {
		return "test-token", nil
	}, func() time.Time { return now }, authTestLimits(), fakeReadiness{}, fakeUploadPreparer{}, fakeAttachedImageReader{})
}

func authTestLimits() AuthLimits {
	return AuthLimits{
		SignupRequestsPerMinute:       1000,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}
}

func authTestCredentialProbeHash(t *testing.T) string {
	t.Helper()
	return authTestPasswordHash(t, "invalid-credential-probe")
}

func authTestPasswordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := authTestPasswordHasher().Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

func authTestPasswordHasher() user.PasswordHasher {
	return user.PasswordHasher{
		Params: user.Argon2idParams{
			Memory:      1024,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		},
		SaltReader: bytes.NewReader(bytes.Repeat([]byte{3}, 16*1024)),
	}
}

type verifyCall struct {
	password string
	encoded  string
}

type recordingPasswordHasher struct {
	delegate    user.PasswordHasher
	verifyCalls []verifyCall
}

func (hasher *recordingPasswordHasher) Hash(password string) (string, error) {
	return hasher.delegate.Hash(password)
}

func (hasher *recordingPasswordHasher) Verify(password string, encoded string) (bool, error) {
	hasher.verifyCalls = append(hasher.verifyCalls, verifyCall{password: password, encoded: encoded})
	return hasher.delegate.Verify(password, encoded)
}

func authCurrentSession(username string, displayName string, tokenHash string, expiresAt time.Time) user.CurrentSession {
	return user.CurrentSession{
		Session: user.Session{
			ID:        user.SessionID("session-id-" + username),
			UserID:    user.UserID("user-id-" + username),
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
		},
		User: user.User{
			ID:    user.UserID("user-id-" + username),
			State: user.UserStateActive,
		},
		Author: user.Author{
			ID:          author.AuthorID("author-id-" + username),
			UserID:      user.UserID("user-id-" + username),
			DisplayName: displayName,
		},
		Username: username,
	}
}

func jsonRequest(method string, path string, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}
