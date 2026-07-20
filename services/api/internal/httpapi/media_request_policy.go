package httpapi

import (
	"strings"

	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const prepareImageUploadGeneratedOperationID = "PrepareImageUpload"

func mediaRequestValidationPolicy(operationID string) (requestValidationPolicy, bool) {
	if operationID != prepareImageUploadGeneratedOperationID {
		return requestValidationPolicy{}, false
	}
	return requestValidationPolicy{
		maxBodyBytes:       media.MaxMultipartBodySize,
		excludeRequestBody: true,
	}, true
}

func generatedInvalidMediaParamError(path string, paramName string) (openapi.ErrorResponse, bool) {
	if strings.HasPrefix(path, "/v1/media/images/") && paramName == "image_id" {
		return openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia}, true
	}
	return openapi.ErrorResponse{}, false
}
