package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/mail"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

// CreateAuthPasswordReset always responds 202 for a well-formed address. The
// response is written before any lookup, so registered and unregistered
// addresses are indistinguishable in body, status, and timing. A reset message
// is dispatched only when a verified address matches.
func (handler server) CreateAuthPasswordReset(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreatePasswordResetRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}
	normalized := user.NormalizeEmail(request.Email)
	if problems := user.ValidateEmail(normalized); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidEmail, problems))
		return
	}
	if handler.auth.mail == nil {
		writeError(w, http.StatusServiceUnavailable, openapi.ErrorResponse{Code: openapi.ErrorCodeMailUnavailable})
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposePasswordReset, normalized); !allowed {
		writeAuthRateLimited(w, retry)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	handler.auth.schedule(func() {
		ctx, cancel := context.WithTimeout(context.Background(), mailDispatchTimeout)
		defer cancel()
		channel, err := handler.auth.contactChannels.FindVerifiedEmail(ctx, normalized)
		if err != nil {
			return
		}
		handler.dispatchPasswordReset(channel)
	})
}

// SetAuthPassword consumes a reset token, sets the password credential, and
// revokes all sessions. An account without a password identity gains one
// through the same path.
func (handler server) SetAuthPassword(w http.ResponseWriter, r *http.Request) {
	var request openapi.SetPasswordRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}
	if problems := user.ValidatePassword(request.Password); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidAuth, problems))
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposePasswordReset, requestSourceKey(r)); !allowed {
		writeAuthRateLimited(w, retry)
		return
	}

	now := handler.auth.clock()
	secretHash, err := handler.auth.passwordHasher.Hash(request.Password)
	if err != nil {
		log.Printf("hash reset password: %v", err)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	if _, err := handler.auth.contactChannels.ConsumeTokenAndSetPassword(r.Context(), user.HashContactChannelToken(request.Token), secretHash, now); err != nil {
		if errors.Is(err, user.ErrContactChannelTokenInvalid) {
			writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidToken})
			return
		}
		log.Printf("set password: %v", err)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	noContent(w, r)
}

// dispatchPasswordReset mints a single-use reset token, stores its hash, and
// sends the reset message on a detached context. Failures are logged and
// swallowed so delivery never affects the response.
func (handler server) dispatchPasswordReset(channel user.ContactChannelRecord) {
	if handler.auth.mail == nil {
		return
	}
	token, err := user.NewContactChannelToken()
	if err != nil {
		log.Printf("mint reset token for user %s: %v", channel.UserID, err)
		return
	}
	tokenID, err := user.NewContactChannelTokenID()
	if err != nil {
		log.Printf("mint reset token id for user %s: %v", channel.UserID, err)
		return
	}
	now := handler.auth.clock()
	ctx, cancel := context.WithTimeout(context.Background(), mailDispatchTimeout)
	defer cancel()
	if err := handler.auth.contactChannels.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID:        tokenID,
		ChannelID: channel.ID,
		Purpose:   user.ContactChannelTokenPurposeReset,
		TokenHash: user.HashContactChannelToken(token),
		CreatedAt: now,
		ExpiresAt: now.Add(user.PasswordResetTokenLifetime),
	}); err != nil {
		log.Printf("store reset token for user %s: %v", channel.UserID, err)
		return
	}
	link := fmt.Sprintf("%s/new-password?token=%s", handler.auth.appBaseURL, token)
	if err := handler.auth.mail.Send(ctx, mail.Message{
		To:      channel.Value,
		Subject: "Recuperar sua senha no sdds",
		Text: fmt.Sprintf(
			"Oi! Toque no link pra criar uma senha nova no sdds:\n\n%s\n\nO link vale por 1 hora. Se não foi você que pediu, é só ignorar.",
			link,
		),
	}); err != nil {
		log.Printf("send reset mail to %s: %v", channel.Value, err)
	}
}
