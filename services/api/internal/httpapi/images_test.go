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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestGetMediaImageRejectsInvalidAttachedResponseBeforeHeaders(t *testing.T) {
	tests := []struct {
		name    string
		image   media.AttachedImage
		hasBody bool
	}{
		{name: "nil body", image: media.AttachedImage{ContentType: "image/jpeg", Size: 1}},
		{name: "zero size", image: media.AttachedImage{ContentType: "image/jpeg"}, hasBody: true},
		{name: "size over maximum", image: media.AttachedImage{ContentType: "image/jpeg", Size: media.MaxEncodedImageSize + 1}, hasBody: true},
		{name: "unsupported content type", image: media.AttachedImage{ContentType: "image/gif", Size: 1}, hasBody: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			body := &trackingReadCloser{Reader: bytes.NewReader([]byte("image"))}
			image := test.image
			if test.hasBody {
				image.Body = body
			}
			response := httptest.NewRecorder()
			serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
				return image, nil
			}}, dir)

			assertImageIntegrityFailure(t, response)
			if test.hasBody && !body.isClosed() {
				t.Fatal("invalid attached-image body was not closed")
			}
			assertScratchEmpty(t, dir)
			for _, header := range []string{"Content-Length", "Cache-Control", "ETag", "X-Content-Type-Options", "Content-Disposition"} {
				if got := response.Header().Get(header); got != "" {
					t.Fatalf("%s = %q, want no success header", header, got)
				}
			}
		})
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

func TestGetMediaImageRejectsSizeMismatchBeforeCommit(t *testing.T) {
	logs := captureImageStreamLogs(t)
	dir := t.TempDir()
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("short"))}
	response := httptest.NewRecorder()
	serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: int64(len("expected")), SHA256: sha256.Sum256([]byte("expected"))}, nil
	}}, dir)

	assertImageIntegrityFailure(t, response)
	if body.isClosed() == false {
		t.Fatal("size-mismatch body was not closed")
	}
	assertScratchEmpty(t, dir)
	if !strings.Contains(logs.String(), "attached image stream failed") {
		t.Fatalf("logs = %q, want sanitized stream failure", logs.String())
	}
}

func TestGetMediaImageRejectsDigestMismatchBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	bodyBytes := []byte("wrong bytes")
	body := &trackingReadCloser{Reader: bytes.NewReader(bodyBytes)}
	response := httptest.NewRecorder()
	serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/png", Size: int64(len(bodyBytes)), SHA256: sha256.Sum256([]byte("right bytes"))}, nil
	}}, dir)

	assertImageIntegrityFailure(t, response)
	if !body.isClosed() {
		t.Fatal("digest-mismatch body was not closed")
	}
	assertScratchEmpty(t, dir)
}

func TestGetMediaImageRejectsSmallDeclaredExtraBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("too-long"))}
	response := httptest.NewRecorder()
	serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/png", Size: int64(len("exact")), SHA256: sha256.Sum256([]byte("exact"))}, nil
	}}, dir)

	assertImageIntegrityFailure(t, response)
	if !body.isClosed() {
		t.Fatal("extra-bytes body was not closed")
	}
	assertScratchEmpty(t, dir)
}
func TestGetMediaImageAcceptsMaximumDeclaredSize(t *testing.T) {
	dir := t.TempDir()
	bodyBytes := bytes.Repeat([]byte{'x'}, int(media.MaxEncodedImageSize))
	body := &trackingReadCloser{Reader: bytes.NewReader(bodyBytes)}
	response := &countingResponseWriter{}
	serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: media.MaxEncodedImageSize, SHA256: sha256.Sum256(bodyBytes)}, nil
	}}, dir)

	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	if response.bytes != media.MaxEncodedImageSize {
		t.Fatalf("response bytes = %d, want %d", response.bytes, media.MaxEncodedImageSize)
	}
	if got := response.Header().Get("Content-Length"); got != "10485760" {
		t.Fatalf("Content-Length = %q, want 10485760", got)
	}
	if !body.isClosed() {
		t.Fatal("maximum-size body was not closed")
	}
	assertScratchEmpty(t, dir)
}

func TestGetMediaImageRejectsLateReadFailureBeforeCommit(t *testing.T) {
	const secret = "s3://provider.example/private-bucket?access_key=secret"
	logs := captureImageStreamLogs(t)
	dir := t.TempDir()
	body := &trackingReadCloser{Reader: &streamErrorReader{data: []byte("image"), err: errors.New(secret)}}
	response := httptest.NewRecorder()
	serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		return media.AttachedImage{Body: body, ContentType: "image/png", Size: int64(len("image")), SHA256: sha256.Sum256([]byte("image"))}, nil
	}}, dir)

	assertImageStorageFailure(t, response)
	if !body.isClosed() {
		t.Fatal("late-error body was not closed")
	}
	assertScratchEmpty(t, dir)
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("logs leaked provider error: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "attached image stream unavailable") {
		t.Fatalf("logs = %q, want sanitized stream-unavailable failure", logs.String())
	}
}

func TestGetMediaImageCleansScratchOnCancellation(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("image"))}
	response := httptest.NewRecorder()
	serveImageDirectInDirWithContext(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
		cancel()
		return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: 5, SHA256: sha256.Sum256([]byte("image"))}, nil
	}}, dir, ctx)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	assertImageErrorCode(t, response, openapi.ErrorCodeMediaStorageUnavailable)
	if !body.isClosed() {
		t.Fatal("canceled body was not closed")
	}
	assertScratchEmpty(t, dir)
}
func TestGetMediaImageWaitsForReadSlotWithCancellation(t *testing.T) {
	imageReadSlots <- struct{}{}
	imageReadSlots <- struct{}{}
	defer func() {
		<-imageReadSlots
		<-imageReadSlots
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := false
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		serveImageDirectInDirWithContext(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
			called = true
			return media.AttachedImage{}, nil
		}}, t.TempDir(), ctx)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("handler returned before read-slot cancellation")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not stop waiting for read slot")
	}
	if called {
		t.Fatal("image reader called while read slot was unavailable")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	assertImageErrorCode(t, response, openapi.ErrorCodeMediaStorageUnavailable)
}

func TestGetMediaImageCleansScratchOnPanic(t *testing.T) {
	dir := t.TempDir()
	body := &trackingReadCloser{Reader: panicReader{}}
	response := httptest.NewRecorder()
	panicked := false
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		serveImageDirectInDir(response, fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
			return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: 1, SHA256: sha256.Sum256([]byte("x"))}, nil
		}}, dir)
	}()

	if !panicked {
		t.Fatal("image read panic was swallowed")
	}
	if !body.isClosed() {
		t.Fatal("panic body was not closed")
	}
	assertScratchEmpty(t, dir)
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
	if !body.isClosed() {
		t.Fatal("writer-error body was not closed")
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("logs leaked provider error: %q", logs.String())
	}
}
func TestGetMediaImageWriteDeadlineReleasesReadSlotAndScratch(t *testing.T) {
	const payloadSize = 64 * 1024

	dir := t.TempDir()
	payload := bytes.Repeat([]byte{'x'}, payloadSize)
	digest := sha256.Sum256(payload)
	body := newCloseSignalReader(bytes.NewReader(payload))
	serverConn, clientConn := net.Pipe()
	listener := &singleConnListener{conn: serverConn, closeCh: make(chan struct{})}
	handler := server{media: mediaHandlers{
		attachedImages: fakeAttachedImageReader{open: func(context.Context, string) (media.AttachedImage, error) {
			return media.AttachedImage{Body: body, ContentType: "image/jpeg", Size: int64(len(payload)), SHA256: digest}, nil
		}},
		scratchDir:           dir,
		responseWriteTimeout: 20 * time.Millisecond,
	}}
	handlerDone := make(chan struct{})
	httpServer := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler.GetMediaImage(writer, request, openapi_types.UUID(uuid.MustParse(testImageID)))
		close(handlerDone)
	})}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = clientConn.Close()
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			_ = httpServer.Close()
			t.Errorf("shutdown server: %v", err)
		}
		select {
		case err := <-serveDone:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("serve error: %v", err)
			}
		case <-time.After(time.Second):
			_ = httpServer.Close()
			t.Error("server did not stop")
		}
	})

	request := "GET /v1/media/images/" + testImageID + " HTTP/1.1\r\nHost: images.test\r\nConnection: close\r\n\r\n"
	if err := clientConn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set request write deadline: %v", err)
	}
	if _, err := io.WriteString(clientConn, request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after response write deadline")
	}
	select {
	case <-body.closed:
	default:
		t.Fatal("image body remained open after response write deadline")
	}
	assertScratchEmpty(t, dir)
	if got := len(imageReadSlots); got != 0 {
		t.Fatalf("read slots = %d, want 0", got)
	}
}

func publicImageRouter(reader media.AttachedImageReader) http.Handler {
	return newRouterForTest(fakeNoteStore{}, fakeCatalog{}, fakeUserStore{}, DefaultAuthLimits(), fakeReadiness{}, fakeUploadPreparer{}, reader)
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

func assertImageIntegrityFailure(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	assertImageErrorCode(t, response, openapi.ErrorCodeMediaIntegrityError)
	if got := response.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q, want no cache header", got)
	}
	if got := response.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag = %q, want no ETag", got)
	}
}
func assertImageStorageFailure(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	assertImageErrorCode(t, response, openapi.ErrorCodeMediaStorageUnavailable)
	if got := response.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, want 5", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q, want no cache header", got)
	}
	if got := response.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag = %q, want no ETag", got)
	}
}

func assertScratchEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read scratch dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("scratch entries = %v, want empty", entries)
	}
}

func hexDigest(digest [32]byte) string {
	return hex.EncodeToString(digest[:])
}

func serveImageDirect(writer http.ResponseWriter, reader media.AttachedImageReader) {
	serveImageDirectInDirWithContext(writer, reader, "", context.Background())
}

func serveImageDirectInDir(writer http.ResponseWriter, reader media.AttachedImageReader, dir string) {
	serveImageDirectInDirWithContext(writer, reader, dir, context.Background())
}

func serveImageDirectInDirWithContext(writer http.ResponseWriter, reader media.AttachedImageReader, dir string, ctx context.Context) {
	serveImageDirectInDirWithContextAndTimeout(writer, reader, dir, ctx, 0)
}

func serveImageDirectInDirWithContextAndTimeout(writer http.ResponseWriter, reader media.AttachedImageReader, dir string, ctx context.Context, timeout time.Duration) {
	handler := server{media: mediaHandlers{attachedImages: reader, scratchDir: dir, responseWriteTimeout: timeout}}
	request := httptest.NewRequest(http.MethodGet, "/v1/media/images/"+testImageID, nil).WithContext(ctx)
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
	mu     sync.Mutex
	closed bool
}

func (reader *trackingReadCloser) Close() error {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.closed = true
	return nil
}

func (reader *trackingReadCloser) isClosed() bool {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	return reader.closed
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

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("image reader failed")
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

type countingResponseWriter struct {
	header http.Header
	status int
	bytes  int64
}

func (writer *countingResponseWriter) Header() http.Header {
	if writer.header == nil {
		writer.header = make(http.Header)
	}
	return writer.header
}

func (writer *countingResponseWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *countingResponseWriter) Write(buffer []byte) (int, error) {
	writer.bytes += int64(len(buffer))
	return len(buffer), nil
}

type closeSignalReader struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func newCloseSignalReader(reader io.Reader) *closeSignalReader {
	return &closeSignalReader{Reader: reader, closed: make(chan struct{})}
}

func (reader *closeSignalReader) Close() error {
	reader.once.Do(func() {
		close(reader.closed)
	})
	return nil
}

type singleConnListener struct {
	conn     net.Conn
	closeCh  chan struct{}
	once     sync.Once
	accepted bool
}

func (listener *singleConnListener) Accept() (net.Conn, error) {
	if !listener.accepted {
		listener.accepted = true
		return listener.conn, nil
	}
	<-listener.closeCh
	return nil, net.ErrClosed
}

func (listener *singleConnListener) Close() error {
	listener.once.Do(func() {
		close(listener.closeCh)
		_ = listener.conn.Close()
	})
	return nil
}

func (listener *singleConnListener) Addr() net.Addr {
	return imageTestAddr("images")
}

type imageTestAddr string

func (imageTestAddr) Network() string {
	return "images"
}

func (addr imageTestAddr) String() string {
	return string(addr)
}
