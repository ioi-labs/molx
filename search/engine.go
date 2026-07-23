package search

import (
	"context"
	"time"
)

// Engine defines a search provider that can be used by Molx.
type Engine interface {
	Name() string
	Search(ctx context.Context, opts Options) ([]Result, error)
}

// Options carries the common parameters for every search engine.
type Options struct {
	Query      string
	Locale     string
	TimeRange  string
	SafeSearch int
	Page       int
	Limit      int
	Proxy      string
	Timeout    time.Duration
}

// Result is a normalized web search result.
type Result struct {
	Title       string
	URL         string
	Description string
	PublishedAt *time.Time
	Thumbnail   string
}

// DefaultLimit is used when no limit is requested.
const DefaultLimit = 10

// MaxLimit caps the number of results returned per engine.
const MaxLimit = 100

// DefaultEngines are used when no engine list is configured.
var DefaultEngines = []string{"duckduckgo", "brave", "startpage"}
