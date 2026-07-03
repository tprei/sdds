package main

import (
	"net/http"
	"testing"
)

func TestNewServerSetsReadTimeouts(t *testing.T) {
	server := newServer(config{httpAddr: ":9090"}, http.NotFoundHandler())

	if server.Addr != ":9090" {
		t.Fatalf("addr = %q, want :9090", server.Addr)
	}
	if server.ReadHeaderTimeout != serverReadHeaderTimeout {
		t.Fatalf("read header timeout = %s, want %s", server.ReadHeaderTimeout, serverReadHeaderTimeout)
	}
	if server.ReadTimeout != serverReadTimeout {
		t.Fatalf("read timeout = %s, want %s", server.ReadTimeout, serverReadTimeout)
	}
	if server.Handler == nil {
		t.Fatal("handler is nil")
	}
}
