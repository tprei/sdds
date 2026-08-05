package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tprei/sdds/services/api/internal/mail"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

type recordingMailSender struct {
	mu       sync.Mutex
	messages []mail.Message
	sendErr  error
}

func (sender *recordingMailSender) Send(_ context.Context, message mail.Message) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.messages = append(sender.messages, message)
	return sender.sendErr
}

func (sender *recordingMailSender) snapshot() []mail.Message {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	out := make([]mail.Message, len(sender.messages))
	copy(out, sender.messages)
	return out
}

func tokenFromMessage(t *testing.T, message mail.Message) string {
	t.Helper()
	idx := strings.Index(message.Text, "token=")
	if idx < 0 {
		t.Fatalf("message text has no token: %q", message.Text)
	}
	rest := message.Text[idx+len("token="):]
	if end := strings.IndexAny(rest, " \n\r\t"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func newSQLiteAuthRouterWithMail(t *testing.T, sender mail.Sender) http.Handler {
	t.Helper()
	return newSQLiteAuthRouterWithMailAndLimits(t, sender, DefaultAuthLimits())
}

func newSQLiteAuthRouterWithMailAndLimits(t *testing.T, sender mail.Sender, limits AuthLimits) http.Handler {
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
	return NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: userStore, ContactChannels: channelStore, Mail: sender, AppBaseURL: "https://app.sdds.test", Schedule: func(fn func()) { fn() }, Limits: limits},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
	)
}

func TestVerifyEmailFullRoundTrip(t *testing.T) {
	sender := &recordingMailSender{}
	router := newSQLiteAuthRouterWithMail(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")

	putResponse := httptest.NewRecorder()
	router.ServeHTTP(putResponse, authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"ana@example.com"}`))
	if putResponse.Code != http.StatusAccepted {
		t.Fatalf("set email status = %d, want 202", putResponse.Code)
	}

	messages := sender.snapshot()
	if len(messages) != 1 {
		t.Fatalf("dispatched %d messages, want 1", len(messages))
	}
	verifyToken := tokenFromMessage(t, messages[0])

	verifyResponse := httptest.NewRecorder()
	router.ServeHTTP(verifyResponse, jsonRequest(http.MethodPost, "/v1/auth/email/verification", `{"token":"`+verifyToken+`"}`))
	if verifyResponse.Code != http.StatusNoContent {
		t.Fatalf("verify status = %d, want 204", verifyResponse.Code)
	}

	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	var session openapi.CurrentSessionResponse
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.User.Email == nil || !session.User.Email.Verified {
		t.Fatalf("email = %v, want verified", session.User.Email)
	}

	replayResponse := httptest.NewRecorder()
	router.ServeHTTP(replayResponse, jsonRequest(http.MethodPost, "/v1/auth/email/verification", `{"token":"`+verifyToken+`"}`))
	if replayResponse.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want 400", replayResponse.Code)
	}
}

func TestVerifyEmailRateLimitsByIdentifier(t *testing.T) {
	sender := &recordingMailSender{}
	limits := DefaultAuthLimits()
	limits.VerificationRequestsPerMinute = 3
	router := newSQLiteAuthRouterWithMailAndLimits(t, sender, limits)
	token := signupWithEmailTestUser(t, router, "ana", "")
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"ana@example.com"}`))

	for i := 0; i < 3; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, authRequest(http.MethodPost, "/v1/auth/email/verifications", token, ""))
		if i < 2 {
			if response.Code != http.StatusAccepted {
				t.Fatalf("request %d status = %d, want 202", i, response.Code)
			}
		} else {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d status = %d, want 429", i, response.Code)
			}
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After header on rate-limited response")
			}
		}
	}
}

func TestSetAuthEmailStillAcceptedWhenMailUnavailable(t *testing.T) {
	sender := &recordingMailSender{sendErr: mail.ErrUnavailable}
	router := newSQLiteAuthRouterWithMail(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"ana@example.com"}`))
	if response.Code != http.StatusAccepted {
		t.Fatalf("set email status = %d, want 202 even when mail is unavailable", response.Code)
	}

	resend := httptest.NewRecorder()
	router.ServeHTTP(resend, authRequest(http.MethodPost, "/v1/auth/email/verifications", token, ""))
	if resend.Code != http.StatusAccepted {
		t.Fatalf("resend status = %d, want 202", resend.Code)
	}
}

func TestResendVerificationReturnsMailUnavailableWhenMailDisabled(t *testing.T) {
	// A nil sender models SDDS_MAIL_MODE=disabled. The guard must run before
	// the limiter, so every request returns 503 regardless of how many fire.
	router := newSQLiteAuthRouterWithMail(t, nil)
	token := signupWithEmailTestUser(t, router, "ana", "")

	for i := 0; i < 7; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, authRequest(http.MethodPost, "/v1/auth/email/verifications", token, ""))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("resend %d status = %d, want 503 (guard must precede the limiter)", i, response.Code)
		}
		var body openapi.ErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode resend error: %v", err)
		}
		if body.Code != openapi.ErrorCodeMailUnavailable {
			t.Fatalf("resend %d code = %s, want %s", i, body.Code, openapi.ErrorCodeMailUnavailable)
		}
	}
}

func TestVerifyAddressAlreadyVerifiedByAnotherReturnsInvalidToken(t *testing.T) {
	sender := &recordingMailSender{}
	router := newSQLiteAuthRouterWithMail(t, sender)

	tokenA := signupWithEmailTestUser(t, router, "ana", "")
	tokenB := signupWithEmailTestUser(t, router, "bruno", "")
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", tokenA, `{"email":"shared@example.com"}`))
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", tokenB, `{"email":"shared@example.com"}`))

	messages := sender.snapshot()
	verifyTokenA := tokenFromMessage(t, messages[0])
	verifyTokenB := tokenFromMessage(t, messages[1])

	verifyA := httptest.NewRecorder()
	router.ServeHTTP(verifyA, jsonRequest(http.MethodPost, "/v1/auth/email/verification", `{"token":"`+verifyTokenA+`"}`))
	if verifyA.Code != http.StatusNoContent {
		t.Fatalf("verify A status = %d, want 204", verifyA.Code)
	}

	verifyB := httptest.NewRecorder()
	router.ServeHTTP(verifyB, jsonRequest(http.MethodPost, "/v1/auth/email/verification", `{"token":"`+verifyTokenB+`"}`))
	if verifyB.Code != http.StatusBadRequest {
		t.Fatalf("verify B status = %d, want 400 (address already verified by another account)", verifyB.Code)
	}
}

type noopMailSender struct{}

func (noopMailSender) Send(context.Context, mail.Message) error { return nil }
