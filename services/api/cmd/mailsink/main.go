// Command mailsink is a test-only HTTP capture endpoint for transactional mail.
// It accepts the Resend POST /emails payload shape, stores each message in
// memory keyed by recipient, and exposes GET /messages?to=<address> so smoke
// and integration tests can extract verification/reset tokens without a real
// provider. It is never deployed to production.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
)

type message struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

type store struct {
	mu       sync.Mutex
	messages []message
}

func (s *store) add(msg message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

func (s *store) forRecipient(address string) []message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var matches []message
	for i := len(s.messages) - 1; i >= 0; i-- {
		for _, to := range s.messages[i].To {
			if strings.EqualFold(to, address) {
				matches = append(matches, s.messages[i])
				break
			}
		}
	}
	return matches
}

func main() {
	addr := os.Getenv("SDDS_MAILSINK_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	messages := &store{}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /emails", func(w http.ResponseWriter, r *http.Request) {
		var msg message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		messages.add(msg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "accepted"})
	})
	mux.HandleFunc("GET /messages", func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("to")
		w.Header().Set("Content-Type", "application/json")
		result := messages.forRecipient(address)
		if result == nil {
			result = []message{}
		}
		_ = json.NewEncoder(w).Encode(map[string][]message{"messages": result})
	})

	_ = http.ListenAndServe(addr, mux)
}
