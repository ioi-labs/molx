package batch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

func TestCreateHandler_BadRequest(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	runner := NewRunner(store, cfg, nil)
	h := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/batch/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateAndStatusFlow(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()

	// Use a runner with a scraper that has no real Obscura client, but prevent
	// the async goroutine from running by completing the job manually before
	// it can call the scraper.
	runner := NewRunner(store, cfg, nil)

	create := &CreateHandler{Runner: runner}
	status := &StatusHandler{Store: store}

	body := bytes.NewReader([]byte(`{"urls":["https://example.com"]}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/batch/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d", rr.Code, http.StatusOK)
	}

	var createResp models.BatchScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid create response: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected job id, got empty")
	}

	// The real goroutine would call Obscura (nil client), so we complete the job
	// immediately to test the status handler independently.
	job, ok := store.Get(createResp.ID)
	if !ok {
		t.Fatalf("job not found in store")
	}
	job.addResult(models.BatchScrapeResultItem{
		Markdown: "# Hello",
		Metadata: models.V2ScrapeMetadata{SourceURL: "https://example.com", URL: "https://example.com"},
	})
	job.setCompleted()

	statusReq := httptest.NewRequest(http.MethodGet, "/v2/batch/scrape/"+createResp.ID, nil)
	rr = httptest.NewRecorder()
	status.ServeHTTP(rr, withChiParam(statusReq, "id", createResp.ID))

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}

	var statusResp models.BatchScrapeStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("invalid status response: %v", err)
	}
	if statusResp.Status != "completed" {
		t.Errorf("status = %q, want completed", statusResp.Status)
	}
	if statusResp.Total != 1 {
		t.Errorf("total = %d, want 1", statusResp.Total)
	}
	if statusResp.Completed != 1 {
		t.Errorf("completed = %d, want 1", statusResp.Completed)
	}
	if len(statusResp.Data) != 1 {
		t.Fatalf("data len = %d, want 1", len(statusResp.Data))
	}
	if statusResp.Data[0].Markdown != "# Hello" {
		t.Errorf("markdown = %q", statusResp.Data[0].Markdown)
	}
}

func TestCreateHandler_InvalidURL(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	runner := NewRunner(store, cfg, nil)
	create := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{"urls":["http://localhost:8080"],"ignoreInvalidURLs":true}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/batch/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp models.BatchScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(resp.InvalidURLs) != 1 {
		t.Errorf("invalidURLs = %v, want 1", resp.InvalidURLs)
	}
}

func TestStatusHandler_NotFound(t *testing.T) {
	store := NewStore()
	status := &StatusHandler{Store: store}

	req := httptest.NewRequest(http.MethodGet, "/v2/batch/scrape/not-real", nil)
	rr := httptest.NewRecorder()
	status.ServeHTTP(rr, withChiParam(req, "id", "not-real"))

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func withChiParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
