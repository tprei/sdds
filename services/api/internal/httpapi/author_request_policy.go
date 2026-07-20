package httpapi

import (
	"strings"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

func generatedInvalidAuthorNotesParamError(path string, paramName string) (openapi.ErrorResponse, bool) {
	if !strings.HasPrefix(path, "/v1/authors/") || !strings.HasSuffix(path, "/notes") {
		return openapi.ErrorResponse{}, false
	}
	if paramName != "limit" && paramName != "cursor" {
		return openapi.ErrorResponse{}, false
	}
	fields := []openapi.ValidationProblem{{
		Field: openapi.ValidationField(paramName),
		Code:  openapi.ValidationProblemCodeInvalid,
	}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidNote, Fields: &fields}, true
}
