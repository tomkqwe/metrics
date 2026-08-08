package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/tomkqwe/metrics/internal/postgreserr"
	"github.com/tomkqwe/metrics/internal/retry"
)

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type PingHandler struct {
	db          DatabasePinger
	retryDelays []time.Duration
}

func NewPingHandler(db DatabasePinger) *PingHandler {
	return &PingHandler{
		db:          db,
		retryDelays: retry.DefaultDelays(),
	}
}

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
