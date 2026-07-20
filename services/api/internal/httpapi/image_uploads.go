package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const imageUploadReadTimeout = 60 * time.Second

type imageUploadParseError struct {
	cause error
}

func (err *imageUploadParseError) Error() string {
	if err == nil || err.cause == nil {
		return ""
	}
	return err.cause.Error()
}

func (err *imageUploadParseError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func (handler server) PrepareImageUpload(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSessionFromContext(r.Context())
	if !ok {
		writeUnauthenticated(w)
		return
	}

	if r.Body == nil {
		r.Body = http.NoBody
	}
	defer closeImageUploadBodyOnCancellation(r.Context(), r.Body)()
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(imageUploadReadTimeout))

	receive := func(_ context.Context, destination io.Writer) (string, error) {
		requestID, err := parseImageUploadMultipart(r, destination)
		if err != nil {
			return "", &imageUploadParseError{cause: err}
		}
		return requestID, nil
	}
	receipt, err := handler.media.imageUploads.PrepareImageUpload(r.Context(), string(current.User.ID), receive)
	if err != nil {
		var parseErr *imageUploadParseError
		if errors.As(err, &parseErr) {
			writeImageUploadParseError(w, parseErr.cause)
			return
		}
		writeImageUploadServiceError(w, err)
		return
	}

	imageID, err := uuid.Parse(receipt.ImageUploadID)
	contentType := openapi.ImageUploadReceiptContentType(receipt.ContentType)
	if err != nil || !contentType.Valid() {
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
		return
	}
	writeJSON(w, http.StatusCreated, openapi.ImageUploadReceipt{
		ImageUploadId: imageID,
		ContentType:   contentType,
		ByteSize:      receipt.ByteSize,
		Width:         int32(receipt.Width),
		Height:        int32(receipt.Height),
		ExpiresAt:     receipt.ExpiresAt.UTC().UnixMilli(),
	})
}

func closeImageUploadBodyOnCancellation(ctx context.Context, body io.ReadCloser) func() {
	if body == nil {
		return func() {}
	}
	var once sync.Once
	closeBody := func() {
		once.Do(func() {
			_ = body.Close()
		})
	}
	stop := context.AfterFunc(ctx, closeBody)
	return func() {
		stop()
		closeBody()
	}
}

func writeImageUploadParseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrMediaTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
	case errors.Is(err, media.ErrInvalidUploadRequest):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: imageUploadValidationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldUploadRequestID, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, errInvalidImageUploadMultipart):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia})
	default:
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
			return
		}
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia})
	}
}

func writeImageUploadServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrInvalidUploadRequest):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: imageUploadValidationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldUploadRequestID, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrInvalidMedia), errors.Is(err, media.ErrMediaDimensions):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: imageUploadValidationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrUnsupportedMediaType):
		writeError(w, http.StatusUnsupportedMediaType, openapi.ErrorResponse{Code: openapi.ErrorCodeUnsupportedMediaType, Fields: imageUploadValidationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrMediaTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge, Fields: imageUploadValidationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrUploadIdempotencyConflict):
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeIdempotencyConflict})
	case errors.Is(err, media.ErrUploadInProgress):
		writeRetryableImageUploadError(w, http.StatusConflict, openapi.ErrorCodeUploadInProgress, media.RetryAfter(err))
	case errors.Is(err, media.ErrUploadExpired):
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeUploadExpired})
	case errors.Is(err, media.ErrUploadQuotaExceeded):
		writeRetryableImageUploadError(w, http.StatusTooManyRequests, openapi.ErrorCodeMediaStagingQuotaExceeded, media.RetryAfter(err))
	case errors.Is(err, media.ErrMediaStorageUnavailable):
		writeRetryableImageUploadError(w, http.StatusServiceUnavailable, openapi.ErrorCodeMediaStorageUnavailable, media.RetryAfter(err))
	case errors.Is(err, media.ErrMediaIntegrity):
		writeError(w, http.StatusConflict, openapi.ErrorResponse{Code: openapi.ErrorCodeMediaIntegrityError})
	default:
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
	}
}

func writeRetryableImageUploadError(w http.ResponseWriter, status int, code openapi.ErrorCode, retryAfter time.Duration) {
	seconds := int(retryAfter / time.Second)
	if retryAfter%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
	writeError(w, status, openapi.ErrorResponse{Code: code})
}

func imageUploadValidationFields(problem openapi.ValidationProblem) *[]openapi.ValidationProblem {
	problems := []openapi.ValidationProblem{problem}
	return &problems
}
