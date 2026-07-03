package httpapi

import (
	"errors"
	"net/http"

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

			if r.Method == http.MethodPost && r.URL.Path == "/v1/notes" {
				r.Body = http.MaxBytesReader(w, r.Body, maxCreateNoteRequestBytes)
			}

			err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    &options,
			})
			if err != nil {
				writeOpenAPIRequestValidationError(w, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeOpenAPIRequestValidationError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
		return
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func requestHasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}
