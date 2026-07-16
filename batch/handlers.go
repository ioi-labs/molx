package batch

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nexora-crawl/middleware"
	"nexora-crawl/models"
)

// CreateHandler serves POST /v2/batch/scrape.
type CreateHandler struct {
	Runner *Runner
}

func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.BatchScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.URLs) == 0 {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing urls")
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	job := h.Runner.Start(context.Background(), req)
	resp := models.BatchScrapeResponse{
		Success:     true,
		ID:          job.ID,
		URL:         scheme + "://" + r.Host + "/v2/batch/scrape/" + job.ID,
		InvalidURLs: job.InvalidURLs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// StatusHandler serves GET /v2/batch/scrape/{id}.
type StatusHandler struct {
	Store *Store
}

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := h.Store.Get(id)
	if !ok {
		middleware.WriteJSONError(w, http.StatusNotFound, "batch scrape not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(job.snapshot())
}
