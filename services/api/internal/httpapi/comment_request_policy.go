package httpapi

import (
	"errors"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const createNoteCommentGeneratedOperationID = "CreateNoteComment"
const createCommentReplyGeneratedOperationID = "CreateCommentReply"

func commentRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	if operationID != createNoteCommentGeneratedOperationID && operationID != createCommentReplyGeneratedOperationID {
		return requestValidationPolicy{}, false
	}
	return requestValidationPolicy{maxBodyBytes: maxCreateCommentRequestBytes}, true
}

func invalidCreateCommentBody(err error) (openapi.ErrorResponse, bool) {
	var requestError *openapi3filter.RequestError
	if !errors.As(err, &requestError) || requestError.Input == nil ||
		requestError.Input.Route == nil || requestError.Input.Route.Operation == nil ||
		(requestError.Input.Route.Operation.OperationID != createNoteCommentGeneratedOperationID &&
			requestError.Input.Route.Operation.OperationID != createCommentReplyGeneratedOperationID) {
		return openapi.ErrorResponse{}, false
	}
	var schemaError *openapi3.SchemaError
	if !errors.As(err, &schemaError) {
		return openapi.ErrorResponse{}, false
	}
	path := schemaError.JSONPointer()
	if len(path) != 1 || path[0] != "body" {
		return openapi.ErrorResponse{}, false
	}

	var code openapi.ValidationProblemCode
	switch schemaError.SchemaField {
	case "required", "minLength":
		code = openapi.ValidationProblemCodeRequired
	case "maxLength":
		code = openapi.ValidationProblemCodeTooLong
	default:
		return openapi.ErrorResponse{}, false
	}
	fields := []openapi.ValidationProblem{{Field: openapi.ValidationFieldBody, Code: code}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidComment, Fields: &fields}, true
}

func generatedInvalidCommentParamError(path string, paramName string) (openapi.ErrorResponse, bool) {
	if !strings.HasPrefix(path, "/v1/notes/") || !strings.HasSuffix(path, "/comments") {
		return openapi.ErrorResponse{}, false
	}
	if paramName != "limit" && paramName != "cursor" {
		return openapi.ErrorResponse{}, false
	}
	fields := []openapi.ValidationProblem{{
		Field: openapi.ValidationField(paramName),
		Code:  openapi.ValidationProblemCodeInvalid,
	}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidComment, Fields: &fields}, true
}
