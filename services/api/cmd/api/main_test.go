package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewServerSetsReadTimeouts(t *testing.T) {
	server := newServer(config{httpAddr: ":9090"}, http.NotFoundHandler())

	got := serverSettings{
		Addr:              server.Addr,
		ReadHeaderTimeout: server.ReadHeaderTimeout,
		ReadTimeout:       server.ReadTimeout,
	}
	want := serverSettings{
		Addr:              ":9090",
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("server settings mismatch (-want +got):\n%s", diff)
	}
	if server.Handler == nil {
		t.Fatal("handler is nil")
	}
}

type serverSettings struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
}
