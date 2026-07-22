package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/middleware"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
	"nexora-crawl/validator"
)

type FetchHandler struct {
	Config *config.Config
	Client obscura.Fetcher
}

func (h *FetchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.FetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.URL == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing url")
		return
	}
	if err := validator.ValidateURL(req.URL); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	timeout := h.Config.Timeout
	if req.Timeout > 0 {
		t := time.Duration(req.Timeout) * time.Second
		if t < timeout {
			timeout = t
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	start := time.Now()
	data, err := h.Client.Fetch(ctx, req)
	elapsed := time.Since(start).Milliseconds()

	resp := models.FetchResponse{
		OK:        err == nil,
		URL:       req.URL,
		ElapsedMs: elapsed,
	}
	if err != nil {
		resp.Error = err.Error()
	} else {
		var parsed any
		if err := json.Unmarshal(data, &parsed); err != nil {
			parsed = string(data)
		}
		resp.Result = parsed
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
