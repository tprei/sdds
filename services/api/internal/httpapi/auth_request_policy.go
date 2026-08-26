package httpapi

const (
	createAuthSessionGeneratedOperationID       = "CreateAuthSession"
	createAuthUserGeneratedOperationID          = "CreateAuthUser"
	setAuthEmailGeneratedOperationID            = "SetAuthEmail"
	verifyAuthEmailGeneratedOperationID         = "VerifyAuthEmail"
	createAuthPasswordResetGeneratedOperationID = "CreateAuthPasswordReset"
	setAuthPasswordGeneratedOperationID         = "SetAuthPassword"
	createAuthOidcSessionGeneratedOperationID   = "CreateAuthOidcSession"
)

func authRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	switch operationID {
	case createAuthSessionGeneratedOperationID, createAuthUserGeneratedOperationID, setAuthEmailGeneratedOperationID, verifyAuthEmailGeneratedOperationID, createAuthPasswordResetGeneratedOperationID, setAuthPasswordGeneratedOperationID, createAuthOidcSessionGeneratedOperationID:
		return requestValidationPolicy{maxBodyBytes: maxAuthRequestBytes}, true
	default:
		return requestValidationPolicy{}, false
	}
}
