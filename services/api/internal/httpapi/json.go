package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeDecodeError(w, err)
		return false
	}

	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeDecodeError(w, err)
		return false
	}

	return true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
		return
	}

	writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidJSON})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, body openapi.ErrorResponse) {
	writeJSON(w, status, body)
}
