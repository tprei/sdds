package httpapi

import (
	"errors"
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
	if !handler.authRateLimiters.allowSignup(input.Username) {
		writeRateLimited(w)
		return
	}

	secretHash, err := handler.passwordHasher.Hash(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	token, err := handler.newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	expiresAt := handler.clock().Add(user.SessionLifetime)
	current, err := handler.users.CreatePasswordUser(r.Context(), user.CreatePasswordUserInput{
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

	writeJSON(w, http.StatusCreated, newAuthSessionResponse(current, token))
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
	if !handler.authRateLimiters.allowLogin(input.Username) {
		writeRateLimited(w)
		return
	}

	login, err := handler.users.FindPasswordLogin(r.Context(), input.Username)
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

	verified, err := handler.passwordHasher.Verify(input.Password, login.SecretHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	if !verified {
		writeInvalidCredentials(w)
		return
	}

	token, err := handler.newSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	expiresAt := handler.clock().Add(user.SessionLifetime)
	current, err := handler.users.CreateSession(r.Context(), user.CreateSessionInput{
		UserID:    login.User.ID,
		TokenHash: user.HashSessionToken(token),
		ExpiresAt: expiresAt,
	})
	if errors.Is(err, user.ErrUserDisabled) {
		writeInvalidCredentials(w)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}

	writeJSON(w, http.StatusCreated, newAuthSessionResponse(current, token))
}

func (handler server) GetAuthSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	writeJSON(w, http.StatusOK, newCurrentSessionResponse(current))
}

func (handler server) DeleteAuthSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if err := handler.users.RevokeSession(r.Context(), current.Session.ID, handler.clock()); err != nil {
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
	if _, err := handler.passwordHasher.Verify(password, handler.invalidCredentialHash); err != nil {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeInvalidCredentials(w)
}

func newAuthSessionResponse(current user.CurrentSession, token string) openapi.AuthSessionResponse {
	return openapi.AuthSessionResponse{
		Token:     token,
		ExpiresAt: current.Session.ExpiresAt.UTC().UnixMilli(),
		User:      newCurrentUserResponse(current),
	}
}

func newCurrentSessionResponse(current user.CurrentSession) openapi.CurrentSessionResponse {
	return openapi.CurrentSessionResponse{
		ExpiresAt: current.Session.ExpiresAt.UTC().UnixMilli(),
		User:      newCurrentUserResponse(current),
	}
}

func newCurrentUserResponse(current user.CurrentSession) openapi.CurrentUser {
	return openapi.CurrentUser{
		Id:       string(current.User.ID),
		Username: current.Username,
		Author: openapi.AuthorSummary{
			Id:          string(current.Author.ID),
			DisplayName: current.Author.DisplayName,
		},
	}
}
