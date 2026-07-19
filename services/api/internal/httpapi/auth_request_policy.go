package httpapi

const (
	createAuthSessionGeneratedOperationID = "CreateAuthSession"
	createAuthUserGeneratedOperationID    = "CreateAuthUser"
)

func authRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	switch operationID {
	case createAuthSessionGeneratedOperationID, createAuthUserGeneratedOperationID:
		return requestValidationPolicy{maxBodyBytes: maxAuthRequestBytes}, true
	default:
		return requestValidationPolicy{}, false
	}
}
