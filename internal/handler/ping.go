package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/tomkqwe/metrics/internal/postgreserr"
	"github.com/tomkqwe/metrics/internal/retry"
)

// DatabasePinger checks whether the database can be reached.
type DatabasePinger interface {
	// PingContext checks connectivity, observing context cancellation.
	PingContext(ctx context.Context) error
}

// PingHandler serves the database health endpoint.
type PingHandler struct {
	db          DatabasePinger
	retryDelays []time.Duration
}

// NewPingHandler creates a health handler; a nil database makes Ping return 500.
func NewPingHandler(db DatabasePinger) *PingHandler {
	return &PingHandler{
		db:          db,
		retryDelays: retry.DefaultDelays(),
	}
}

// Ping handles GET /ping and returns 200 for a reachable database or 500 otherwise.
// PostgreSQL connection exceptions are retried with the configured delays.
func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := retry.DoWithDelays(r.Context(), h.retryDelays, func() error {
		return h.db.PingContext(r.Context())
	}, postgreserr.IsConnectionException); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
