package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tprei/sdds/services/api/internal/oidc"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/sqlite"
)

// newSQLiteOIDCAuthRouter mirrors newSQLiteAuthRouter with a provider
// verifier injected, so the sign-in flow runs against real stores.
func newSQLiteOIDCAuthRouter(t *testing.T, verifier oidc.Verifier) (http.Handler, *sql.DB) {
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
	router := NewRouter(
		NotesDependencies{Stores: fakeNoteStore{}, Publisher: fakeNoteStore{}, Searcher: fakeNoteStore{}, Catalog: fakeCatalog{}},
		CommentDependencies{Store: fakeCommentStore{}},
		ReportDependencies{Store: fakeReportStore{}, CommentTargets: fakeCommentStore{}},
		EventDependencies{Store: fakeEventStore{}, Limits: DefaultEventLimits()},
		AuthDependencies{Users: sqlite.NewUserStore(db), ContactChannels: sqlite.NewContactChannelStore(db), Mail: &recordingMailSender{}, Schedule: func(fn func()) { fn() }, Limits: oidcTestLimits(), OIDC: verifier},
		MediaDependencies{ImageUploads: fakeUploadPreparer{}, AttachedImages: fakeAttachedImageReader{}},
		SystemDependencies{Readiness: fakeReadiness{}},
		PublicReadDependencies{},
	)
	return router, db
}

// TestCreateAuthOidcSessionTwoRequestFlow proves the provider sign-in flow end
// to end against the real router and real stores: a new subject is asked for a
// username, the identical token reposted with one creates the account, and the
// identical token again signs the same user in with no username.
func TestCreateAuthOidcSessionTwoRequestFlow(t *testing.T) {
	const tokenBody = `{"provider":"google","id_token":"id-token","nonce":"request-nonce"`
	router, db := newSQLiteOIDCAuthRouter(t, fakeOIDCVerifier{verify: func(_ context.Context, provider oidc.Provider, _ string, _ string) (oidc.Identity, error) {
		return oidc.Identity{
			Provider:      provider,
			Subject:       "subject-1",
			Email:         "person@example.com",
			EmailVerified: true,
		}, nil
	}})
	ctx := context.Background()

	first := postOIDCSession(router, tokenBody+"}")
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), first)
	requireOIDCError(t, first, http.StatusConflict, openapi.ErrorCodeUsernameRequired)
	var userCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("users = %d after username_required, want none", userCount)
	}

	second := postOIDCSession(router, tokenBody+`,"username":"ana"}`)
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), second)
	if second.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", second.Code, second.Body.String())
	}
	var created openapi.AuthSessionResponse
	if err := json.Unmarshal(second.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if created.User.Username != "ana" {
		t.Fatalf("username = %q, want ana", created.User.Username)
	}
	if created.Token == "" {
		t.Fatal("token is empty")
	}

	third := postOIDCSession(router, tokenBody+"}")
	requireOpenAPIResponse(t, jsonRequest(http.MethodPost, "/v1/auth/oidc/sessions", ""), third)
	if third.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", third.Code, third.Body.String())
	}
	var returning openapi.AuthSessionResponse
	if err := json.Unmarshal(third.Body.Bytes(), &returning); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if returning.User.Id != created.User.Id {
		t.Fatalf("returning user = %s, want the created user %s", returning.User.Id, created.User.Id)
	}

	var localKind, localIdentifier string
	if err := db.QueryRowContext(ctx, `SELECT kind, normalized_identifier FROM user_login_identities WHERE user_id = ? AND provider = 'local'`, created.User.Id).Scan(&localKind, &localIdentifier); err != nil {
		t.Fatalf("read local identity: %v", err)
	}
	if localKind != "oidc" || localIdentifier != "ana" {
		t.Fatalf("local identity = (%s, %s), want (oidc, ana)", localKind, localIdentifier)
	}
	var providerIdentity int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_login_identities WHERE user_id = ? AND kind = 'oidc' AND provider = 'google' AND normalized_identifier = 'subject-1'`, created.User.Id).Scan(&providerIdentity); err != nil {
		t.Fatalf("count google identity: %v", err)
	}
	if providerIdentity != 1 {
		t.Fatalf("google identity rows = %d, want exactly one", providerIdentity)
	}

	session := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	request.Header.Set("Authorization", "Bearer "+created.Token)
	router.ServeHTTP(session, request)
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, want 200", session.Code)
	}
}
