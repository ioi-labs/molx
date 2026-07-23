package batch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"molx/config"
	"molx/models"
	"molx/obscura"
)

// fakeFetcher is a stub Obscura fetcher for batch tests.
type fakeFetcher struct {
	mu       sync.Mutex
	body     []byte
	htmlBody []byte
	err      error
	reqs     []models.FetchRequest
	calls    int
}

func (f *fakeFetcher) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	f.calls++
	if req.Dump == "html" && len(f.htmlBody) > 0 {
		return f.htmlBody, f.err
	}
	return f.body, f.err
}

func (f *fakeFetcher) firstReq() models.FetchRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		return models.FetchRequest{}
	}
	return f.reqs[0]
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

var _ obscura.Fetcher = (*fakeFetcher)(nil)


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
	fetcher := &fakeFetcher{body: []byte("# Hello")}
	runner := NewRunner(store, cfg, fetcher)

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

	deadline := time.Now().Add(2 * time.Second)
	var statusResp models.BatchScrapeStatusResponse
	for {
		statusReq := httptest.NewRequest(http.MethodGet, "/v2/batch/scrape/"+createResp.ID, nil)
		rr = httptest.NewRecorder()
		status.ServeHTTP(rr, withChiParam(statusReq, "id", createResp.ID))
		if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
			t.Fatalf("invalid status response: %v", err)
		}
		if statusResp.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not complete in time")
		}
		time.Sleep(10 * time.Millisecond)
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
	fetcher := &fakeFetcher{body: []byte("# Hello")}
	runner := NewRunner(store, cfg, fetcher)
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

func TestCreateHandler_Params(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	fetcher := &fakeFetcher{
		body:     []byte("# Hello"),
		htmlBody: []byte("<html><body><h1>Hello</h1><a href='/page'>page</a></body></html>"),
	}
	runner := NewRunner(store, cfg, fetcher)
	create := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{"urls":["https://example.com"],"ignoreInvalidURLs":true,"maxConcurrency":2,"formats":["markdown","links"],"onlyMainContent":true,"excludeTags":["aside"],"waitFor":1500,"timeout":10000,"mobile":true,"proxy":"http://proxy.example.com:8080","blockAds":true,"actions":[{"type":"wait","milliseconds":500}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/batch/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var createResp models.BatchScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected job id")
	}
	if len(createResp.InvalidURLs) != 0 {
		t.Errorf("invalidURLs = %v, want empty", createResp.InvalidURLs)
	}

	// Verify async runner eventually completes and respects params.
	deadline := time.Now().Add(2 * time.Second)
	for {
		job, ok := store.Get(createResp.ID)
		if !ok {
			t.Fatalf("job not found")
		}
		if job.snapshot().Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not complete in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if fetcher.callCount() == 0 {
		t.Fatalf("expected fetcher calls")
	}
	first := fetcher.firstReq()
	if first.Dump != "markdown" {
		t.Errorf("first dump = %q, want markdown", first.Dump)
	}
	if first.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("proxy = %q", first.Proxy)
	}
	if first.UserAgent == "" {
		t.Errorf("expected mobile user agent")
	}
	if !first.Stealth {
		t.Errorf("expected stealth=true for blockAds")
	}
	if first.Eval == "" {
		t.Errorf("expected eval from actions")
	}
}

func TestCreateHandler_RejectInvalidURL(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	fetcher := &fakeFetcher{body: []byte("# Hello")}
	runner := NewRunner(store, cfg, fetcher)
	create := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{"urls":["http://localhost:8080"],"ignoreInvalidURLs":false}`))
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

func withChiParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
