package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tprei/sdds/services/api/internal/httpapi"
)

const defaultHTTPAddr = ":8080"

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := httpAddr()
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("api listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func httpAddr() string {
	addr := os.Getenv("SDDS_HTTP_ADDR")
	if addr == "" {
		return defaultHTTPAddr
	}
	return addr
}
