package models

import "time"

// BatchScrapeRequest mirrors Firecrawl's POST /v2/batch/scrape body.
type BatchScrapeRequest struct {
	URLs              []string         `json:"urls"`
	IgnoreInvalidURLs bool             `json:"ignoreInvalidURLs,omitempty"`
	MaxConcurrency    int              `json:"maxConcurrency,omitempty"`
	Formats           []string         `json:"formats,omitempty"`
	OnlyMainContent   bool             `json:"onlyMainContent,omitempty"`
	IncludeTags       []string         `json:"includeTags,omitempty"`
	ExcludeTags       []string         `json:"excludeTags,omitempty"`
	WaitFor           int              `json:"waitFor,omitempty"` // milliseconds
	Timeout           int              `json:"timeout,omitempty"` // milliseconds
	Mobile            bool             `json:"mobile,omitempty"`
	Proxy             string           `json:"proxy,omitempty"`
	BlockAds          bool             `json:"blockAds,omitempty"`
	Actions           []V2ScrapeAction `json:"actions,omitempty"`
}

// BatchScrapeResponse is returned immediately when a batch job is created.
type BatchScrapeResponse struct {
	Success     bool     `json:"success"`
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	InvalidURLs []string `json:"invalidURLs,omitempty"`
}

// BatchScrapeStatusResponse is returned by GET /v2/batch/scrape/{id}.
type BatchScrapeStatusResponse struct {
	Status      string                  `json:"status"`
	Total       int                     `json:"total"`
	Completed   int                     `json:"completed"`
	CreatedAt   time.Time               `json:"createdAt"`
	CompletedAt *time.Time              `json:"completedAt,omitempty"`
	ExpiresAt   time.Time               `json:"expiresAt"`
	Duration    float64                 `json:"duration"`
	Next        *string                 `json:"next,omitempty"`
	Data        []BatchScrapeResultItem `json:"data,omitempty"`
}

// BatchScrapeResultItem is one page result inside the batch status data array.
type BatchScrapeResultItem struct {
	Markdown string           `json:"markdown,omitempty"`
	HTML     string           `json:"html,omitempty"`
	RawHTML  string           `json:"rawHtml,omitempty"`
	Links    []string         `json:"links,omitempty"`
	Metadata V2ScrapeMetadata `json:"metadata"`
	Error    string           `json:"error,omitempty"`
}
