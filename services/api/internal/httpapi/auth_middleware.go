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
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeUnauthenticated(w)
				return
			}

			current, err := users.FindCurrentSession(r.Context(), user.HashSessionToken(token), clock())
			if err != nil {
				if isUnauthenticatedSessionError(err) {
					writeUnauthenticated(w)
					return
				}
				writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
				return
			}

			ctx := context.WithValue(r.Context(), currentSessionContextKey{}, current)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
