package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tomkqwe/metrics/internal/retry"
)

// HTTPObserver posts JSON events with a bounded delivery time.
type HTTPObserver struct {
	url         string
	client      *http.Client
	retryDelays []time.Duration
}

// NewHTTPObserver creates an observer for an absolute HTTP(S) URL with a five-second timeout.
func NewHTTPObserver(address string) (*HTTPObserver, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse audit URL: %w", err)
	}
	if parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("audit URL must be an absolute HTTP or HTTPS URL")
	}
	return &HTTPObserver{url: address, retryDelays: retry.DefaultDelays(), client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Notify posts JSON, retrying transport failures after 1, 3 and 5 seconds.
// Each attempt has a five-second timeout. Non-2xx responses are not retried.
func (h *HTTPObserver) Notify(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	return retry.DoWithDelays(ctx, h.retryDelays, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create audit request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := h.client.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %w", errTransport, err)
		}
		defer resp.Body.Close()
		// Bound draining so an unexpectedly large response cannot hold a worker indefinitely.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("audit receiver returned status %d", resp.StatusCode)
		}
		return nil
	}, func(err error) bool {
		return ctx.Err() == nil && errors.Is(err, errTransport)
	})
}

var errTransport = errors.New("send audit event")
