package batch

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nexora-crawl/config"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
	"nexora-crawl/scraper"
	"nexora-crawl/validator"
)

// JobState tracks the lifecycle and results of a batch scrape.
type JobState struct {
	ID          string
	Status      string
	Total       int
	Completed   int
	InvalidURLs []string
	CreatedAt   time.Time
	CompletedAt *time.Time
	ExpiresAt   time.Time
	Results     []models.BatchScrapeResultItem
	mu          sync.RWMutex
}

func (j *JobState) snapshot() models.BatchScrapeStatusResponse {
	j.mu.RLock()
	defer j.mu.RUnlock()

	now := time.Now()
	duration := now.Sub(j.CreatedAt).Seconds()
	if j.CompletedAt != nil {
		duration = j.CompletedAt.Sub(j.CreatedAt).Seconds()
	}

	return models.BatchScrapeStatusResponse{
		Status:      j.Status,
		Total:       j.Total,
		Completed:   j.Completed,
		CreatedAt:   j.CreatedAt,
		CompletedAt: j.CompletedAt,
		ExpiresAt:   j.ExpiresAt,
		Duration:    duration,
		Next:        nil,
		Data:        append([]models.BatchScrapeResultItem(nil), j.Results...),
	}
}

func (j *JobState) setCompleted() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "completed"
	now := time.Now()
	j.CompletedAt = &now
}

func (j *JobState) addResult(r models.BatchScrapeResultItem) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Results = append(j.Results, r)
	j.Completed++
}

// Store keeps jobs in memory.
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

// Runner executes batch scrapes asynchronously with bounded concurrency.
type Runner struct {
	store   *Store
	scraper *scraper.V2Scraper
}

func NewRunner(store *Store, cfg *config.Config, client obscura.Fetcher) *Runner {
	return &Runner{
		store:   store,
		scraper: scraper.NewV2(cfg, client),
	}
}

// Start creates and schedules a new batch job.
func (r *Runner) Start(ctx context.Context, req models.BatchScrapeRequest) *JobState {
	job := &JobState{
		ID:        uuid.New().String(),
		Status:    "scraping",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	validURLs := make([]string, 0, len(req.URLs))
	for _, u := range req.URLs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := validator.ValidateURL(u); err != nil {
			if !req.IgnoreInvalidURLs {
				job.InvalidURLs = append(job.InvalidURLs, u)
				job.Total = 0
				job.setCompleted()
				r.store.Save(job)
				return job
			}
			job.InvalidURLs = append(job.InvalidURLs, u)
			continue
		}
		validURLs = append(validURLs, u)
	}

	job.Total = len(validURLs)
	r.store.Save(job)

	if job.Total == 0 {
		job.setCompleted()
		return job
	}

	concurrency := req.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	go r.runAll(ctx, job, validURLs, req, concurrency)
	return job
}

func (r *Runner) runAll(ctx context.Context, job *JobState, urls []string, req models.BatchScrapeRequest, concurrency int) {
	ctx, cancel := context.WithTimeout(ctx, r.scraper.Config.Timeout*time.Duration(max(len(urls), 1)))
	defer cancel()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()
			r.runOne(ctx, job, u, req)
		}(url)
	}

	wg.Wait()
	job.setCompleted()
}

func (r *Runner) runOne(ctx context.Context, job *JobState, url string, req models.BatchScrapeRequest) {
	defer func() {
		if rec := recover(); rec != nil {
			job.addResult(models.BatchScrapeResultItem{
				Metadata: models.V2ScrapeMetadata{SourceURL: url, URL: url},
			})
		}
	}()

	start := time.Now()
	data, err := r.scraper.Run(ctx, url, scraper.Options{
		Formats:         req.Formats,
		OnlyMainContent:  req.OnlyMainContent,
		OnlyCleanContent: req.OnlyCleanContent,
		IncludeTags:      req.IncludeTags,
		ExcludeTags:      req.ExcludeTags,
		WaitFor:          req.WaitFor,
		Timeout:          req.Timeout,
		Mobile:           req.Mobile,
		Proxy:            firstNonEmpty(req.Proxy, r.scraper.Config.Proxy),
		BlockAds:         req.BlockAds,
		Actions:          req.Actions,
	})

	item := models.BatchScrapeResultItem{
		Metadata: models.V2ScrapeMetadata{SourceURL: url, URL: url},
	}
	if err == nil {
		item.Markdown = data.Markdown
		item.HTML = data.HTML
		item.Links = data.Links
		item.Metadata = data.Metadata
		slog.Info("batch scrape finished", "url", url, "elapsed_ms", time.Since(start).Milliseconds())
	} else {
		item.Error = err.Error()
		slog.Warn("batch scrape failed", "url", url, "error", err, "elapsed_ms", time.Since(start).Milliseconds())
	}
	job.addResult(item)
}
