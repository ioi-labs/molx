package models

// FetchRequest represents a single-page fetch through the Obscura CLI.
type FetchRequest struct {
	URL         string `json:"url"`
	Dump        string `json:"dump,omitempty"`
	Selector    string `json:"selector,omitempty"`
	Wait        int    `json:"wait,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
	WaitUntil   string `json:"wait_until,omitempty"`
	Eval        string `json:"eval,omitempty"`
	Proxy       string `json:"proxy,omitempty"`
	Stealth     bool   `json:"stealth,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
	StorageDir  string `json:"storage_dir,omitempty"`
}

// ScrapeRequest represents a batch scrape job.
type ScrapeRequest struct {
	URLs        []string `json:"urls"`
	Eval        string   `json:"eval,omitempty"`
	Concurrency int      `json:"concurrency,omitempty"`
	Timeout     int      `json:"timeout,omitempty"`
	Format      string   `json:"format,omitempty"`
	Proxy       string   `json:"proxy,omitempty"`
	Stealth     bool     `json:"stealth,omitempty"`
	UserAgent   string   `json:"user_agent,omitempty"`
}
