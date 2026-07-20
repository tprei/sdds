package httpapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/tprei/sdds/services/api/internal/media"
)

func TestPrepareImageUploadOverridesServerReadTimeout(t *testing.T) {
	body, contentType := multipartBody(t, []byte("x"), true)
	server := &http.Server{
		Handler: authenticatedUploadRouter(fakeUploadPreparer{prepare: func(ctx context.Context, _ string, receive media.UploadReceiver) (media.UploadReceipt, error) {
			_, err := receive(ctx, io.Discard)
			return media.UploadReceipt{ImageUploadID: "4d8c7a48-443e-4f52-a3e7-45cbcf5d6a19", ContentType: "image/jpeg", ByteSize: 1, Width: 1, Height: 1, ExpiresAt: time.Now().Add(time.Hour)}, err
		}}),
		ReadTimeout: 1 * time.Second,
	}
	address, stop := serveTestHTTPServer(t, server)
	defer stop()

	response, err := sendThrottledRequest(address, http.MethodPost, "/v1/media/image-uploads", contentType, body.Bytes(), 1200*time.Millisecond)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, http.StatusCreated, responseBody)
	}
}

func TestNonUploadRequestRetainsServerReadTimeout(t *testing.T) {
	readErrors := make(chan error, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, readErr := io.Copy(io.Discard, r.Body)
			readErrors <- readErr
			if readErr != nil {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}),
		ReadTimeout: 1 * time.Second,
	}
	address, stop := serveTestHTTPServer(t, server)
	defer stop()

	response, _ := sendThrottledRequest(address, http.MethodPost, "/", "application/octet-stream", []byte("body"), 1200*time.Millisecond)
	if response != nil {
		_ = response.Body.Close()
	}
	select {
	case readErr := <-readErrors:
		if readErr == nil {
			t.Fatal("request body read succeeded, want server read timeout")
		}
		var timeoutErr net.Error
		if !errors.As(readErr, &timeoutErr) || !timeoutErr.Timeout() {
			t.Fatalf("request body read error = %v, want timeout", readErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request body read error")
	}
}

func serveTestHTTPServer(t *testing.T, server *http.Server) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	return listener.Addr().String(), func() {
		shutdownErr := server.Shutdown(context.Background())
		serveErr := <-serveDone
		if shutdownErr != nil {
			t.Errorf("shutdown server: %v", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Errorf("serve server: %v", serveErr)
		}
	}
}

func sendThrottledRequest(address, method, path, contentType string, body []byte, delay time.Duration) (*http.Response, error) {
	connection, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	request := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", method, path, address, contentType, len(body))
	if _, err := io.WriteString(connection, request); err != nil {
		_ = connection.Close()
		return nil, err
	}
	time.Sleep(delay)
	if _, err := connection.Write(body); err != nil {
		_ = connection.Close()
		return nil, err
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	return response, nil
}
