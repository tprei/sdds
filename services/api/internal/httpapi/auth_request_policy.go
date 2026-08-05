package httpapi

const (
	createAuthSessionGeneratedOperationID = "CreateAuthSession"
	createAuthUserGeneratedOperationID    = "CreateAuthUser"
	setAuthEmailGeneratedOperationID      = "SetAuthEmail"
)

func authRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	switch operationID {
	case createAuthSessionGeneratedOperationID, createAuthUserGeneratedOperationID, setAuthEmailGeneratedOperationID:
		return requestValidationPolicy{maxBodyBytes: maxAuthRequestBytes}, true
	default:
		return requestValidationPolicy{}, false
	}
}
