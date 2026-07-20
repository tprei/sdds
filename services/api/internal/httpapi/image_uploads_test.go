package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/media"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/user"
)

const testUploadRequestID = "c6bf4f8d-3a5a-4c98-bf3f-5ddf8ecb87f6"

func TestPrepareImageUploadAuthenticatesBeforeReadingBody(t *testing.T) {
	body := &countingReader{Reader: strings.NewReader("not-read")}
	router := newRouterForTest(fakeNoteStore{},
		fakeCatalog{},
		fakeUserStore{findCurrentSession: func(context.Context, string, time.Time) (user.CurrentSession, error) {
			return user.CurrentSession{}, user.ErrSessionNotFound
		}},
		DefaultAuthLimits(),
		fakeReadiness{},
		fakeUploadPreparer{}, fakeAttachedImageReader{})
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
}

func TestPrepareImageUploadExcludesGenericBodyValidation(t *testing.T) {
	fileBytes := []byte("image bytes are validated by the application service")
	var reader *countingReader
	var source *bytes.Reader
	var got []byte
	service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, userID string, receive media.UploadReceiver) (media.UploadReceipt, error) {
		if userID != "upload-user" {
			t.Fatalf("user ID = %q, want upload-user", userID)
		}
		if reader.reads != 0 {
			t.Fatalf("body reads before receive = %d, want 0", reader.reads)
		}
		var buffer bytes.Buffer
		requestID, err := receive(ctx, &buffer)
		if err != nil {
			t.Fatalf("receive upload: %v", err)
		}
		if requestID != testUploadRequestID {
			t.Fatalf("request ID = %q, want %q", requestID, testUploadRequestID)
		}
		if source.Len() != 0 {
			t.Fatalf("parser left %d body bytes unread", source.Len())
		}
		got = buffer.Bytes()
		return media.UploadReceipt{
			ImageUploadID: "4d8c7a48-443e-4f52-a3e7-45cbcf5d6a19",
			ContentType:   "image/jpeg",
			ByteSize:      int64(len(fileBytes)),
			Width:         1200,
			Height:        900,
			ExpiresAt:     time.UnixMilli(1780000000000).UTC(),
		}, nil
	}}
	router := authenticatedUploadRouter(service)
	body, contentType := multipartBody(t, fileBytes, false)
	source = bytes.NewReader(body.Bytes())
	reader = &countingReader{Reader: source}
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", reader)
	if request.Body != reader {
		t.Fatal("handler did not receive the tracking multipart stream")
	}
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if diff := cmp.Diff(fileBytes, got); diff != "" {
		t.Fatalf("service source mismatch (-want +got):\n%s", diff)
	}
	var receipt openapi.ImageUploadReceipt
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.ImageUploadId.String() != "4d8c7a48-443e-4f52-a3e7-45cbcf5d6a19" {
		t.Fatalf("image upload ID = %q", receipt.ImageUploadId)
	}
}

func TestPrepareImageUploadMapsRetryableErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		receive    func(context.Context, media.UploadReceiver) error
		status     int
		code       openapi.ErrorCode
		retryAfter string
		field      openapi.ValidationField
	}{
		{name: "in progress", err: media.ErrUploadInProgress, status: http.StatusConflict, code: openapi.ErrorCodeUploadInProgress, retryAfter: "120"},
		{name: "quota", err: media.ErrUploadQuotaExceeded, status: http.StatusTooManyRequests, code: openapi.ErrorCodeMediaStagingQuotaExceeded, retryAfter: "60"},
		{name: "storage", err: media.ErrMediaStorageUnavailable, status: http.StatusServiceUnavailable, code: openapi.ErrorCodeMediaStorageUnavailable, retryAfter: "5"},
		{name: "invalid request", err: media.ErrInvalidUploadRequest, status: http.StatusBadRequest, code: openapi.ErrorCodeInvalidMedia, field: openapi.ValidationFieldUploadRequestID},
		{name: "invalid media", err: media.ErrInvalidMedia, status: http.StatusBadRequest, code: openapi.ErrorCodeInvalidMedia, field: openapi.ValidationFieldFile},
		{name: "invalid dimensions", err: media.ErrMediaDimensions, status: http.StatusBadRequest, code: openapi.ErrorCodeInvalidMedia, field: openapi.ValidationFieldFile},
		{name: "unsupported media", err: media.ErrUnsupportedMediaType, status: http.StatusUnsupportedMediaType, code: openapi.ErrorCodeUnsupportedMediaType, field: openapi.ValidationFieldFile},
		{name: "service media too large", err: media.ErrMediaTooLarge, status: http.StatusRequestEntityTooLarge, code: openapi.ErrorCodeRequestTooLarge, field: openapi.ValidationFieldFile},
		{name: "parser media too large", receive: func(ctx context.Context, receive media.UploadReceiver) error {
			_, err := receive(ctx, mediaTooLargeWriter{})
			return err
		}, status: http.StatusRequestEntityTooLarge, code: openapi.ErrorCodeRequestTooLarge},
		{name: "direct nil destination", receive: func(ctx context.Context, receive media.UploadReceiver) error { _, err := receive(ctx, nil); return err }, status: http.StatusBadRequest, code: openapi.ErrorCodeInvalidMedia},
		{name: "idempotency conflict", err: media.ErrUploadIdempotencyConflict, status: http.StatusConflict, code: openapi.ErrorCodeIdempotencyConflict},
		{name: "expired", err: media.ErrUploadExpired, status: http.StatusConflict, code: openapi.ErrorCodeUploadExpired},
		{name: "integrity", err: media.ErrMediaIntegrity, status: http.StatusConflict, code: openapi.ErrorCodeMediaIntegrityError},
		{name: "internal", err: errors.New("unexpected"), status: http.StatusInternalServerError, code: openapi.ErrorCodeInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
				if test.receive != nil {
					return media.UploadReceipt{}, test.receive(ctx, receive)
				}
				_, receiveErr := receive(ctx, io.Discard)
				if receiveErr != nil {
					return media.UploadReceipt{}, receiveErr
				}
				return media.UploadReceipt{}, test.err
			}}
			router := authenticatedUploadRouter(service)
			body, contentType := multipartBody(t, []byte("image bytes"), false)
			request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if response.Header().Get("Retry-After") != test.retryAfter {
				t.Fatalf("Retry-After = %q, want %q", response.Header().Get("Retry-After"), test.retryAfter)
			}
			assertErrorCode(t, response, test.code, test.field)
			requireOpenAPIResponse(t, request, response)
		})
	}
}

func authenticatedUploadRouter(service ImageUploadPreparer) http.Handler {
	handler := newRouterForTest(fakeNoteStore{},
		fakeCatalog{},
		fakeUserStore{findCurrentSession: func(_ context.Context, tokenHash string, _ time.Time) (user.CurrentSession, error) {
			if tokenHash != user.HashSessionToken("current-token") {
				return user.CurrentSession{}, user.ErrSessionNotFound
			}
			return user.CurrentSession{User: user.User{ID: "upload-user", State: user.UserStateActive}}, nil
		}},
		DefaultAuthLimits(),
		fakeReadiness{},
		service, fakeAttachedImageReader{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer current-token")
		handler.ServeHTTP(w, r)
	})
}

type countingReader struct {
	io.Reader
	reads int
}

func (reader *countingReader) Read(p []byte) (int, error) {
	reader.reads++
	return reader.Reader.Read(p)
}

func (reader *countingReader) Close() error { return nil }

type mediaTooLargeWriter struct{}

func (mediaTooLargeWriter) Write([]byte) (int, error) { return 0, media.ErrMediaTooLarge }

func multipartBody(t *testing.T, fileBytes []byte, requestIDFirst bool) (*bytes.Buffer, string) {
	t.Helper()
	return multipartBodyWith(t, func(writer *multipart.Writer) error {
		if requestIDFirst {
			if err := writer.WriteField("upload_request_id", testUploadRequestID); err != nil {
				return err
			}
		}
		if err := writeMultipartPart(writer, "file", "photo.jpg", true, fileBytes); err != nil {
			return err
		}
		if !requestIDFirst {
			if err := writer.WriteField("upload_request_id", testUploadRequestID); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeMultipartPart(writer *multipart.Writer, name, filename string, withFilename bool, value []byte) error {
	var (
		part io.Writer
		err  error
	)
	if withFilename {
		part, err = writer.CreateFormFile(name, filename)
	} else {
		part, err = writer.CreateFormField(name)
	}
	if err != nil {
		return err
	}
	_, err = part.Write(value)
	return err
}
func writeUploadParts(writer *multipart.Writer, requestID, filename string, withFilename bool, fileBytes []byte) error {
	if err := writer.WriteField("upload_request_id", requestID); err != nil {
		return err
	}
	return writeMultipartPart(writer, "file", filename, withFilename, fileBytes)
}

func multipartBodyWith(t *testing.T, write func(*multipart.Writer) error) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := write(writer); err != nil {
		t.Fatalf("write multipart body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart body: %v", err)
	}
	return body, writer.FormDataContentType()
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want openapi.ErrorCode, wantField ...openapi.ValidationField) {
	t.Helper()
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != want {
		t.Fatalf("error code = %q, want %q", body.Code, want)
	}
	if len(wantField) > 0 && wantField[0] != "" {
		requireValidationProblems(t, body.Fields, []openapi.ValidationProblem{{Field: wantField[0], Code: openapi.ValidationProblemCodeInvalid}})
		return
	}
	if body.Fields != nil {
		t.Fatalf("validation fields = %#v, want nil", *body.Fields)
	}
}
func TestPrepareImageUploadRejectsInvalidMultipart(t *testing.T) {
	validBody := func(writer *multipart.Writer) error {
		return writeUploadParts(writer, testUploadRequestID, "photo.jpg", true, []byte("image"))
	}
	tests := []struct {
		name        string
		write       func(*multipart.Writer) error
		contentType string
		trim        int
		noFileBytes bool
		field       openapi.ValidationField
	}{
		{name: "unknown", write: func(writer *multipart.Writer) error {
			return writer.WriteField("extra", "value")
		}},
		{name: "wrong Content-Type", write: validBody, contentType: "application/json"},
		{name: "missing boundary", write: validBody, contentType: "multipart/form-data"},
		{name: "empty boundary", write: validBody, contentType: `multipart/form-data; boundary=""`},
		{name: "duplicate request ID", write: func(writer *multipart.Writer) error {
			if err := writer.WriteField("upload_request_id", testUploadRequestID); err != nil {
				return err
			}
			return writer.WriteField("upload_request_id", testUploadRequestID)
		}},
		{name: "duplicate file", write: func(writer *multipart.Writer) error {
			for range 2 {
				if err := writeMultipartPart(writer, "file", "photo.jpg", true, []byte("image")); err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "missing request ID", write: func(writer *multipart.Writer) error {
			return writeMultipartPart(writer, "file", "photo.jpg", true, []byte("image"))
		}},
		{name: "missing file", write: func(writer *multipart.Writer) error {
			return writer.WriteField("upload_request_id", testUploadRequestID)
		}},
		{name: "file without filename", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, testUploadRequestID, "", false, []byte("image"))
		}, noFileBytes: true},
		{name: "file with empty filename", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, testUploadRequestID, "", true, []byte("image"))
		}, noFileBytes: true},
		{name: "request ID with empty filename", write: func(writer *multipart.Writer) error {
			return writeMultipartPart(writer, "upload_request_id", "", true, []byte(testUploadRequestID))
		}, noFileBytes: true},
		{name: "request ID with filename", write: func(writer *multipart.Writer) error {
			return writeMultipartPart(writer, "upload_request_id", "id.txt", true, []byte(testUploadRequestID))
		}, noFileBytes: true},
		{name: "wrong disposition", write: func(writer *multipart.Writer) error {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", `attachment; name="file"; filename="photo.jpg"`)
			part, err := writer.CreatePart(header)
			if err != nil {
				return err
			}
			_, err = part.Write([]byte("image"))
			return err
		}},
		{name: "invalid request ID", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, "not-a-uuid", "photo.jpg", true, []byte("image"))
		}, noFileBytes: true, field: openapi.ValidationFieldUploadRequestID},
		{name: "raw32 request ID", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, strings.ReplaceAll(testUploadRequestID, "-", ""), "photo.jpg", true, []byte("image"))
		}, noFileBytes: true, field: openapi.ValidationFieldUploadRequestID},
		{name: "braced request ID", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, "{"+testUploadRequestID+"}", "photo.jpg", true, []byte("image"))
		}, noFileBytes: true, field: openapi.ValidationFieldUploadRequestID},
		{name: "uppercase request ID", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, strings.ToUpper(testUploadRequestID), "photo.jpg", true, []byte("image"))
		}, noFileBytes: true, field: openapi.ValidationFieldUploadRequestID},
		{name: "request ID over 128 bytes", write: func(writer *multipart.Writer) error {
			return writeUploadParts(writer, strings.Repeat("a", 129), "photo.jpg", true, []byte("image"))
		}, noFileBytes: true, field: openapi.ValidationFieldUploadRequestID},
		{name: "truncated", write: validBody, trim: 20},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			published := false
			var received bytes.Buffer
			service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
				_, err := receive(ctx, &received)
				if err != nil {
					return media.UploadReceipt{}, err
				}
				published = true
				return uploadTestReceipt(), nil
			}}
			router := authenticatedUploadRouter(service)
			body, contentType := multipartBodyWith(t, test.write)
			contentType = [...]string{contentType, test.contentType}[min(1, len(test.contentType))]
			if test.trim > 0 {
				bodyBytes := body.Bytes()
				body = bytes.NewBuffer(bodyBytes[:len(bodyBytes)-test.trim])
			}
			request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			assertErrorCode(t, response, openapi.ErrorCodeInvalidMedia, test.field)
			requireOpenAPIResponse(t, request, response)
			if published {
				t.Fatal("upload was published for invalid multipart")
			}
			if (test.noFileBytes || test.contentType != "") && received.Len() != 0 {
				t.Fatalf("received file bytes = %d, want 0", received.Len())
			}
		})
	}
}

func TestPrepareImageUploadRejectsTrailingPartBeforePublication(t *testing.T) {
	body, contentType := multipartBodyWith(t, func(writer *multipart.Writer) error {
		if err := writer.WriteField("upload_request_id", testUploadRequestID); err != nil {
			return err
		}
		part, err := writer.CreateFormFile("file", "photo.jpg")
		if err != nil {
			return err
		}
		if _, err := part.Write([]byte("image")); err != nil {
			return err
		}
		return writer.WriteField("trailing", "invalid")
	})
	published := false
	service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
		_, err := receive(ctx, io.Discard)
		if err != nil {
			return media.UploadReceipt{}, err
		}
		published = true
		return uploadTestReceipt(), nil
	}}
	router := authenticatedUploadRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, response, openapi.ErrorCodeInvalidMedia)
	requireOpenAPIResponse(t, request, response)
	if published {
		t.Fatal("upload was published before trailing part validation")
	}
}

func TestPrepareImageUploadRejectsOuterBodyLimit(t *testing.T) {
	body, contentType := multipartBody(t, bytes.Repeat([]byte("x"), int(media.MaxMultipartBodySize)), false)
	service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
		_, err := receive(ctx, io.Discard)
		return media.UploadReceipt{}, err
	}}
	router := authenticatedUploadRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	assertErrorCode(t, response, openapi.ErrorCodeRequestTooLarge)
	requireOpenAPIResponse(t, request, response)
}

func TestPrepareImageUploadResponseMatchesOpenAPIShape(t *testing.T) {
	service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
		if _, err := receive(ctx, io.Discard); err != nil {
			return media.UploadReceipt{}, err
		}
		return uploadTestReceipt(), nil
	}}
	router := authenticatedUploadRouter(service)
	body, contentType := multipartBody(t, []byte("image"), false)
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	requireOpenAPIResponse(t, request, response)
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}
	wire := decodeResponseObject(t, response.Body.Bytes())
	if len(wire) != 6 {
		t.Fatalf("receipt fields = %d, want 6", len(wire))
	}
	for _, field := range []string{"image_upload_id", "content_type", "byte_size", "width", "height", "expires_at"} {
		if _, ok := wire[field]; !ok {
			t.Fatalf("receipt missing field %q: %#v", field, wire)
		}
	}
}

func TestPrepareImageUploadClosesBodyOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &blockingUploadBody{closed: make(chan struct{})}
	started := make(chan struct{})
	service := fakeUploadPreparer{prepareImageUpload: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
		close(started)
		_, err := receive(ctx, io.Discard)
		return media.UploadReceipt{}, err
	}}
	handler := server{media: mediaHandlers{imageUploads: service}}
	sessionContext := context.WithValue(ctx, currentSessionContextKey{}, user.CurrentSession{User: user.User{ID: "upload-user", State: user.UserStateActive}})
	request := httptest.NewRequest(http.MethodPost, "/v1/media/image-uploads", nil).WithContext(sessionContext)
	request.Body = body
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.PrepareImageUpload(response, request)
		close(done)
	}()

	<-started
	cancel()
	select {
	case <-body.closed:
	case <-time.After(time.Second):
		t.Fatal("request body was not closed after cancellation")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("upload handler did not return after cancellation")
	}
}

func uploadTestReceipt() media.UploadReceipt {
	return media.UploadReceipt{
		ImageUploadID: "4d8c7a48-443e-4f52-a3e7-45cbcf5d6a19",
		ContentType:   "image/jpeg",
		ByteSize:      5,
		Width:         10,
		Height:        8,
		ExpiresAt:     time.UnixMilli(1780000000000).UTC(),
	}
}

type blockingUploadBody struct {
	closed chan struct{}
	once   sync.Once
}

func (body *blockingUploadBody) Read([]byte) (int, error) {
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *blockingUploadBody) Close() error {
	body.once.Do(func() {
		close(body.closed)
	})
	return nil
}
