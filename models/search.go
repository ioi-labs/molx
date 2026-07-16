package models

// SearchRequest mirrors a minimal Firecrawl search request.
type SearchRequest struct {
	Query         string         `json:"query"`
	Limit         int            `json:"limit,omitempty"`
	Language      string         `json:"language,omitempty"`
	TimeRange     string         `json:"time_range,omitempty"`
	SafeSearch    int            `json:"safesearch,omitempty"`
	ScrapeOptions *SearchScrapeOptions `json:"scrapeOptions,omitempty"`
}

// SearchScrapeOptions controls whether and how search results are scraped.
type SearchScrapeOptions struct {
	Formats         []string `json:"formats,omitempty"`
	OnlyMainContent bool     `json:"onlyMainContent,omitempty"`
	IncludeTags     []string `json:"includeTags,omitempty"`
	ExcludeTags     []string `json:"excludeTags,omitempty"`
	WaitFor         int      `json:"waitFor,omitempty"`
	Timeout         int      `json:"timeout,omitempty"`
	Mobile          bool     `json:"mobile,omitempty"`
	Proxy           string   `json:"proxy,omitempty"`
	BlockAds        bool     `json:"blockAds,omitempty"`
}

// SearchResponse is the unified JSON envelope returned by POST /search.
type SearchResponse struct {
	Success bool           `json:"success"`
	Data    []SearchResult `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// SearchResult is one item in the search response.
type SearchResult struct {
	Title       string           `json:"title,omitempty"`
	URL         string           `json:"url,omitempty"`
	Description string           `json:"description,omitempty"`
	Markdown    string           `json:"markdown,omitempty"`
	HTML        string           `json:"html,omitempty"`
	Text        string           `json:"text,omitempty"`
	Links       []string         `json:"links,omitempty"`
	Metadata    V2ScrapeMetadata `json:"metadata"`
}

// SearXNGResponse is the JSON body returned by SearXNG /search?format=json.
type SearXNGResponse struct {
	Results []SearXNGResult `json:"results"`
}

// SearXNGResult is one search result from SearXNG.
type SearXNGResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// WantsSearchScrape reports whether the request asks for any scrapeable format.
func (r *SearchRequest) WantsSearchScrape() bool {
	return r.ScrapeOptions != nil && len(r.ScrapeOptions.Formats) > 0
}
