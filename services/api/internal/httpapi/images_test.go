package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

const testImageID = "4d8c7a48-443e-4f52-a3e7-45cbcf5d6a19"

func TestGetMediaImageStreamsExactBytesAndHeaders(t *testing.T) {
	body := []byte("jpeg bytes")
	digest := sha256.Sum256(body)
	router := publicImageRouter(fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: io.NopCloser(bytes.NewReader(body)), ContentType: "image/jpeg", Size: int64(len(body)), SHA256: digest}, nil
	}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/media/images/"+testImageID, nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !bytes.Equal(response.Body.Bytes(), body) {
		t.Fatalf("body = %q, want %q", response.Body.Bytes(), body)
	}
	if got := response.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", got)
	}
	if got := response.Header().Get("Content-Length"); got != "10" {
		t.Fatalf("Content-Length = %q, want 10", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("ETag"); got != `"`+hexDigest(digest)+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := response.Header().Get("Content-Disposition"); got != "inline" {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestGetMediaImageMapsReaderErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       openapi.ErrorCode
		retryAfter string
	}{
		{name: "not found", err: media.ErrImageNotFound, status: http.StatusNotFound, code: openapi.ErrorCodeNotFound},
		{name: "integrity", err: media.ErrMediaIntegrity, status: http.StatusInternalServerError, code: openapi.ErrorCodeMediaIntegrityError},
		{name: "unavailable", err: media.ErrMediaStorageUnavailable, status: http.StatusServiceUnavailable, code: openapi.ErrorCodeMediaStorageUnavailable, retryAfter: "5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := publicImageRouter(fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
				return media.AttachedImage{}, test.err
			}})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/media/images/"+testImageID, nil)

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if got := response.Header().Get("Retry-After"); got != test.retryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, test.retryAfter)
			}
			assertImageErrorCode(t, response, test.code)
		})
	}
}

func TestGetMediaImageRejectsInvalidUUID(t *testing.T) {
	called := false
	router := publicImageRouter(fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		called = true
		return media.AttachedImage{}, nil
	}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/media/images/not-a-uuid", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertImageErrorCode(t, response, openapi.ErrorCodeInvalidMedia)
	if called {
		t.Fatal("image reader called for invalid UUID")
	}
}

func TestGetMediaImageStopsAtStoredSizeAndLogsNoStorageKey(t *testing.T) {
	body := []byte("too-long")
	digest := sha256.Sum256([]byte("exact"))
	router := publicImageRouter(fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: io.NopCloser(bytes.NewReader(body)), ContentType: "image/png", Size: int64(len("exact")), SHA256: digest}, nil
	}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/media/images/"+testImageID, nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got, want := response.Body.String(), "too-l"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func publicImageRouter(reader media.AttachedImageReader) http.Handler {
	return NewRouter(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{}, reader)
}

type fakeAttachedImageReader struct {
	open func(context.Context, string) (media.AttachedImage, error)
}

func (reader fakeAttachedImageReader) OpenAttached(ctx context.Context, imageID string) (media.AttachedImage, error) {
	if reader.open == nil {
		return media.AttachedImage{}, errors.New("image reader not implemented")
	}
	return reader.open(ctx, imageID)
}

func assertImageErrorCode(t *testing.T, response *httptest.ResponseRecorder, want openapi.ErrorCode) {
	t.Helper()
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != want {
		t.Fatalf("error code = %q, want %q", body.Code, want)
	}
}

func hexDigest(digest [32]byte) string {
	return hex.EncodeToString(digest[:])
}
func TestGetMediaImageLogsShortEOFAndClosesBody(t *testing.T) {
	logs := captureImageStreamLogs(t)
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("short"))}
	digest := sha256.Sum256([]byte("expected"))
	response := httptest.NewRecorder()
	serveImageDirect(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: int64(len("expected")), SHA256: digest}, nil
	}})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !body.closed {
		t.Fatal("short body was not closed")
	}
	if !strings.Contains(logs.String(), "attached image stream failed") {
		t.Fatalf("logs = %q, want sanitized stream failure", logs.String())
	}
}

func TestGetMediaImageSanitizesMidstreamReaderError(t *testing.T) {
	const secret = "s3://provider.example/private-bucket?access_key=secret"
	logs := captureImageStreamLogs(t)
	body := &trackingReadCloser{Reader: &streamErrorReader{data: []byte("part"), err: errors.New(secret)}}
	response := httptest.NewRecorder()
	serveImageDirect(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/png", Size: 5, SHA256: sha256.Sum256([]byte("part!"))}, nil
	}})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body.closed != true {
		t.Fatal("midstream-error body was not closed")
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("logs leaked provider error: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "attached image stream failed") {
		t.Fatalf("logs = %q, want sanitized stream failure", logs.String())
	}
}

func TestGetMediaImageWriterErrorKeepsCommittedStatus(t *testing.T) {
	const secret = "provider write failed at s3://private.example/key"
	logs := captureImageStreamLogs(t)
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("image"))}
	writer := &failingResponseWriter{err: errors.New(secret)}
	serveImageDirect(writer, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: 5, SHA256: sha256.Sum256([]byte("image"))}, nil
	}})

	if writer.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusOK)
	}
	if writer.writeHeaders != 1 {
		t.Fatalf("WriteHeader calls = %d, want 1", writer.writeHeaders)
	}
	if writer.writes != 1 {
		t.Fatalf("Write calls = %d, want 1", writer.writes)
	}
	if !body.closed {
		t.Fatal("writer-error body was not closed")
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("logs leaked provider error: %q", logs.String())
	}
}

func serveImageDirect(writer http.ResponseWriter, reader media.AttachedImageReader) {
	handler := server{imageReader: reader}
	request := httptest.NewRequest(http.MethodGet, "/v1/media/images/"+testImageID, nil)
	handler.GetMediaImage(writer, request, openapi_types.UUID(uuid.MustParse(testImageID)))
}

func captureImageStreamLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := slog.Default()
	logs := new(bytes.Buffer)
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return logs
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (reader *trackingReadCloser) Close() error {
	reader.closed = true
	return nil
}

type streamErrorReader struct {
	data []byte
	err  error
	sent bool
}

func (reader *streamErrorReader) Read(p []byte) (int, error) {
	if reader.sent {
		return 0, reader.err
	}
	reader.sent = true
	return copy(p, reader.data), nil
}

type failingResponseWriter struct {
	header       http.Header
	status       int
	writeHeaders int
	writes       int
	err          error
}

func (writer *failingResponseWriter) Header() http.Header {
	if writer.header == nil {
		writer.header = make(http.Header)
	}
	return writer.header
}

func (writer *failingResponseWriter) WriteHeader(status int) {
	writer.writeHeaders++
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *failingResponseWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, writer.err
}
