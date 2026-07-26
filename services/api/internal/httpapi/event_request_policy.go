package httpapi

const createEventsGeneratedOperationID = "CreateEvents"

func eventRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	if operationID != createEventsGeneratedOperationID {
		return requestValidationPolicy{}, false
	}
	return requestValidationPolicy{maxBodyBytes: eventMaxBodyBytes, excludeRequestBody: true}, true
}
