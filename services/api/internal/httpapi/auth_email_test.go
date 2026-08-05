package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

func newSQLiteAuthRouter(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	userStore := sqlite.NewUserStore(db)
	channelStore := sqlite.NewContactChannelStore(db)
	router := NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: userStore, ContactChannels: channelStore, Mail: &recordingMailSender{}, Schedule: func(fn func()) { fn() }, Limits: DefaultAuthLimits()},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
		PublicReadDependencies{},
	)
	return router, db
}

func authRequest(method, path, token, body string) *http.Request {
	request := jsonRequest(method, path, body)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func signupWithEmailTestUser(t *testing.T, router http.Handler, username, email string) string {
	t.Helper()
	payload := map[string]string{"username": username, "password": "secret-password", "display_name": username}
	if email != "" {
		payload["email"] = email
	}
	body, _ := json.Marshal(payload)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/users", string(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var session openapi.AuthSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	return session.Token
}

func TestSetAuthEmailRequiresAuthentication(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPut, "/v1/auth/email", "", `{"email":"ana@example.com"}`))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestSetAuthEmailRejectsMalformedAddress(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	token := signupWithEmailTestUser(t, router, "ana", "")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"notanemail"}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code != openapi.ErrorCodeInvalidEmail {
		t.Fatalf("code = %s, want invalid_email", body.Code)
	}
	if body.Fields == nil || len(*body.Fields) != 1 || (*body.Fields)[0].Field != openapi.ValidationFieldEmail || (*body.Fields)[0].Code != "invalid" {
		t.Fatalf("fields = %v, want [{email invalid}]", body.Fields)
	}
}

func TestSetAuthEmailAcceptsValidAddressAndNormalizes(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	token := signupWithEmailTestUser(t, router, "ana", "")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":" Ana.Silva+Notas@Example.COM "}`))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", response.Code)
	}

	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("get session status = %d, want 200", sessionResponse.Code)
	}
	var session openapi.CurrentSessionResponse
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.User.Email == nil {
		t.Fatal("email is absent; want the stored address")
	}
	if session.User.Email.Address != "ana.silva+notas@example.com" {
		t.Fatalf("address = %q, want ana.silva+notas@example.com", session.User.Email.Address)
	}
	if session.User.Email.Verified {
		t.Fatal("verified = true, want false")
	}
}

func TestSetAuthEmailDoesNotRevealAddressTakenByAnother(t *testing.T) {
	router, db := newSQLiteAuthRouter(t)
	tokenA := signupWithEmailTestUser(t, router, "ana", "")
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", tokenA, `{"email":"shared@example.com"}`))

	// User A's address is verified out of band; the token flow ships in a later PR.
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `UPDATE user_contact_channels SET verified_at = 1, verified_via = 'token' WHERE normalized_value = 'shared@example.com'`); err != nil {
		t.Fatalf("verify address: %v", err)
	}

	tokenB := signupWithEmailTestUser(t, router, "bruno", "")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPut, "/v1/auth/email", tokenB, `{"email":"shared@example.com"}`))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (must not reveal the address is taken)", response.Code)
	}
}

func TestCreateAuthUserWithMalformedEmailStillSucceeds(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	token := signupWithEmailTestUser(t, router, "ana", "notanemail")

	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	var session openapi.CurrentSessionResponse
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.User.Email != nil {
		t.Fatalf("email = %v, want absent (malformed signup email must be ignored)", session.User.Email)
	}
}

func TestSignupEmailIsStoredUnverified(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	token := signupWithEmailTestUser(t, router, "ana", " Ana.Silva+Notas@Example.COM ")

	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	var session openapi.CurrentSessionResponse
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.User.Email == nil {
		t.Fatal("signup email was not stored")
	}
	if session.User.Email.Address != "ana.silva+notas@example.com" {
		t.Fatalf("address = %q, want normalized value", session.User.Email.Address)
	}
	if session.User.Email.Verified {
		t.Fatal("verified = true, want false")
	}
}

func TestSetAuthEmailRejectsOversizedBody(t *testing.T) {
	router, _ := newSQLiteAuthRouter(t)
	token := signupWithEmailTestUser(t, router, "ana", "")

	oversized := `{"email":"` + strings.Repeat("a", int(maxAuthRequestBytes)) + `"}` + strings.Repeat(" ", int(maxAuthRequestBytes))
	request := authRequest(http.MethodPut, "/v1/auth/email", token, oversized)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	requireOpenAPIResponse(t, request, response)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code != openapi.ErrorCodeRequestTooLarge {
		t.Fatalf("code = %s, want %s", body.Code, openapi.ErrorCodeRequestTooLarge)
	}
}

func TestSetAuthEmailRateLimitsPerAccount(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	limits := DefaultAuthLimits()
	limits.VerificationRequestsPerMinute = 1
	limits.VerificationGlobalRequestsPerMinute = 60
	router := NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{
			Users: sqlite.NewUserStore(db), ContactChannels: sqlite.NewContactChannelStore(db),
			Mail:   noopMailSender{},
			Limits: limits,
		},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
		PublicReadDependencies{},
	)
	token := signupWithEmailTestUser(t, router, "rate-thiago", "")
	body := `{"email":"first@example.com"}`

	first := httptest.NewRecorder()
	router.ServeHTTP(first, authRequest(http.MethodPut, "/v1/auth/email", token, body))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first PUT status = %d, want 202: %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"second@example.com"}`))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second PUT status = %d, want 429: %s", second.Code, second.Body.String())
	}
	var errorBody openapi.ErrorResponse
	if err := json.Unmarshal(second.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errorBody.Code != openapi.ErrorCodeRateLimited {
		t.Fatalf("second PUT code = %s, want %s", errorBody.Code, openapi.ErrorCodeRateLimited)
	}
}
