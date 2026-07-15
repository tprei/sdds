package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const uploadReadTimeout = 60 * time.Second

type uploadPreparer interface {
	Prepare(context.Context, string, media.UploadReceiver) (media.UploadReceipt, error)
}

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
	defer closeUploadBodyOnCancellation(r.Context(), r.Body)()
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadReadTimeout))
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxMultipartBodySize)

	receive := func(_ context.Context, destination io.Writer) (string, error) {
		requestID, err := parseImageUploadMultipart(r, destination)
		if err != nil {
			return "", &imageUploadParseError{cause: err}
		}
		return requestID, nil
	}
	receipt, err := handler.uploadService.Prepare(r.Context(), string(current.User.ID), receive)
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

func closeUploadBodyOnCancellation(ctx context.Context, body io.ReadCloser) func() {
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

func parseImageUploadMultipart(r *http.Request, destination io.Writer) (string, error) {
	if r == nil || r.Body == nil {
		return "", errInvalidImageUploadMultipart
	}
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return "", errInvalidImageUploadMultipart
	}
	if destination == nil {
		return "", errInvalidImageUploadMultipart
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	var (
		requestID string
		seenID    bool
		seenFile  bool
	)
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil || part == nil {
			if nextErr == nil {
				nextErr = errInvalidImageUploadMultipart
			}
			return "", nextErr
		}

		name, filename, hasFilename, dispositionErr := parseImageUploadPart(part)
		if dispositionErr != nil {
			return "", dispositionErr
		}
		switch name {
		case "upload_request_id":
			if seenID || hasFilename {
				return "", errInvalidImageUploadMultipart
			}
			value, valueErr := readUploadRequestID(part)
			if valueErr != nil {
				return "", valueErr
			}
			parsed, parseErr := uuid.Parse(value)
			if parseErr != nil || parsed.String() != value {
				return "", media.ErrInvalidUploadRequest
			}
			requestID = value
			seenID = true
		case "file":
			if seenFile || !hasFilename || filename == "" {
				return "", errInvalidImageUploadMultipart
			}
			if _, copyErr := io.Copy(destination, part); copyErr != nil {
				return "", copyErr
			}
			seenFile = true
		default:
			return "", errInvalidImageUploadMultipart
		}
	}

	if !seenID || !seenFile {
		return "", errInvalidImageUploadMultipart
	}
	return requestID, nil
}

func parseImageUploadPart(part *multipart.Part) (name, filename string, hasFilename bool, err error) {
	if part == nil {
		return "", "", false, errInvalidImageUploadMultipart
	}
	mediaType, params, parseErr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if parseErr != nil || mediaType != "form-data" {
		return "", "", false, errInvalidImageUploadMultipart
	}
	name, ok := params["name"]
	if !ok || name == "" {
		return "", "", false, errInvalidImageUploadMultipart
	}
	filename, hasFilename = params["filename"]
	return name, filename, hasFilename, nil
}

func readUploadRequestID(part *multipart.Part) (string, error) {
	value, err := io.ReadAll(io.LimitReader(part, 128))
	if err != nil {
		return "", err
	}
	if len(value) == 0 {
		return "", media.ErrInvalidUploadRequest
	}

	var extra [1]byte
	for {
		count, readErr := part.Read(extra[:])
		if count > 0 {
			return "", media.ErrInvalidUploadRequest
		}
		if errors.Is(readErr, io.EOF) {
			return string(value), nil
		}
		if readErr != nil {
			return "", readErr
		}
	}
}

var errInvalidImageUploadMultipart = errors.New("invalid image upload multipart body")

func writeImageUploadParseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrMediaTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge})
	case errors.Is(err, media.ErrInvalidUploadRequest):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: validationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldUploadRequestID, Code: openapi.ValidationProblemCodeInvalid})})
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
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: validationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldUploadRequestID, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrInvalidMedia), errors.Is(err, media.ErrMediaDimensions):
		writeError(w, http.StatusBadRequest, openapi.ErrorResponse{Code: openapi.ErrorCodeInvalidMedia, Fields: validationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrUnsupportedMediaType):
		writeError(w, http.StatusUnsupportedMediaType, openapi.ErrorResponse{Code: openapi.ErrorCodeUnsupportedMediaType, Fields: validationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
	case errors.Is(err, media.ErrMediaTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, openapi.ErrorResponse{Code: openapi.ErrorCodeRequestTooLarge, Fields: validationFields(openapi.ValidationProblem{Field: openapi.ValidationFieldFile, Code: openapi.ValidationProblemCodeInvalid})})
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

func validationFields(problem openapi.ValidationProblem) *[]openapi.ValidationProblem {
	problems := []openapi.ValidationProblem{problem}
	return &problems
}
