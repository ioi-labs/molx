package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/middleware"
	"nexora-crawl/models"
	"nexora-crawl/scraper"
)

// SearchHandler serves POST /search by proxying queries to SearXNG and
// optionally scraping each result.
type SearchHandler struct {
	Config  *config.Config
	Scraper *scraper.V2Scraper
}

const maxSearchLimit = 100
const defaultSearchLimit = 10
const defaultSearchScrapeLimit = 5
const searchScrapeConcurrency = 5
const defaultSearchScrapeTimeoutMs = 60000

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		middleware.WriteJSONError(w, http.StatusBadRequest, "missing query")
		return
	}
	if h.Config.SearXNGURL == "" {
		middleware.WriteJSONError(w, http.StatusServiceUnavailable, "search not configured")
		return
	}

	searxResp, err := h.searchSearXNG(r.Context(), req)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusBadGateway, fmt.Sprintf("searxng failed: %v", err))
		return
	}

	results := h.mapResults(searxResp.Results, req)

	if req.WantsSearchScrape() {
		scrapeTimeout := searchScrapeTimeout(results, req.ScrapeOptions)
		ctx, cancel := context.WithTimeout(r.Context(), scrapeTimeout)
		defer cancel()
		results = h.scrapeResults(ctx, results, req.ScrapeOptions)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.SearchResponse{Success: true, Data: results})
}

func (h *SearchHandler) searchSearXNG(ctx context.Context, req models.SearchRequest) (*models.SearXNGResponse, error) {
	u, err := url.Parse(h.Config.SearXNGURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", req.Query)
	q.Set("format", "json")
	q.Set("categories", "general")
	if req.Language != "" {
		q.Set("language", req.Language)
	}
	if req.TimeRange != "" {
		q.Set("time_range", req.TimeRange)
	}
	if req.SafeSearch > 0 {
		q.Set("safesearch", strconv.Itoa(req.SafeSearch))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NexoraCrawl/1.0)")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("status %d (SearXNG JSON API may be disabled on this instance)", resp.StatusCode)
		}
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var out models.SearXNGResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *SearchHandler) mapResults(searx []models.SearXNGResult, req models.SearchRequest) []models.SearchResult {
	limit := req.Limit
	if limit <= 0 || limit > maxSearchLimit {
		if req.WantsSearchScrape() {
			limit = defaultSearchScrapeLimit
		} else {
			limit = defaultSearchLimit
		}
	}
	if limit > len(searx) {
		limit = len(searx)
	}

	out := make([]models.SearchResult, 0, limit)
	for i := 0; i < limit; i++ {
		res := searx[i]
		out = append(out, models.SearchResult{
			Title:       strings.TrimSpace(res.Title),
			URL:         res.URL,
			Description: strings.TrimSpace(res.Content),
			Metadata:    models.V2ScrapeMetadata{SourceURL: res.URL, URL: res.URL},
		})
	}
	return out
}

func searchScrapeTimeout(results []models.SearchResult, opts *models.SearchScrapeOptions) time.Duration {
	perResult := opts.Timeout
	if perResult <= 0 {
		perResult = defaultSearchScrapeTimeoutMs
	}
	return time.Duration(len(results))*time.Duration(perResult)*time.Millisecond + 10*time.Second
}

func (h *SearchHandler) scrapeResults(ctx context.Context, results []models.SearchResult, opts *models.SearchScrapeOptions) []models.SearchResult {
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

func (h *SearchHandler) scrapeOne(ctx context.Context, item models.SearchResult, opts *models.SearchScrapeOptions) models.SearchResult {
	start := time.Now()
	data, err := h.Scraper.Run(ctx, item.URL, scraper.Options{
		Formats:         opts.Formats,
		OnlyMainContent: opts.OnlyMainContent,
		IncludeTags:     opts.IncludeTags,
		ExcludeTags:     opts.ExcludeTags,
		WaitFor:         opts.WaitFor,
		Timeout:         opts.Timeout,
		Mobile:          opts.Mobile,
		Proxy:           opts.Proxy,
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
