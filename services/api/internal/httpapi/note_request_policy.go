package httpapi

import (
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const createNoteGeneratedOperationID = "CreateNote"

func noteRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	if operationID != createNoteGeneratedOperationID {
		return requestValidationPolicy{}, false
	}
	return requestValidationPolicy{maxBodyBytes: maxCreateNoteRequestBytes}, true
}

func isTooManyCreateNoteImages(err error) bool {
	var requestError *openapi3filter.RequestError
	if !errors.As(err, &requestError) || requestError.Input == nil ||
		requestError.Input.Route == nil || requestError.Input.Route.Operation == nil ||
		requestError.Input.Route.Operation.OperationID != createNoteGeneratedOperationID {
		return false
	}
	var schemaError *openapi3.SchemaError
	if !errors.As(err, &schemaError) || schemaError.SchemaField != "maxItems" {
		return false
	}
	path := schemaError.JSONPointer()
	return len(path) == 1 && path[0] == "image_upload_ids"
}

func generatedInvalidNoteParamError(path string, paramName string) (openapi.ErrorResponse, bool) {
	switch {
	case path == "/v1/notes" && paramName == "category_slug":
		return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidNote}, true
	case path == "/v1/search/notes" && (paramName == "q" || paramName == "category_slug"):
		return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidSearch}, true
	default:
		return openapi.ErrorResponse{}, false
	}
}

func writeTooManyCreateNoteImages(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeTooManyImages})
}
