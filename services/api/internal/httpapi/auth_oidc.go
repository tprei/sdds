package httpapi

import (
	"errors"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/oidc"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

// CreateAuthOidcSession exchanges a provider-issued identity token for a
// session. The token is verified against the provider's public keys and then
// discarded; it is never stored and never used as a session credential. A new
// subject that has not chosen a username yet is answered with
// 409 username_required, and the client reposts the same token with one.
func (handler server) CreateAuthOidcSession(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreateOidcSessionRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}
	if handler.auth.oidc == nil {
		writeError(w, http.StatusServiceUnavailable, openapi.ErrorResponse{Code: openapi.ErrorCodeOidcUnavailable})
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposeOIDC, requestSourceKey(r)); !allowed {
		writeRetryableRateLimited(w, retry)
		return
	}

	username := ""
	if request.Username != nil {
		username = user.NormalizeUsername(*request.Username)
		if problems := user.ValidateUsername(username); len(problems) > 0 {
			writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidAuth, problems))
			return
		}
	}

	identity, err := handler.auth.oidc.Verify(r.Context(), oidc.Provider(request.Provider), request.IdToken, request.Nonce)
	if errors.Is(err, oidc.ErrInvalidToken) {
		writeInvalidCredentials(w)
		return
	}
	if errors.Is(err, oidc.ErrUnavailable) {
		writeError(w, http.StatusServiceUnavailable, openapi.ErrorResponse{Code: openapi.ErrorCodeOidcUnavailable})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	token, err := handler.auth.newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	current, err := handler.auth.users.ResolveOIDCIdentity(r.Context(), user.ResolveOIDCIdentityInput{
		Provider:      string(identity.Provider),
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   identity.DisplayName,
		Username:      username,
		TokenHash:     user.HashSessionToken(token),
		ExpiresAt:     handler.auth.clock().Add(user.SessionLifetime),
	})
	if errors.Is(err, user.ErrUsernameRequired) {
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeUsernameRequired})
		return
	}
	if errors.Is(err, user.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, usernameTakenResponse())
		return
	}
	if errors.Is(err, user.ErrUserDisabled) || errors.Is(err, user.ErrInvalidCredentials) {
		writeInvalidCredentials(w)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	session, err := handler.newAuthSessionResponse(r.Context(), current, token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

// DeleteAuthIdentity disconnects one login identity from the authenticated
// account. The last remaining way in is refused, and an identity owned by
// another user answers the same 404 as a nonexistent one so the route cannot
// be used to probe for identity IDs.
func (handler server) DeleteAuthIdentity(w http.ResponseWriter, r *http.Request, identityID string) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	err := handler.auth.users.DeleteLoginIdentity(r.Context(), current.User.ID, user.LoginIdentityID(identityID))
	if errors.Is(err, user.ErrLoginIdentityNotFound) {
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
		return
	}
	if errors.Is(err, user.ErrLastLoginIdentity) {
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeLastSignInMethod})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	noContent(w, r)
}
