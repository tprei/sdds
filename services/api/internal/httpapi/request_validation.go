package httpapi

import (
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func openAPIRequestValidator() func(http.Handler) http.Handler {
	spec, err := openapi.GetSpec()
	if err != nil {
		panic(err)
	}
	spec.Servers = nil

	router, err := legacy.NewRouter(spec)
	if err != nil {
		panic(err)
	}

	options := openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if route.Operation.RequestBody == nil && requestHasBody(r) {
				writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
				return
			}

			policy, hasPolicy := requestValidationPolicyForOperation(route.Operation.OperationID)
			if hasPolicy && policy.maxBodyBytes > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, policy.maxBodyBytes)
			}

			requestOptions := options
			validationRoute := route
			if hasPolicy && policy.excludeRequestBody {
				// The application multipart parser is the sole consumer of this bounded body.
				// Kin-openapi security validation also buffers it, despite ExcludeRequestBody.
				// Auth middleware already verified this route, so never consume, buffer, or substitute it here.
				requestOptions.ExcludeRequestBody = true
				operation := *route.Operation
				security := openapi3.SecurityRequirements{}
				operation.Security = &security
				routeCopy := *route
				routeCopy.Operation = &operation
				validationRoute = &routeCopy
			}
			err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      validationRoute,
				Options:    &requestOptions,
			})
			if err != nil {
				writeOpenAPIRequestValidationError(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeOpenAPIRequestValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
		return
	}
	if isTooManyCreateNoteImages(err) {
		writeTooManyCreateNoteImages(w)
		return
	}

	var requestError *openapi3filter.RequestError
	if errors.As(err, &requestError) && requestError.Parameter != nil {
		if response, ok := generatedOpenAPIParameterError(r.URL.Path, requestError.Parameter.Name); ok {
			writeError(w, http.StatusBadRequest, response)
			return
		}
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func writeGeneratedOpenAPIError(w http.ResponseWriter, r *http.Request, err error) {
	var invalidParamError *openapi.InvalidParamFormatError
	if errors.As(err, &invalidParamError) {
		if response, ok := generatedOpenAPIParameterError(r.URL.Path, invalidParamError.ParamName); ok {
			writeError(w, http.StatusBadRequest, response)
			return
		}
	}
	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

type requestValidationPolicy struct {
	maxBodyBytes       int64
	excludeRequestBody bool
}

func requestValidationPolicyForOperation(operationID string) (requestValidationPolicy, bool) {
	if policy, ok := authRequestValidationPolicy(operationID); ok {
		return policy, true
	}
	if policy, ok := noteRequestValidationPolicy(operationID); ok {
		return policy, true
	}
	return mediaRequestValidationPolicy(operationID)
}

func generatedOpenAPIParameterError(path string, paramName string) (openapi.ErrorResponse, bool) {
	if response, ok := generatedInvalidAuthorNotesParamError(path, paramName); ok {
		return response, true
	}
	if response, ok := generatedInvalidNoteParamError(path, paramName); ok {
		return response, true
	}
	return generatedInvalidMediaParamError(path, paramName)
}

func requestHasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}
