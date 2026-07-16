package models

// V2ScrapeRequest mirrors the Firecrawl v2 scrape request shape.
// We accept both `url` and `urls` for compatibility, but only the first URL is
// processed.
type V2ScrapeRequest struct {
	URL              string           `json:"url,omitempty"`
	URLs             []string         `json:"urls,omitempty"`
	Formats          []string         `json:"formats,omitempty"`
	OnlyMainContent  bool             `json:"onlyMainContent,omitempty"`
	IncludeTags      []string         `json:"includeTags,omitempty"`
	ExcludeTags      []string         `json:"excludeTags,omitempty"`
	WaitFor          int              `json:"waitFor,omitempty"` // milliseconds
	Timeout          int              `json:"timeout,omitempty"` // milliseconds
	Mobile           bool             `json:"mobile,omitempty"`
	Proxy            string           `json:"proxy,omitempty"`
	BlockAds         bool             `json:"blockAds,omitempty"`
	Actions          []V2ScrapeAction `json:"actions,omitempty"`
}

// TargetURL returns the single URL to process. It prefers `url`, then the first
// entry in `urls`. An empty string means no URL was supplied.
func (r *V2ScrapeRequest) TargetURL() string {
	if r.URL != "" {
		return r.URL
	}
	if len(r.URLs) > 0 {
		return r.URLs[0]
	}
	return ""
}

// V2ScrapeAction is a page action to run before extraction.
// Only "wait" and "click" are supported.
type V2ScrapeAction struct {
	Type         string `json:"type"`
	Milliseconds int    `json:"milliseconds,omitempty"`
	Selector     string `json:"selector,omitempty"`
}

// V2ScrapeResponse mirrors the Firecrawl v2 scrape response shape.
type V2ScrapeResponse struct {
	Success bool          `json:"success"`
	Data    *V2ScrapeData `json:"data,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// V2ScrapeData holds the extracted content and metadata.
type V2ScrapeData struct {
	Markdown string           `json:"markdown,omitempty"`
	HTML     string           `json:"html,omitempty"`
	Text     string           `json:"text,omitempty"`
	Links    []string         `json:"links,omitempty"`
	Metadata V2ScrapeMetadata `json:"metadata"`
}

// V2ScrapeMetadata holds page-level metadata.
type V2ScrapeMetadata struct {
	Title     string `json:"title,omitempty"`
	Language  string `json:"language,omitempty"`
	SourceURL string `json:"sourceURL,omitempty"`
	URL       string `json:"url,omitempty"`
	Favicon   string `json:"favicon,omitempty"`
	Viewport  string `json:"viewport,omitempty"`
}
