package mail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testConfig(t *testing.T, url string) Config {
	t.Helper()
	return Config{
		apiURL:      url,
		apiToken:    "test-token",
		fromAddress: "sdds <nao-responda@sdds.test>",
		timeout:     time.Second,
		loaded:      true,
	}
}

func TestClientSendHappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/" {
			t.Fatalf("path = %q, want /", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q, want application/json", got)
		}
		var decoded sendRequest
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if decoded.From != "sdds <nao-responda@sdds.test>" {
			t.Fatalf("from = %q, want sdds <nao-responda@sdds.test>", decoded.From)
		}
		if len(decoded.To) != 1 || decoded.To[0] != "to@example.com" {
			t.Fatalf("to = %+v, want [to@example.com]", decoded.To)
		}
		if decoded.Subject != "assunto" {
			t.Fatalf("subject = %q, want assunto", decoded.Subject)
		}
		if decoded.Text != "corpo" {
			t.Fatalf("text = %q, want corpo", decoded.Text)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.Send(context.Background(), Message{
		To:      "to@example.com",
		Subject: "assunto",
		Text:    "corpo",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestClientSendRejectsNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(testConfig(t, server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.Send(context.Background(), Message{
		To:      "to@example.com",
		Subject: "assunto",
		Text:    "corpo",
	}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("send error = %v, want ErrUnavailable", err)
	}
}

func TestClientSendRespectsContextTimeout(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer server.Close()
	defer close(blocked)

	config := testConfig(t, server.URL)
	config.timeout = 10 * time.Millisecond
	client, err := New(config)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.Send(context.Background(), Message{
		To:      "to@example.com",
		Subject: "assunto",
		Text:    "corpo",
	}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("send error = %v, want ErrUnavailable", err)
	}
}

func TestNewRejectsUnloadedConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for unloaded config, got nil")
	}
}
