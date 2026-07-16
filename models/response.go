package models

// FetchResponse is the unified JSON envelope returned by POST /fetch.
type FetchResponse struct {
	OK         bool   `json:"ok"`
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	ElapsedMs  int64  `json:"elapsed_ms,omitempty"`
}

// ScrapeResponse is the unified JSON envelope returned by POST /scrape.
type ScrapeResponse struct {
	OK          bool    `json:"ok"`
	TotalURLs   int     `json:"total_urls,omitempty"`
	TotalTimeMs int64   `json:"total_time_ms,omitempty"`
	Results     []any   `json:"results,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}
