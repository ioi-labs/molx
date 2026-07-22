package scraper

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

// V2ScrapeHandler serves POST /scrape and POST /v2/scrape using the shared
// single-URL scraper.
type V2ScrapeHandler struct {
	Config  *config.Config
	Client  obscura.Fetcher
	Scraper *V2Scraper
}

func (h *V2ScrapeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.V2ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	targetURL := req.TargetURL()
	if targetURL == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing url")
		return
	}
	if err := validator.ValidateURL(targetURL); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.Config.Timeout)
	defer cancel()

	start := time.Now()
	data, err := h.Scraper.Run(ctx, targetURL, Options{
		Formats:          req.Formats,
		OnlyMainContent:  req.OnlyMainContent,
		OnlyCleanContent: req.OnlyCleanContent,
		IncludeTags:      req.IncludeTags,
		ExcludeTags:      req.ExcludeTags,
		WaitFor:          req.WaitFor,
		Timeout:          req.Timeout,
		Mobile:           req.Mobile,
		Proxy:            firstNonEmpty(req.Proxy, h.Config.Proxy),
		BlockAds:         req.BlockAds,
		Actions:          req.Actions,
	})
	elapsed := time.Since(start).Milliseconds()

	resp := models.V2ScrapeResponse{Success: err == nil}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Data = data
	}
	_ = elapsed

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
