package enrich

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"nexora-crawl/middleware"
	"nexora-crawl/models"
)

// CreateHandler serves POST /enrich.
type CreateHandler struct {
	Runner *Runner
}

func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.EnrichRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing prompt")
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	job := h.Runner.Start(context.Background(), req)
	resp := models.EnrichCreateResponse{
		Success: true,
		ID:      job.ID,
		URL:     scheme + "://" + r.Host + "/enrich/" + job.ID,
		Status:  job.snapshot().Status,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// StatusHandler serves GET /enrich/{id}.
type StatusHandler struct {
	Store *Store
}

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := h.Store.Get(id)
	if !ok {
		middleware.WriteJSONError(w, http.StatusNotFound, "enrich job not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(job.snapshot())
}
