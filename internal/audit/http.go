package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// HTTPObserver posts JSON events with a bounded delivery time.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver creates an observer for an absolute HTTP(S) URL with a five-second timeout.
func NewHTTPObserver(address string) (*HTTPObserver, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("audit URL must be an absolute HTTP or HTTPS URL")
	}
	return &HTTPObserver{url: address, client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Notify posts an event as JSON and returns an error for transport failures or non-2xx responses.
func (h *HTTPObserver) Notify(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create audit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("send audit event: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audit receiver returned status %d", resp.StatusCode)
	}
	return nil
}
