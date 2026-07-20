package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const (
	imageReadScratchPrefix    = "sdds-image-read-"
	imageReadSlotCount        = 2
	imageResponseWriteTimeout = media.DefaultOperationTimeout
)

var imageReadSlots = make(chan struct{}, imageReadSlotCount)

type imageBody struct {
	body     io.ReadCloser
	once     sync.Once
	closeErr error
}

type imageSpool struct {
	file       *os.File
	once       sync.Once
	cleanupErr error
}

type imageSpoolWriter struct {
	file   *os.File
	hasher hash.Hash
	err    error
}

func (handler server) GetMediaImage(w http.ResponseWriter, r *http.Request, imageID openapi_types.UUID) {
	parsedID := uuid.UUID(imageID)
	canonicalID := parsedID.String()

	releaseReadSlot, err := acquireImageReadSlot(r.Context())
	if err != nil {
		writeMediaImageError(w, err)
		return
	}
	defer releaseReadSlot()

	image, err := handler.imageReader.OpenAttached(r.Context(), canonicalID)
	if err != nil {
		writeMediaImageError(w, err)
		return
	}
	if image.Body == nil || image.Size <= 0 || image.Size > media.MaxEncodedImageSize || !validImageContentType(image.ContentType) {
		closeAttachedImage(image.Body)
		writeError(w, http.StatusInternalServerError, openapi.ErrorResponse{Code: openapi.ErrorCodeMediaIntegrityError})
		return
	}

	body, stopBodyWatch := watchImageBody(r.Context(), image.Body)
	defer stopBodyWatch()

	spool, err := newImageSpool(handler.imageReadScratchDir)
	if err != nil {
		stopBodyWatch()
		writeMediaImageError(w, err)
		return
	}
	defer func() {
		if cleanupErr := spool.CloseRemove(); cleanupErr != nil {
			slog.Error("attached image scratch cleanup failed", "image_id", canonicalID)
		}
	}()

	verifyErr := spoolAndVerifyImage(r.Context(), body, spool, image.Size, image.SHA256)
	stopBodyWatch()
	if verifyErr != nil {
		switch {
		case errors.Is(verifyErr, media.ErrMediaIntegrity):
			slog.Error("attached image stream failed", "image_id", canonicalID)
		case errors.Is(verifyErr, media.ErrMediaStorageUnavailable):
			slog.Error("attached image stream unavailable", "image_id", canonicalID)
		}
		writeMediaImageError(w, verifyErr)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(handler.imageWriteTimeout()))
	digest := hex.EncodeToString(image.SHA256[:])
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(image.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+digest+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)

	if _, err := io.CopyN(w, spool.file, image.Size); err != nil {
		slog.Error("attached image response failed", "image_id", canonicalID)
	}
}

func newImageSpool(dir string) (*imageSpool, error) {
	file, err := os.CreateTemp(dir, imageReadScratchPrefix)
	if err != nil {
		return nil, errors.Join(media.ErrMediaStorageUnavailable, fmt.Errorf("create image scratch file: %w", err))
	}
	return &imageSpool{file: file}, nil
}

func acquireImageReadSlot(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case imageReadSlots <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-imageReadSlots
			})
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (handler server) imageWriteTimeout() time.Duration {
	if handler.imageResponseWriteTimeout > 0 {
		return handler.imageResponseWriteTimeout
	}
	return imageResponseWriteTimeout
}

func spoolAndVerifyImage(ctx context.Context, body io.Reader, spool *imageSpool, expectedSize int64, expectedDigest [32]byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	writer := &imageSpoolWriter{file: spool.file, hasher: sha256.New()}
	copied, copyErr := io.Copy(writer, io.LimitReader(body, expectedSize+1))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if writer.err != nil {
		return errors.Join(media.ErrMediaStorageUnavailable, fmt.Errorf("write image scratch file: %w", writer.err))
	}
	if copyErr != nil {
		return errors.Join(media.ErrMediaStorageUnavailable, copyErr)
	}
	if copied != expectedSize || copied > media.MaxEncodedImageSize || !bytes.Equal(writer.hasher.Sum(nil), expectedDigest[:]) {
		return media.ErrMediaIntegrity
	}
	if _, err := spool.file.Seek(0, io.SeekStart); err != nil {
		return errors.Join(media.ErrMediaStorageUnavailable, fmt.Errorf("rewind image scratch file: %w", err))
	}
	return nil
}

func (writer *imageSpoolWriter) Write(buffer []byte) (int, error) {
	count, err := writer.file.Write(buffer)
	if count > 0 {
		_, _ = writer.hasher.Write(buffer[:count])
	}
	if err == nil && count != len(buffer) {
		err = io.ErrShortWrite
	}
	if err != nil {
		writer.err = err
	}
	return count, err
}

func watchImageBody(ctx context.Context, body io.ReadCloser) (*imageBody, func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	wrapped := &imageBody{body: body}
	stop := make(chan struct{})
	done := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			_ = wrapped.Close()
		case <-stop:
		}
	}()
	stopWatch := func() {
		stopOnce.Do(func() {
			close(stop)
			<-done
			_ = wrapped.Close()
		})
	}
	return wrapped, stopWatch
}

func (body *imageBody) Read(buffer []byte) (int, error) {
	return body.body.Read(buffer)
}

func (body *imageBody) Close() error {
	body.once.Do(func() {
		body.closeErr = body.body.Close()
	})
	return body.closeErr
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

func (spool *imageSpool) CloseRemove() error {
	if spool == nil || spool.file == nil {
		return nil
	}
	spool.once.Do(func() {
		closeErr := spool.file.Close()
		removeErr := os.Remove(spool.file.Name())
		spool.cleanupErr = errors.Join(closeErr, removeErr)
	})
	return spool.cleanupErr
}
