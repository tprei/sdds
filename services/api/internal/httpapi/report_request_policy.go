package httpapi

import (
	"errors"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const createReportGeneratedOperationID = "CreateReport"

func reportRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	if operationID != createReportGeneratedOperationID {
		return requestValidationPolicy{}, false
	}
	return requestValidationPolicy{maxBodyBytes: maxCreateReportRequestBytes}, true
}

// invalidCreateReportBody maps required/minLength/maxLength/enum schema failures
// on the four report fields to invalid_report. Type and shape failures (wrong
// JSON value type, unknown properties, trailing JSON) fall through to invalid_json.
func invalidCreateReportBody(err error) (openapi.ErrorResponse, bool) {
	var requestError *openapi3filter.RequestError
	if !errors.As(err, &requestError) || requestError.Input == nil ||
		requestError.Input.Route == nil || requestError.Input.Route.Operation == nil ||
		requestError.Input.Route.Operation.OperationID != createReportGeneratedOperationID {
		return openapi.ErrorResponse{}, false
	}
	var schemaError *openapi3.SchemaError
	if !errors.As(err, &schemaError) {
		return openapi.ErrorResponse{}, false
	}

	field, ok := reportValidationField(schemaError.JSONPointer())
	if !ok {
		return openapi.ErrorResponse{}, false
	}

	var code openapi.ValidationProblemCode
	switch schemaError.SchemaField {
	case "required", "minLength":
		code = openapi.ValidationProblemCodeRequired
	case "maxLength":
		code = openapi.ValidationProblemCodeTooLong
	case "enum":
		code = openapi.ValidationProblemCodeInvalid
	default:
		return openapi.ErrorResponse{}, false
	}

	fields := []openapi.ValidationProblem{{Field: field, Code: code}}
	return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidReport, Fields: &fields}, true
}

// reportValidationField resolves the failing report field from the schema error
// JSON pointer. The pointer is relative to the request body object, so a failing
// property is a single-segment pointer naming one of the four report fields.
func reportValidationField(path []string) (openapi.ValidationField, bool) {
	if len(path) != 1 {
		return "", false
	}
	switch path[0] {
	case "target_type":
		return openapi.ValidationFieldTargetType, true
	case "target_id":
		return openapi.ValidationFieldTargetID, true
	case "reason":
		return openapi.ValidationFieldReason, true
	case "details":
		return openapi.ValidationFieldDetails, true
	default:
		return "", false
	}
}
