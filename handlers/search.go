package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/middleware"
	"nexora-crawl/models"
	"nexora-crawl/scraper"
	"nexora-crawl/search"
)

// V2SearchHandler serves POST /v2/search with a Firecrawl-compatible shape.
// It queries the configured native search engines and optionally scrapes each result.
type V2SearchHandler struct {
	Config  *config.Config
	Scraper *scraper.V2Scraper
	Engines []search.Engine
}

const maxSearchLimit = 100
const defaultSearchLimit = 10
const defaultSearchScrapeLimit = 5
const searchScrapeConcurrency = 5
const defaultSearchScrapeTimeoutMs = 60000

func (h *V2SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.V2SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing query")
		return
	}
	if len(h.Engines) == 0 {
		middleware.WriteJSONError(w, http.StatusServiceUnavailable, "search not configured")
		return
	}

	searxResp, err := h.searchNative(r.Context(), req)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusBadGateway, fmt.Sprintf("search failed: %v", err))
		return
	}

	results := h.mapResults(searxResp, req)

	if req.WantsSearchScrape() {
		scrapeTimeout := searchScrapeTimeout(results, req.ScrapeOptions)
		ctx, cancel := context.WithTimeout(r.Context(), scrapeTimeout)
		defer cancel()
		results = h.scrapeResults(ctx, results, req.ScrapeOptions)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.V2SearchResponse{Success: true, Data: results})
}

func (h *V2SearchHandler) searchNative(ctx context.Context, req models.V2SearchRequest) ([]search.Result, error) {
	opts := search.Options{
		Query:      req.Query,
		Locale:     req.Language,
		TimeRange:  req.TimeRange,
		SafeSearch: req.SafeSearch,
		Page:       1,
		Limit:      req.Limit,
		Proxy:      h.Config.Proxy,
		Timeout:    h.Config.SearchTimeout,
	}
	return search.Aggregate(ctx, h.Engines, opts)
}

func (h *V2SearchHandler) mapResults(results []search.Result, req models.V2SearchRequest) []models.V2SearchResult {
	limit := req.Limit
	if limit <= 0 || limit > maxSearchLimit {
		if req.WantsSearchScrape() {
			limit = defaultSearchScrapeLimit
		} else {
			limit = defaultSearchLimit
		}
	}
	if limit > len(results) {
		limit = len(results)
	}

	out := make([]models.V2SearchResult, 0, limit)
	for i := 0; i < limit; i++ {
		res := results[i]
		out = append(out, models.V2SearchResult{
			Title:       strings.TrimSpace(res.Title),
			URL:         res.URL,
			Description: strings.TrimSpace(res.Description),
			Metadata:    models.V2ScrapeMetadata{SourceURL: res.URL, URL: res.URL},
		})
	}
	return out
}

func searchScrapeTimeout(results []models.V2SearchResult, opts *models.V2SearchScrapeOptions) time.Duration {
	perResult := opts.Timeout
	if perResult <= 0 {
		perResult = defaultSearchScrapeTimeoutMs
	}
	return time.Duration(len(results))*time.Duration(perResult)*time.Millisecond + 10*time.Second
}

func (h *V2SearchHandler) scrapeResults(ctx context.Context, results []models.V2SearchResult, opts *models.V2SearchScrapeOptions) []models.V2SearchResult {
	sem := make(chan struct{}, searchScrapeConcurrency)
	var wg sync.WaitGroup

	for i := range results {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = h.scrapeOne(ctx, results[idx], opts)
		}(i)
	}
	wg.Wait()
	return results
}

func (h *V2SearchHandler) scrapeOne(ctx context.Context, item models.V2SearchResult, opts *models.V2SearchScrapeOptions) models.V2SearchResult {
	start := time.Now()
	data, err := h.Scraper.Run(ctx, item.URL, scraper.Options{
		Formats:         opts.Formats,
		OnlyMainContent: opts.OnlyMainContent,
		IncludeTags:     opts.IncludeTags,
		ExcludeTags:     opts.ExcludeTags,
		WaitFor:         opts.WaitFor,
		Timeout:         opts.Timeout,
		Mobile:          opts.Mobile,
		Proxy:           firstNonEmpty(opts.Proxy, h.Config.Proxy),
		BlockAds:        opts.BlockAds,
	})
	if err != nil {
		slog.Warn("search scrape failed", "url", item.URL, "error", err, "elapsed_ms", time.Since(start).Milliseconds())
		item.Metadata.Title = item.Title
		return item
	}

	slog.Info("search scrape finished", "url", item.URL, "elapsed_ms", time.Since(start).Milliseconds())
	item.Markdown = data.Markdown
	item.HTML = data.HTML
	item.Text = data.Text
	item.Links = data.Links
	item.Metadata = data.Metadata
	return item
}


// LegacySearchHandler proxies GET and POST /search using the native engines but
// returning the SearXNG-compatible JSON shape.
type LegacySearchHandler struct {
	Config  *config.Config
	Engines []search.Engine
}

func (h *LegacySearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(h.Engines) == 0 {
		middleware.WriteJSONError(w, http.StatusServiceUnavailable, "search not configured")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" && r.Method == http.MethodPost {
		_ = r.ParseForm()
		query = strings.TrimSpace(r.PostForm.Get("q"))
	}
	if query == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing query parameter q")
		return
	}

	opts := search.Options{
		Query:      query,
		Locale:     r.URL.Query().Get("language"),
		TimeRange:  r.URL.Query().Get("time_range"),
		SafeSearch: parseSafeSearch(r.URL.Query().Get("safesearch")),
		Page:       parsePage(r.URL.Query().Get("pageno")),
		Limit:      parseLimit(r.URL.Query().Get("limit")),
		Proxy:      h.Config.Proxy,
		Timeout:    h.Config.SearchTimeout,
	}

	results, err := search.Aggregate(r.Context(), h.Engines, opts)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusBadGateway, fmt.Sprintf("search failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.SearXNGResponse{
		Results: mapSearXNG(results),
	})
}

func mapSearXNG(results []search.Result) []models.SearXNGResult {
	out := make([]models.SearXNGResult, len(results))
	for i, r := range results {
		out[i] = models.SearXNGResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Description,
		}
	}
	return out
}

func parseSafeSearch(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 2 {
		return 0
	}
	return n
}

func parsePage(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 10
	}
	return n
}
