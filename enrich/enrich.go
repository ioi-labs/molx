package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nexora-crawl/config"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
	"nexora-crawl/scraper"
	"nexora-crawl/search"
	"nexora-crawl/validator"
)

// Config for the LLM enrichment endpoint.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Client asks an OpenAI-compatible endpoint to produce JSON matching a schema.
type Client struct {
	cfg    Config
	httpDo func(*http.Request) (*http.Response, error)
}

// NewClient creates an enrichment LLM client.
// ponytail: stdlib HTTP only; no OpenAI SDK dependency.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, httpDo: http.DefaultClient.Do}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Enrich asks the LLM to return JSON matching schema according to the prompt.
func (c *Client) Enrich(ctx context.Context, prompt string, schema any, contextText string) (any, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" || c.cfg.Model == "" {
		return nil, errors.New("LLM not provided")
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	system := fmt.Sprintf(`You are a data extraction assistant.
The user wants structured data extracted from web content.
Return ONLY a JSON object that matches the JSON Schema below. Do not wrap the output in markdown code fences or add explanatory text.

JSON Schema:
%s

User extraction instruction:
%s`, schemaJSON, prompt)

	payload := chatRequest{
		Model: c.cfg.Model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: contextText},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.cfg.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpDo(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out chatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, errors.New(out.Error.Message)
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return nil, errors.New("LLM returned empty content")
	}

	var result any
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("LLM returned invalid JSON: %w", err)
	}
	return result, nil
}

// JobState tracks an enrichment job.
type JobState struct {
	ID          string
	Status      string
	Prompt      string
	Schema      any
	URLs        []string
	Result      any
	Error       string
	CreatedAt   time.Time
	CompletedAt *time.Time
	ExpiresAt   time.Time
	mu          sync.RWMutex
}

func (j *JobState) snapshot() models.EnrichStatusResponse {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return models.EnrichStatusResponse{
		ID:          j.ID,
		Status:      j.Status,
		Prompt:      j.Prompt,
		Schema:      j.Schema,
		URLs:        append([]string(nil), j.URLs...),
		Result:      copyAny(j.Result),
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		CompletedAt: j.CompletedAt,
		ExpiresAt:   j.ExpiresAt,
	}
}

// copyAny returns a deep copy of a JSON-shaped value via marshal/unmarshal.
// ponytail: avoids caller mutating shared job result without pulling in a cloning library.
func copyAny(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func (j *JobState) setRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "running"
}

func (j *JobState) setCompleted(result any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "completed"
	j.Result = result
	now := time.Now()
	j.CompletedAt = &now
}

func (j *JobState) setFailed(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "failed"
	j.Error = err
	now := time.Now()
	j.CompletedAt = &now
}

// Store keeps enrichment jobs in memory.
// ponytail: in-memory only, persist when data loss actually matters.
type Store struct {
	mu   sync.RWMutex
	jobs map[string]*JobState
}

func NewStore() *Store {
	return &Store{jobs: make(map[string]*JobState)}
}

func (s *Store) Save(j *JobState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

func (s *Store) Get(id string) (*JobState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

// Runner executes enrichment jobs asynchronously.
type Runner struct {
	store   *Store
	scraper *scraper.V2Scraper
	engines []search.Engine
	llm     *Client
	cfg     *config.Config
}

// NewRunner wires the runner with dependencies needed to search, scrape, and enrich.
func NewRunner(store *Store, cfg *config.Config, client obscura.Fetcher, engines []search.Engine) *Runner {
	return &Runner{
		store:   store,
		scraper: scraper.NewV2(cfg, client),
		engines: engines,
		llm: NewClient(Config{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
		}),
		cfg: cfg,
	}
}

// Start creates and schedules an enrichment job.
func (r *Runner) Start(ctx context.Context, req models.EnrichRequest) *JobState {
	job := &JobState{
		ID:        uuid.New().String(),
		Status:    "pending",
		Prompt:    req.Prompt,
		Schema:    req.Schema,
		URLs:      req.URLs,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	r.store.Save(job)
	go func() { r.run(ctx, job, req) }()
	return job
}

func (r *Runner) run(ctx context.Context, job *JobState, req models.EnrichRequest) {
	urls := req.URLs
	slog.Info("enrich started", "id", job.ID, "urls_count", len(urls), "has_urls", len(urls) > 0)

	// ponytail: single search pass; no recursive browsing or planning agent.
	if len(urls) == 0 {
		var err error
		urls, err = r.searchURLs(ctx, job.ID, req.Prompt)
		if err != nil {
			slog.Warn("enrich failed", "id", job.ID, "stage", "search", "error", err)
			job.setFailed(err.Error())
			r.store.Save(job)
			return
		}
		job.mu.Lock()
		job.URLs = urls
		job.mu.Unlock()
		r.store.Save(job)
	}

	contextText, err := r.scrapeAll(ctx, job.ID, urls)
	if err != nil {
		slog.Warn("enrich failed", "id", job.ID, "stage", "scrape", "error", err)
		job.setFailed(err.Error())
		r.store.Save(job)
		return
	}

	result, err := r.llm.Enrich(ctx, req.Prompt, req.Schema, contextText)
	if err != nil {
		slog.Warn("enrich failed", "id", job.ID, "stage", "llm", "error", err)
		job.setFailed(err.Error())
		r.store.Save(job)
		return
	}

	job.setCompleted(result)
	r.store.Save(job)
	slog.Info("enrich completed", "id", job.ID, "urls", len(urls), "context_bytes", len(contextText))
}

func (r *Runner) searchURLs(ctx context.Context, id, prompt string) ([]string, error) {
	if len(r.engines) == 0 {
		return nil, errors.New("search not configured")
	}
	opts := search.Options{
		Query:   prompt,
		Limit:   5,
		Proxy:   r.cfg.Proxy,
		Timeout: r.cfg.SearchTimeout,
	}
	results, err := search.Aggregate(ctx, r.engines, opts)
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(results))
	for _, res := range results {
		urls = append(urls, res.URL)
	}
	slog.Info("enrich search done", "id", id, "urls_found", len(urls))
	return urls, nil
}

func (r *Runner) scrapeAll(ctx context.Context, id string, urls []string) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("no urls to scrape")
	}

	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout*time.Duration(max(len(urls), 1)))
	defer cancel()

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	parts := make([]string, len(urls))

	for i, url := range urls {
		if err := validator.ValidateURL(url); err != nil {
			slog.Warn("enrich skipped invalid url", "id", id, "url", url, "error", err)
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, u string) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := r.scraper.Run(ctx, u, scraper.Options{
				Formats:         []string{"markdown"},
				OnlyMainContent: true,
				BlockAds:        true,
			})
			if err != nil {
				slog.Warn("enrich scrape failed", "id", id, "url", u, "error", err)
				return
			}
			parts[idx] = fmt.Sprintf("--- URL: %s ---\n%s", u, data.Markdown)
		}(i, url)
	}
	wg.Wait()

	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	if b.Len() == 0 {
		return "", errors.New("all scrapes failed")
	}
	slog.Info("enrich scrape done", "id", id, "urls", len(urls), "context_bytes", b.Len())
	return b.String(), nil
}
