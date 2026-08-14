package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

type currentSessionContextKey struct{}

func requireAuth(users user.Store, clock func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, ok := authenticateRequest(users, clock, w, r)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// optionalAuth attaches the current session when a bearer token is present, and
// passes the request through untouched when no Authorization header is supplied.
// A present-but-invalid token still yields 401 unauthenticated: callers (including
// the mobile app's session-expiry detection) rely on 401 rather than a silent
// downgrade to anonymous.
func optionalAuth(users user.Store, clock func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx, ok := authenticateRequest(users, clock, w, r)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateRequest resolves the bearer session, writing the error response and
// reporting false when the request cannot be authenticated.
func authenticateRequest(users user.Store, clock func() time.Time, w http.ResponseWriter, r *http.Request) (context.Context, bool) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeUnauthenticated(w)
		return nil, false
	}
	current, err := users.FindCurrentSession(r.Context(), user.HashSessionToken(token), clock())
	if err != nil {
		if isUnauthenticatedSessionError(err) {
			writeUnauthenticated(w)
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return nil, false
	}
	return context.WithValue(r.Context(), currentSessionContextKey{}, current), true
}

// viewerUserID returns the authenticated viewer's id, or an empty id with ok=false
// when the request carries no session (anonymous public read).
func viewerUserID(ctx context.Context) (user.UserID, bool) {
	current, ok := currentSessionFromContext(ctx)
	if !ok {
		return "", false
	}
	return current.User.ID, true
}

func currentSessionFromContext(ctx context.Context) (user.CurrentSession, bool) {
	current, ok := ctx.Value(currentSessionContextKey{}).(user.CurrentSession)
	return current, ok
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func isUnauthenticatedSessionError(err error) bool {
	return errors.Is(err, user.ErrSessionNotFound) ||
		errors.Is(err, user.ErrSessionExpired) ||
		errors.Is(err, user.ErrSessionRevoked) ||
		errors.Is(err, user.ErrUserDisabled)
}

func writeUnauthenticated(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, openapi.ErrorResponse{Code: openapi.ErrorCodeUnauthenticated})
}
