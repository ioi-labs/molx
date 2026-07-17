package search

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Aggregate runs a query across multiple engines concurrently, deduplicates by
// URL, and interleaves results from each engine.
func Aggregate(ctx context.Context, engines []Engine, opts Options) ([]Result, error) {
	if len(engines) == 0 {
		return nil, ErrNoResults
	}

	type engineResult struct {
		idx int
		res []Result
		err error
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	results := make([]engineResult, len(engines))
	var wg sync.WaitGroup

	for i, eng := range engines {
		wg.Add(1)
		go func(idx int, e Engine) {
			defer wg.Done()
			res, err := e.Search(ctx, opts)
			results[idx] = engineResult{idx: idx, res: res, err: err}
			if err != nil {
				slog.Warn("search engine failed", "engine", e.Name(), "error", err)
			}
		}(i, eng)
	}
	wg.Wait()

	// Try each engine in a deterministic random order, taking as many results as
	// possible from the first working engine, then falling back to the next one
	// when the current engine is exhausted or failed. This mirrors how a user
	// would retry a query across providers.
	order := deterministicOrder(engines, opts.Query)

	var merged []Result
	seen := make(map[string]bool)
	for _, idx := range order {
		r := results[idx]
		if r.err != nil {
			continue
		}
		for _, item := range r.res {
			key := normalizeURLKey(item.URL)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, item)
		}
	}

	if len(merged) == 0 {
		// If all engines failed, surface the first error in original order.
		for _, r := range results {
			if r.err != nil {
				return nil, r.err
			}
		}
		return nil, ErrNoResults
	}

	limit := ClampLimit(opts.Limit)
	if limit > len(merged) {
		limit = len(merged)
	}
	return merged[:limit], nil
}

// deterministicOrder returns a pseudo-random permutation of engine indices that
// is stable for the same query string, so repeated requests for the same query
// hit the same primary engine but different queries spread load across engines.
func deterministicOrder(engines []Engine, query string) []int {
	indices := make([]int, len(engines))
	for i := range engines {
		indices[i] = i
	}
	if len(indices) <= 1 {
		return indices
	}

	// Mix the query with the engine index and shuffle.
	seed := uint64(0)
	for _, r := range query {
		seed = seed*31 + uint64(r)
	}
	r := rand.New(rand.NewSource(int64(seed)))
	r.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})
	return indices
}

// AggregateOptions is a small alias exported for callers that wire engines.
type AggregateOptions = Options

func normalizeURLKey(raw string) string {
	u := strings.ToLower(raw)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "www.")
	u = strings.TrimSuffix(u, "/")
	return u
}

// TimeRangeCode converts a generic time range to engine-specific codes.
func TimeRangeCode(engine, tr string) string {
	switch engine {
	case "brave":
		m := map[string]string{"day": "pd", "week": "pw", "month": "pm", "year": "py"}
		return m[tr]
	case "duckduckgo", "startpage":
		return TimeRange[tr]
	}
	return ""
}

// SearXNGResult maps a normalized Result to the SearXNG JSON shape.
func SearXNGResult(r Result) SearXNGResultModel {
	return SearXNGResultModel{
		Title:   r.Title,
		URL:     r.URL,
		Content: r.Description,
	}
}

// SearXNGResultModel is the legacy JSON shape exposed by /search.
type SearXNGResultModel struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// V2Result maps a normalized Result to the Firecrawl v2 search shape.
func V2Result(r Result, sourceURL string) V2SearchResultModel {
	return V2SearchResultModel{
		Title:       r.Title,
		URL:         r.URL,
		Description: r.Description,
		Metadata: V2ScrapeMetadataModel{
			SourceURL: sourceURL,
			URL:       sourceURL,
		},
	}
}

// V2SearchResultModel is the response item shape for /v2/search.
type V2SearchResultModel struct {
	Title       string             `json:"title,omitempty"`
	URL         string             `json:"url,omitempty"`
	Description string             `json:"description,omitempty"`
	Markdown    string             `json:"markdown,omitempty"`
	HTML        string             `json:"html,omitempty"`
	Text        string             `json:"text,omitempty"`
	Links       []string           `json:"links,omitempty"`
	Metadata    V2ScrapeMetadataModel `json:"metadata"`
}

// V2ScrapeMetadataModel mirrors the existing models.V2ScrapeMetadata shape
// without importing the larger models package (avoiding cycle).
type V2ScrapeMetadataModel struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	SourceURL   string `json:"sourceURL,omitempty"`
	URL         string `json:"url,omitempty"`
}

// init ensures unused imports do not break future expansion.
func init() { _ = time.Second }
