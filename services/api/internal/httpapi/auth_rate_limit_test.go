package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestCreateAuthUserRateLimitReturnsTooManyRequestsForSameSource(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute:       1,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`, "203.0.113.10:1000")
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	response := httptest.NewRecorder()
	request := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"maria","password":"secret-password","display_name":"Maria"}`, "203.0.113.10:2000")
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	requireErrorCode(t, response, openapi.ErrorCodeRateLimited)
	if createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", createCalls)
	}
}

func TestCreateAuthUserAllowsDifferentSources(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute:       1,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`, "203.0.113.10:1000")
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"maria","password":"secret-password","display_name":"Maria"}`, "203.0.113.20:1000")
	router.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusCreated {
		t.Fatalf("second status = %d, want %d", secondResponse.Code, http.StatusCreated)
	}
	if createCalls != 2 {
		t.Fatalf("create calls = %d, want 2", createCalls)
	}
}

func TestMalformedSignupDoesNotConsumeAuthLimit(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute:       1000,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	malformedResponse := httptest.NewRecorder()
	malformedRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":`, "203.0.113.10:1000")
	router.ServeHTTP(malformedResponse, malformedRequest)
	if malformedResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, want %d", malformedResponse.Code, http.StatusBadRequest)
	}

	validResponse := httptest.NewRecorder()
	validRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`, "203.0.113.10:1000")
	router.ServeHTTP(validResponse, validRequest)
	if validResponse.Code != http.StatusCreated {
		t.Fatalf("valid status = %d, want %d", validResponse.Code, http.StatusCreated)
	}
	if createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", createCalls)
	}
}

func TestInvalidSignupDoesNotConsumeAuthLimit(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute:       1,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	invalidResponse := httptest.NewRecorder()
	invalidRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"x","password":"short","display_name":""}`, "203.0.113.10:1000")
	router.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	validResponse := httptest.NewRecorder()
	validRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`, "203.0.113.10:1000")
	router.ServeHTTP(validResponse, validRequest)
	if validResponse.Code != http.StatusCreated {
		t.Fatalf("valid status = %d, want %d", validResponse.Code, http.StatusCreated)
	}
	if createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", createCalls)
	}
}

func TestCreateAuthSessionRateLimitReturnsTooManyRequestsForSameAccount(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	findCalls := 0
	router := newAuthRateLimitTestRouter(t, newAuthRateLimitLoginStore(t, &findCalls), AuthLimits{
		SignupRequestsPerMinute:       1000,
		LoginRequestsPerMinute:        1,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`, "203.0.113.10:1000")
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	response := httptest.NewRecorder()
	request := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`, "203.0.113.20:1000")
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	requireErrorCode(t, response, openapi.ErrorCodeRateLimited)
	if findCalls != 1 {
		t.Fatalf("find calls = %d, want 1", findCalls)
	}
}

func TestCreateAuthSessionAllowsDifferentAccounts(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	findCalls := 0
	router := newAuthRateLimitTestRouter(t, newAuthRateLimitLoginStore(t, &findCalls), AuthLimits{
		SignupRequestsPerMinute:       1000,
		LoginRequestsPerMinute:        1,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`, "203.0.113.10:1000")
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"maria","password":"secret-password"}`, "203.0.113.20:1000")
	router.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusCreated {
		t.Fatalf("second status = %d, want %d", secondResponse.Code, http.StatusCreated)
	}
	if findCalls != 2 {
		t.Fatalf("find calls = %d, want 2", findCalls)
	}
}

func TestCreateAuthSessionGlobalRateLimitReturnsTooManyRequests(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	findCalls := 0
	router := newAuthRateLimitTestRouter(t, newAuthRateLimitLoginStore(t, &findCalls), AuthLimits{
		SignupRequestsPerMinute:       1000,
		LoginRequestsPerMinute:        1000,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`, "203.0.113.10:1000")
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	response := httptest.NewRecorder()
	request := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"maria","password":"secret-password"}`, "203.0.113.20:1000")
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	requireErrorCode(t, response, openapi.ErrorCodeRateLimited)
	if findCalls != 1 {
		t.Fatalf("find calls = %d, want 1", findCalls)
	}
}

func TestAuthRateLimitersAreIndependentAndRefill(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	findCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
		findPasswordLogin: authRateLimitFindPasswordLogin(t, &findCalls),
		createSession: func(_ context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
			return authCurrentSession("thiago", "Thiago", input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute:       1,
		LoginRequestsPerMinute:        1,
		SignupGlobalRequestsPerMinute: 1000,
		LoginGlobalRequestsPerMinute:  1000,
		PasswordHashConcurrency:       4,
	}, func() time.Time { return now })

	signupResponse := httptest.NewRecorder()
	signupRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`, "203.0.113.10:1000")
	router.ServeHTTP(signupResponse, signupRequest)
	if signupResponse.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", signupResponse.Code, http.StatusCreated)
	}

	loginResponse := httptest.NewRecorder()
	loginRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`, "203.0.113.10:1000")
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d", loginResponse.Code, http.StatusCreated)
	}

	now = now.Add(time.Minute)
	refillResponse := httptest.NewRecorder()
	refillRequest := authRateLimitRequest(http.MethodPost, "/v1/auth/users", `{"username":"maria","password":"secret-password","display_name":"Maria"}`, "203.0.113.10:1000")
	router.ServeHTTP(refillResponse, refillRequest)
	if refillResponse.Code != http.StatusCreated {
		t.Fatalf("refill status = %d, want %d", refillResponse.Code, http.StatusCreated)
	}
	if createCalls != 2 {
		t.Fatalf("create calls = %d, want 2", createCalls)
	}
	if findCalls != 1 {
		t.Fatalf("find calls = %d, want 1", findCalls)
	}
}

func TestKeyedRateLimitersEvictOldestEntry(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	limiters := newKeyedRequestsPerMinuteLimiters(1, 2)

	if !takeRateLimitToken(now, limiters.limiterFor("first", now)) {
		t.Fatal("first limiter rejected initial token")
	}
	limiters.limiterFor("second", now.Add(time.Second))
	limiters.limiterFor("third", now.Add(2*time.Second))

	if !takeRateLimitToken(now.Add(3*time.Second), limiters.limiterFor("first", now.Add(3*time.Second))) {
		t.Fatal("first limiter was not evicted")
	}
	if len(limiters.entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(limiters.entries))
	}
}

func newAuthRateLimitTestRouter(
	t *testing.T,
	users fakeUserStore,
	limits AuthLimits,
	clock func() time.Time,
) http.Handler {
	t.Helper()
	return newRouter(fakeNoteStore{}, fakeCatalog{}, users, authTestPasswordHasher(), authTestCredentialProbeHash(t), func() (string, error) {
		return "rate-limit-token", nil
	}, clock, limits)
}

func newAuthRateLimitLoginStore(t *testing.T, findCalls *int) fakeUserStore {
	t.Helper()
	return fakeUserStore{
		findPasswordLogin: authRateLimitFindPasswordLogin(t, findCalls),
		createSession: func(_ context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
			return authCurrentSession("login", "Login", input.TokenHash, input.ExpiresAt), nil
		},
	}
}

func authRateLimitFindPasswordLogin(t *testing.T, findCalls *int) func(context.Context, string) (user.PasswordLogin, error) {
	t.Helper()
	secretHash := authTestPasswordHash(t, "secret-password")
	return func(_ context.Context, username string) (user.PasswordLogin, error) {
		(*findCalls)++
		return user.PasswordLogin{
			User:       user.User{ID: user.UserID("user-id-" + username), State: user.UserStateActive},
			Author:     user.Author{ID: user.AuthorID("author-id-" + username), UserID: user.UserID("user-id-" + username), DisplayName: username},
			Username:   username,
			SecretHash: secretHash,
		}, nil
	}
}

func authRateLimitRequest(method string, path string, body string, remoteAddr string) *http.Request {
	request := jsonRequest(method, path, body)
	request.RemoteAddr = remoteAddr
	return request
}

func requireErrorCode(t *testing.T, response *httptest.ResponseRecorder, want openapi.ErrorCode) {
	t.Helper()

	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != want {
		t.Fatalf("code = %s, want %s", body.Code, want)
	}
}
