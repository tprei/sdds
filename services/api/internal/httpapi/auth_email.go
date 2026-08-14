package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/tprei/sdds/services/api/internal/mail"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const mailDispatchTimeout = 15 * time.Second

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
	if handler.auth.mail == nil {
		writeError(w, http.StatusServiceUnavailable, openapi.ErrorResponse{Code: openapi.ErrorCodeMailUnavailable})
		return
	}

	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposeVerification, string(current.User.ID)); !allowed {
		writeRetryableRateLimited(w, retry)
		return
	}

	channel, err := handler.auth.contactChannels.UpsertUnverifiedEmail(r.Context(), current.User.ID, normalized, handler.auth.clock())
	if err != nil {
		log.Printf("upsert email for user %s: %v", current.User.ID, err)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	handler.auth.schedule(func() { handler.dispatchEmailVerification(channel) })
}

// CreateAuthEmailVerification resends the verification message for the current
// user's address. The response is identical whether or not a pending address
// exists; a message is dispatched only when an unverified address is on file.
func (handler server) CreateAuthEmailVerification(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}
	if handler.auth.mail == nil {
		writeError(w, http.StatusServiceUnavailable, openapi.ErrorResponse{Code: openapi.ErrorCodeMailUnavailable})
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposeVerification, string(current.User.ID)); !allowed {
		writeRetryableRateLimited(w, retry)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	handler.auth.schedule(func() {
		ctx, cancel := context.WithTimeout(context.Background(), mailDispatchTimeout)
		defer cancel()
		channel, err := handler.auth.contactChannels.FindPendingEmailForUser(ctx, current.User.ID)
		if err != nil {
			return
		}
		handler.dispatchEmailVerification(channel)
	})
}

// VerifyAuthEmail consumes a verification token and marks the address verified.
// An expired, reused, or unknown token reports 400 invalid_token.
func (handler server) VerifyAuthEmail(w http.ResponseWriter, r *http.Request) {
	var request openapi.VerifyEmailRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}
	if retry, allowed := handler.auth.rateLimiters.allow(r, authPurposeVerification, requestSourceKey(r)); !allowed {
		writeRetryableRateLimited(w, retry)
		return
	}

	now := handler.auth.clock()
	_, err := handler.auth.contactChannels.ConsumeTokenAndMarkVerified(r.Context(), user.HashContactChannelToken(request.Token), user.ContactChannelVerifiedViaToken, now)
	if errors.Is(err, user.ErrContactChannelTokenInvalid) || errors.Is(err, user.ErrContactChannelAlreadyVerified) {
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidToken})
		return
	}
	if err != nil {
		log.Printf("verify email: %v", err)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	noContent(w, r)
}

// dispatchEmailVerification mints a single-use token, stores its hash, and sends
// the verification message. It runs on a detached context because the request
// context is cancelled once the response is written. Every failure is logged
// and swallowed so delivery never breaks the calling response.
func (handler server) dispatchEmailVerification(channel user.ContactChannelRecord) {
	if handler.auth.mail == nil {
		return
	}
	token, err := user.NewContactChannelToken()
	if err != nil {
		log.Printf("mint verification token for user %s: %v", channel.UserID, err)
		return
	}
	tokenID, err := user.NewContactChannelTokenID()
	if err != nil {
		log.Printf("mint verification token id for user %s: %v", channel.UserID, err)
		return
	}
	now := handler.auth.clock()
	ctx, cancel := context.WithTimeout(context.Background(), mailDispatchTimeout)
	defer cancel()
	if err := handler.auth.contactChannels.CreateToken(ctx, user.CreateContactChannelTokenInput{
		ID:        tokenID,
		ChannelID: channel.ID,
		Purpose:   user.ContactChannelTokenPurposeVerify,
		TokenHash: user.HashContactChannelToken(token),
		CreatedAt: now,
		ExpiresAt: now.Add(user.EmailVerificationTokenLifetime),
	}); err != nil {
		log.Printf("store verification token for user %s: %v", channel.UserID, err)
		return
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", handler.auth.appBaseURL, token)
	if err := handler.auth.mail.Send(ctx, mail.Message{
		To:      channel.Value,
		Subject: "Confirme seu e-mail no sdds",
		Text: fmt.Sprintf(
			"Oi! Toque no link pra confirmar seu e-mail no sdds:\n\n%s\n\nO link vale por 24 horas. Se não foi você, é só ignorar.",
			link,
		),
	}); err != nil {
		log.Printf("send verification mail to %s: %v", channel.Value, err)
	}
}

// captureSignupEmail stores an address supplied at signup. A malformed or
// un-storable address never fails signup: validation problems and storage
// errors are logged and dropped.
func (handler server) captureSignupEmail(ctx context.Context, userID user.UserID, raw string) (user.ContactChannelRecord, bool) {
	normalized := user.NormalizeEmail(raw)
	if problems := user.ValidateEmail(normalized); len(problems) > 0 {
		return user.ContactChannelRecord{}, false
	}
	channel, err := handler.auth.contactChannels.UpsertUnverifiedEmail(ctx, userID, normalized, handler.auth.clock())
	if err != nil {
		log.Printf("capture signup email for user %s: %v", userID, err)
		return user.ContactChannelRecord{}, false
	}
	return channel, true
}
