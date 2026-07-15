package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func (handler server) GetMediaImage(w http.ResponseWriter, r *http.Request, imageID openapi_types.UUID) {
	parsedID := uuid.UUID(imageID)
	canonicalID := parsedID.String()

	image, err := handler.imageReader.OpenAttached(r.Context(), canonicalID)
	if err != nil {
		writeMediaImageError(w, err)
		return
	}
	if image.Body == nil || image.Size <= 0 || !validImageContentType(image.ContentType) {
		closeAttachedImage(image.Body)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeMediaIntegrityError})
		return
	}
	defer closeAttachedImage(image.Body)

	digest := hex.EncodeToString(image.SHA256[:])
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(image.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+digest+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)

	hasher := sha256.New()
	written, streamErr := io.CopyN(io.MultiWriter(w, hasher), image.Body, image.Size)
	streamFailed := streamErr != nil || written != image.Size
	if !streamFailed {
		var extra [1]byte
		extraBytes, extraErr := io.ReadFull(image.Body, extra[:])
		streamFailed = extraBytes != 0 || !errors.Is(extraErr, io.EOF)
	}
	streamFailed = streamFailed || !bytes.Equal(hasher.Sum(nil), image.SHA256[:])
	if streamFailed {
		slog.Error("attached image stream failed", "image_id", canonicalID)
	}
}

func writeMediaImageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrImageNotFound):
		writeError(w, http.StatusNotFound, openapi.ErrorResponse{Code: openapi.ErrorCodeNotFound})
	case errors.Is(err, media.ErrMediaStorageUnavailable):
		writeRetryableImageUploadError(w, http.StatusServiceUnavailable, openapi.ErrorCodeMediaStorageUnavailable, media.RetryAfter(err))
	case errors.Is(err, media.ErrMediaIntegrity):
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeMediaIntegrityError})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeRetryableImageUploadError(w, http.StatusServiceUnavailable, openapi.ErrorCodeMediaStorageUnavailable, media.RetryAfter(media.ErrMediaStorageUnavailable))
	default:
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeInternal})
	}
}

func validImageContentType(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png"
}

func closeAttachedImage(body io.ReadCloser) {
	if body != nil {
		_ = body.Close()
	}
}
