package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnavailable wraps every failure mode of the mail provider: transport
// errors and non-2xx responses. Callers never distinguish those causes -- from
// the domain's perspective mail delivery is simply unavailable and there is no
// fallback mode.
var ErrUnavailable = errors.New("mail delivery unavailable")

// Message is a single transactional email to one recipient.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender sends a transactional message. The *Client implements it; the
// interface lets callers substitute a fake in tests.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

type Client struct {
	httpClient  *http.Client
	apiURL      string
	apiToken    string
	fromAddress string
	timeout     time.Duration
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

// New builds a Client from a loaded Config. It performs no network I/O; the
// provider is reached only by Send.
func New(config Config) (*Client, error) {
	if !config.loaded {
		return nil, errors.New("mail config is not loaded")
	}
	return &Client{
		httpClient:  &http.Client{},
		apiURL:      config.apiURL,
		apiToken:    config.apiToken,
		fromAddress: config.fromAddress,
		timeout:     config.timeout,
	}, nil
}

// Send posts one message to the Resend JSON API. Transport failures and any
// non-2xx response are reported wrapped in ErrUnavailable; the API token never
// appears in the returned error.
func (c *Client) Send(ctx context.Context, message Message) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := json.Marshal(sendRequest{
		From:    c.fromAddress,
		To:      []string{message.To},
		Subject: message.Subject,
		Text:    message.Text,
	})
	if err != nil {
		return fmt.Errorf("%w: encode request: %v", ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}
