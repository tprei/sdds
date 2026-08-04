package httpapi

import (
	"context"
	"log"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

// SetAuthEmail stores a contact address for the current user as unverified.
// The address is accepted and a verification message is dispatched after the
// response. A bad address reports 400; everything else returns 202.
func (handler server) SetAuthEmail(w http.ResponseWriter, r *http.Request) {
	var request openapi.SetUserEmailRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}

	normalized := user.NormalizeEmail(request.Email)
	if problems := user.ValidateEmail(normalized); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidEmail, problems))
		return
	}

	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if _, allowed := handler.auth.rateLimiters.allow(r, authPurposeVerification, string(current.User.ID)); !allowed {
		writeRateLimited(w)
		return
	}

	if _, err := handler.auth.contactChannels.UpsertUnverifiedEmail(r.Context(), current.User.ID, normalized, handler.auth.clock()); err != nil {
		log.Printf("upsert email for user %s: %v", current.User.ID, err)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// captureSignupEmail stores an address supplied at signup. A malformed or
// un-storable address never fails signup: validation problems and storage
// errors are logged and dropped.
func (handler server) captureSignupEmail(ctx context.Context, userID user.UserID, raw string) {
	normalized := user.NormalizeEmail(raw)
	if problems := user.ValidateEmail(normalized); len(problems) > 0 {
		return
	}
	if _, err := handler.auth.contactChannels.UpsertUnverifiedEmail(ctx, userID, normalized, handler.auth.clock()); err != nil {
		log.Printf("capture signup email for user %s: %v", userID, err)
	}
}
