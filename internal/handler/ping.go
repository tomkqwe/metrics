package handler

import (
	"context"
	"net/http"
)

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type PingHandler struct {
	db DatabasePinger
}

func NewPingHandler(db DatabasePinger) *PingHandler {
	return &PingHandler{
		db: db,
	}
}

func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := h.db.PingContext(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
