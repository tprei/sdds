package httpapi

import (
	"net"
	"net/http"
	"net/url"
)

const (
	corsAllowedHeaders = "Content-Type"
	corsAllowedMethods = "GET, POST, OPTIONS"
	headerOrigin       = "Origin"
)

func localBrowserCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get(headerOrigin)
		allowedOrigin := isAllowedLocalOrigin(origin)
		if allowedOrigin {
			w.Header().Add("Vary", headerOrigin)
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
		}

		if allowedOrigin && r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedLocalOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	switch parsed.Scheme {
	case "http", "https":
	default:
		return false
	}

	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
