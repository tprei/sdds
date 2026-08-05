package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const maxAuthRequestBytes int64 = 8 * 1024

func (handler server) CreateAuthUser(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreateUserRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}

	input := user.NormalizeCreateUserInput(user.CreateUserInput{
		Username:    request.Username,
		Password:    request.Password,
		DisplayName: request.DisplayName,
	})
	if problems := user.ValidateCreateUserInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidAuth, problems))
		return
	}
	if _, allowed := handler.auth.rateLimiters.allow(r, authPurposeSignup, input.Username); !allowed {
		writeRateLimited(w)
		return
	}

	secretHash, err := handler.auth.passwordHasher.Hash(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	token, err := handler.auth.newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	expiresAt := handler.auth.clock().Add(user.SessionLifetime)
	current, err := handler.auth.users.CreatePasswordUser(r.Context(), user.CreatePasswordUserInput{
		Username:    input.Username,
		DisplayName: input.DisplayName,
		SecretHash:  secretHash,
		TokenHash:   user.HashSessionToken(token),
		ExpiresAt:   expiresAt,
	})
	if errors.Is(err, user.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, usernameTakenResponse())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	if request.Email != nil && *request.Email != "" {
		handler.captureSignupEmail(r.Context(), current.User.ID, *request.Email)
	}
	writeJSON(w, http.StatusCreated, handler.newAuthSessionResponse(r.Context(), current, token))
}

func (handler server) CreateAuthSession(w http.ResponseWriter, r *http.Request) {
	var request openapi.CreateSessionRequest
	if !decodeJSONRequest(w, r, maxAuthRequestBytes, &request) {
		return
	}

	input := user.NormalizeLoginInput(user.LoginInput{
		Username: request.Username,
		Password: request.Password,
	})
	if problems := user.ValidateLoginInput(input); len(problems) > 0 {
		writeError(w, http.StatusBadRequest, authValidationErrorResponse(openapi.ErrorCodeInvalidAuth, problems))
		return
	}
	if _, allowed := handler.auth.rateLimiters.allow(r, authPurposeLogin, input.Username); !allowed {
		writeRateLimited(w)
		return
	}

	login, err := handler.auth.users.FindPasswordLogin(r.Context(), input.Username)
	if errors.Is(err, user.ErrInvalidCredentials) {
		handler.writeInvalidCredentialsAfterVerification(w, input.Password)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	if login.User.State != user.UserStateActive {
		handler.writeInvalidCredentialsAfterVerification(w, input.Password)
		return
	}

	verified, err := handler.auth.passwordHasher.Verify(input.Password, login.SecretHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	if !verified {
		writeInvalidCredentials(w)
		return
	}

	token, err := handler.auth.newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	expiresAt := handler.auth.clock().Add(user.SessionLifetime)
	current, err := handler.auth.users.CreateSession(r.Context(), user.CreateSessionInput{
		UserID:            login.User.ID,
		TokenHash:         user.HashSessionToken(token),
		ExpiresAt:         expiresAt,
		CredentialVersion: login.CredentialVersion,
		FenceCredential:   true,
	})
	if errors.Is(err, user.ErrUserDisabled) || errors.Is(err, user.ErrInvalidCredentials) {
		writeInvalidCredentials(w)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusCreated, handler.newAuthSessionResponse(r.Context(), current, token))
}

func (handler server) GetAuthSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	writeJSON(w, http.StatusOK, handler.newCurrentSessionResponse(r.Context(), current))
}

func (handler server) DeleteAuthSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if err := handler.auth.users.RevokeSession(r.Context(), current.Session.ID, handler.auth.clock()); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	noContent(w, r)
}

func authValidationErrorResponse(code openapi.ErrorCode, problems []user.ValidationProblem) openapi.ErrorResponse {
	fields := make([]openapi.ValidationProblem, 0, len(problems))
	for _, problem := range problems {
		fields = append(fields, openapi.ValidationProblem{
			Field: openapi.ValidationField(problem.Field),
			Code:  openapi.ValidationProblemCode(problem.Code),
		})
	}
	return openapi.ErrorResponse{Code: code, Fields: &fields}
}

func usernameTakenResponse() openapi.ErrorResponse {
	fields := []openapi.ValidationProblem{{
		Field: openapi.ValidationFieldUsername,
		Code:  openapi.ValidationProblemCodeTaken,
	}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeUsernameTaken, Fields: &fields}
}

func writeInvalidCredentials(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidAuth})
}

func (handler server) writeInvalidCredentialsAfterVerification(w http.ResponseWriter, password string) {
	if _, err := handler.auth.passwordHasher.Verify(password, handler.auth.invalidCredentialHash); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeInvalidCredentials(w)
}

func (handler server) newAuthSessionResponse(ctx context.Context, current user.CurrentSession, token string) openapi.AuthSessionResponse {
	return openapi.AuthSessionResponse{
		Token:     token,
		ExpiresAt: current.Session.ExpiresAt.UTC().UnixMilli(),
		User:      handler.newCurrentUserResponse(ctx, current),
	}
}

func (handler server) newCurrentSessionResponse(ctx context.Context, current user.CurrentSession) openapi.CurrentSessionResponse {
	return openapi.CurrentSessionResponse{
		ExpiresAt: current.Session.ExpiresAt.UTC().UnixMilli(),
		User:      handler.newCurrentUserResponse(ctx, current),
	}
}

func (handler server) newCurrentUserResponse(ctx context.Context, current user.CurrentSession) openapi.CurrentUser {
	response := openapi.CurrentUser{
		Id:       string(current.User.ID),
		Username: current.Username,
		Author: openapi.AuthorSummary{
			Id:          string(current.Author.ID),
			DisplayName: current.Author.DisplayName,
		},
	}
	channel, err := handler.auth.contactChannels.FindEmailForUser(ctx, current.User.ID)
	if err != nil {
		if !errors.Is(err, user.ErrContactChannelNotFound) {
			log.Printf("find email for user %s: %v", current.User.ID, err)
		}
		return response
	}
	response.Email = &openapi.UserEmail{Address: channel.Value, Verified: channel.VerifiedAt != nil}
	return response
}
