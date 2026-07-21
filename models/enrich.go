package models

import "time"

// EnrichRequest starts an async enrichment job. If URLs is empty, the prompt
// is used to search the web; otherwise the provided URLs are scraped.
type EnrichRequest struct {
	Prompt string   `json:"prompt"`
	Schema any      `json:"schema"`
	URLs   []string `json:"urls,omitempty"`
}

// EnrichCreateResponse is returned immediately when a job is accepted.
type EnrichCreateResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
	Status  string `json:"status"`
}

// EnrichStatusResponse is returned by GET /enrich/{id}.
type EnrichStatusResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Prompt      string    `json:"prompt"`
	Schema      any       `json:"schema,omitempty"`
	URLs        []string  `json:"urls,omitempty"`
	Result      any       `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt"`
}
