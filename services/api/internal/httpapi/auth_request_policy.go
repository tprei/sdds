package httpapi

const (
	createAuthSessionGeneratedOperationID = "CreateAuthSession"
	createAuthUserGeneratedOperationID    = "CreateAuthUser"
	setAuthEmailGeneratedOperationID      = "SetAuthEmail"
	verifyAuthEmailGeneratedOperationID   = "VerifyAuthEmail"
)

func authRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	switch operationID {
	case createAuthSessionGeneratedOperationID, createAuthUserGeneratedOperationID, setAuthEmailGeneratedOperationID, verifyAuthEmailGeneratedOperationID:
		return requestValidationPolicy{maxBodyBytes: maxAuthRequestBytes}, true
	default:
		return requestValidationPolicy{}, false
	}
}
