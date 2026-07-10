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

func TestCreateAuthUserRateLimitReturnsTooManyRequests(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	createCalls := 0
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute: 1,
		LoginRequestsPerMinute:  1000,
		PasswordHashConcurrency: 4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`)
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"maria","password":"secret-password","display_name":"Maria"}`)
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

func TestCreateAuthSessionRateLimitReturnsTooManyRequests(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	findCalls := 0
	secretHash := authTestPasswordHash(t, "secret-password")
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
			findCalls++
			return user.PasswordLogin{
				User:       user.User{ID: "user-id-thiago", State: user.UserStateActive},
				Author:     user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
				Username:   "thiago",
				SecretHash: secretHash,
			}, nil
		},
		createSession: func(_ context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
			return authCurrentSession("thiago", "Thiago", input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute: 1000,
		LoginRequestsPerMinute:  1,
		PasswordHashConcurrency: 4,
	}, func() time.Time { return now })

	firstResponse := httptest.NewRecorder()
	firstRequest := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`)
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusCreated)
	}

	response := httptest.NewRecorder()
	request := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`)
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
	secretHash := authTestPasswordHash(t, "secret-password")
	router := newAuthRateLimitTestRouter(t, fakeUserStore{
		createPasswordUser: func(_ context.Context, input user.CreatePasswordUserInput) (user.CurrentSession, error) {
			createCalls++
			return authCurrentSession(input.Username, input.DisplayName, input.TokenHash, input.ExpiresAt), nil
		},
		findPasswordLogin: func(context.Context, string) (user.PasswordLogin, error) {
			findCalls++
			return user.PasswordLogin{
				User:       user.User{ID: "user-id-thiago", State: user.UserStateActive},
				Author:     user.Author{ID: "author-id-thiago", UserID: "user-id-thiago", DisplayName: "Thiago"},
				Username:   "thiago",
				SecretHash: secretHash,
			}, nil
		},
		createSession: func(_ context.Context, input user.CreateSessionInput) (user.CurrentSession, error) {
			return authCurrentSession("thiago", "Thiago", input.TokenHash, input.ExpiresAt), nil
		},
	}, AuthLimits{
		SignupRequestsPerMinute: 1,
		LoginRequestsPerMinute:  1,
		PasswordHashConcurrency: 4,
	}, func() time.Time { return now })

	signupResponse := httptest.NewRecorder()
	signupRequest := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"thiago","password":"secret-password","display_name":"Thiago"}`)
	router.ServeHTTP(signupResponse, signupRequest)
	if signupResponse.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", signupResponse.Code, http.StatusCreated)
	}

	loginResponse := httptest.NewRecorder()
	loginRequest := jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"thiago","password":"secret-password"}`)
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d", loginResponse.Code, http.StatusCreated)
	}

	now = now.Add(time.Minute)
	refillResponse := httptest.NewRecorder()
	refillRequest := jsonRequest(http.MethodPost, "/v1/auth/users", `{"username":"maria","password":"secret-password","display_name":"Maria"}`)
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
