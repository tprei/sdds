package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/mail"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

// verifyEmailForReset drives an address through to verified state using the
// recording sender, returning the index past the last dispatched message.
func verifyEmailForReset(t *testing.T, router http.Handler, sender *recordingMailSender, token, address string) {
	t.Helper()
	before := len(sender.snapshot())
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"`+address+`"}`))
	// The dispatch may run on the production goroutine scheduler, so poll for
	// the verification message instead of reading the snapshot synchronously.
	deadline := time.Now().Add(2 * time.Second)
	var messages []mail.Message
	for time.Now().Before(deadline) {
		messages = sender.snapshot()
		if len(messages) > before {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(messages) <= before {
		t.Fatalf("verification message for %s was not dispatched within 2s", address)
	}
	verifyToken := tokenFromMessage(t, messages[len(messages)-1])
	verify := httptest.NewRecorder()
	router.ServeHTTP(verify, jsonRequest(http.MethodPost, "/v1/auth/email/verification", `{"token":"`+verifyToken+`"}`))
	if verify.Code != http.StatusNoContent {
		t.Fatalf("verify address status = %d, want 204", verify.Code)
	}
}

func TestPasswordResetFullFlowRevokesSessionsAndEnablesLogin(t *testing.T) {
	sender := &recordingMailSender{}
	router := newSQLiteAuthRouterWithMail(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")

	verifyEmailForReset(t, router, sender, token, "ana@example.com")

	resetResponse := httptest.NewRecorder()
	router.ServeHTTP(resetResponse, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ana@example.com"}`))
	if resetResponse.Code != http.StatusAccepted {
		t.Fatalf("reset request status = %d, want 202", resetResponse.Code)
	}
	messages := sender.snapshot()
	resetToken := tokenFromMessage(t, messages[len(messages)-1])

	setResponse := httptest.NewRecorder()
	router.ServeHTTP(setResponse, jsonRequest(http.MethodPost, "/v1/auth/password", `{"token":"`+resetToken+`","password":"nova-senha-1"}`))
	if setResponse.Code != http.StatusNoContent {
		t.Fatalf("set password status = %d, want 204", setResponse.Code)
	}

	// The old session is revoked.
	sessionResponse := httptest.NewRecorder()
	router.ServeHTTP(sessionResponse, authRequest(http.MethodGet, "/v1/auth/session", token, ""))
	if sessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old session status = %d, want 401 after reset", sessionResponse.Code)
	}

	// Login with the new password succeeds.
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, jsonRequest(http.MethodPost, "/v1/auth/sessions", `{"username":"ana","password":"nova-senha-1"}`))
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want 201", loginResponse.Code)
	}
}

func TestPasswordResetUnverifiedAddressDispatchesNothing(t *testing.T) {
	sender := &recordingMailSender{}
	router := newSQLiteAuthRouterWithMail(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")
	router.ServeHTTP(httptest.NewRecorder(), authRequest(http.MethodPut, "/v1/auth/email", token, `{"email":"ana@example.com"}`))

	before := len(sender.snapshot())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ana@example.com"}`))
	if response.Code != http.StatusAccepted {
		t.Fatalf("reset request status = %d, want 202", response.Code)
	}
	if got := len(sender.snapshot()) - before; got != 0 {
		t.Fatalf("dispatched %d reset messages for an unverified address, want 0", got)
	}
}

func TestPasswordResetRegisteredAndUnknownAreIndistinguishable(t *testing.T) {
	sender := &recordingMailSender{}
	router := newSQLiteAuthRouterWithMail(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")
	verifyEmailForReset(t, router, sender, token, "ana@example.com")

	registered := httptest.NewRecorder()
	router.ServeHTTP(registered, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ana@example.com"}`))
	unknown := httptest.NewRecorder()
	router.ServeHTTP(unknown, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ninguem@example.com"}`))

	if registered.Code != unknown.Code {
		t.Fatalf("status differs: registered %d, unknown %d", registered.Code, unknown.Code)
	}
	if registered.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs: registered %q, unknown %q", registered.Body.String(), unknown.Body.String())
	}
}

func TestPasswordResetTokenReplayRejected(t *testing.T) {
	sender := &recordingMailSender{}
	limits := DefaultAuthLimits()
	limits.PasswordResetRequestsPerMinute = 1000
	limits.PasswordResetGlobalRequestsPerMinute = 1000
	router := newSQLiteAuthRouterWithMailLimits(t, sender, limits)
	token := signupWithEmailTestUser(t, router, "ana", "")
	verifyEmailForReset(t, router, sender, token, "ana@example.com")

	router.ServeHTTP(httptest.NewRecorder(), jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ana@example.com"}`))
	resetToken := tokenFromMessage(t, sender.snapshot()[len(sender.snapshot())-1])

	first := httptest.NewRecorder()
	router.ServeHTTP(first, jsonRequest(http.MethodPost, "/v1/auth/password", `{"token":"`+resetToken+`","password":"nova-senha-1"}`))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first set password status = %d, want 204", first.Code)
	}
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, jsonRequest(http.MethodPost, "/v1/auth/password", `{"token":"`+resetToken+`","password":"outra-senha-1"}`))
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want 400", replay.Code)
	}
}

func TestPasswordResetRateLimitsByIdentifier(t *testing.T) {
	sender := &recordingMailSender{}
	limits := DefaultAuthLimits()
	limits.PasswordResetRequestsPerMinute = 2
	router := newSQLiteAuthRouterWithMailAndLimits(t, sender, limits)

	for i := 0; i < 3; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"limite@example.com"}`))
		if i < 2 {
			if response.Code != http.StatusAccepted {
				t.Fatalf("request %d status = %d, want 202", i, response.Code)
			}
		} else {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d status = %d, want 429", i, response.Code)
			}
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After header")
			}
		}
	}
}

func TestPasswordResetResponseNotDelayedBySlowSender(t *testing.T) {
	sender := newBlockingMailSender()
	// Schedule is nil so NewRouter uses the production goroutine dispatcher;
	// the detached send must not extend the client-visible response latency.
	router := newSQLiteAuthRouterWithMailAsync(t, sender)
	token := signupWithEmailTestUser(t, router, "ana", "")
	verifyEmailForReset(t, router, &sender.recordingMailSender, token, "ana@example.com")

	sender.arm()
	start := time.Now()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, jsonRequest(http.MethodPost, "/v1/auth/password-resets", `{"email":"ana@example.com"}`))
	elapsed := time.Since(start)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", response.Code)
	}
	if elapsed > time.Second {
		t.Fatalf("response took %s before the blocked send completed; dispatch must run off the request goroutine", elapsed)
	}

	sender.releaseBlocked()
	select {
	case <-sender.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the detached reset send to complete")
	}
}

type blockingMailSender struct {
	recordingMailSender
	block   bool
	mu      sync.Mutex
	release chan struct{}
	done    chan struct{}
}

func newBlockingMailSender() *blockingMailSender {
	return &blockingMailSender{release: make(chan struct{})}
}

// arm makes the next Send block until release is called, then signals done.
func (sender *blockingMailSender) arm() {
	sender.mu.Lock()
	sender.block = true
	sender.done = make(chan struct{})
	sender.release = make(chan struct{})
	sender.mu.Unlock()
}

func (sender *blockingMailSender) Send(ctx context.Context, message mail.Message) error {
	sender.mu.Lock()
	block := sender.block
	release := sender.release
	done := sender.done
	sender.mu.Unlock()
	// done is non-nil only while armed; signal completion either way so the
	// test does not depend on whether release ran before Send entered.
	if block {
		select {
		case <-release:
		case <-ctx.Done():
			closeIfOpen(done)
			return ctx.Err()
		}
	}
	err := sender.recordingMailSender.Send(ctx, message)
	closeIfOpen(done)
	return err
}

func (sender *blockingMailSender) releaseBlocked() {
	sender.mu.Lock()
	sender.block = false
	close(sender.release)
	sender.mu.Unlock()
}

func closeIfOpen(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}

var _ openapi.SetPasswordRequest
