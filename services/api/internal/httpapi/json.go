package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type errorCode string

const (
	errorCodeInternal        errorCode = "internal_error"
	errorCodeInvalidJSON     errorCode = "invalid_json"
	errorCodeInvalidNote     errorCode = "invalid_note"
	errorCodeRequestTooLarge errorCode = "request_too_large"
)

type errorResponse struct {
	Code   errorCode                   `json:"code"`
	Fields []validationProblemResponse `json:"fields,omitempty"`
}

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
		writeError(w, http.StatusRequestEntityTooLarge, errorResponse{Code: errorCodeRequestTooLarge})
		return
	}

	writeError(w, http.StatusBadRequest, errorResponse{Code: errorCodeInvalidJSON})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, body errorResponse) {
	writeJSON(w, status, body)
}
