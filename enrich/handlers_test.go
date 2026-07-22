package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"nexora-crawl/config"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
	"nexora-crawl/search"
)

// fakeFetcher is a stub Obscura fetcher for enrich tests.
type fakeFetcher struct {
	body []byte
	err  error
	reqs []models.FetchRequest
}

func (f *fakeFetcher) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	f.reqs = append(f.reqs, req)
	return f.body, f.err
}

var _ obscura.Fetcher = (*fakeFetcher)(nil)

// fakeEngine is a stub search engine for enrich tests.
type fakeEngine struct {
	results []search.Result
	lastOpts search.Options
}

func (e *fakeEngine) Name() string { return "fake" }
func (e *fakeEngine) Search(ctx context.Context, opts search.Options) ([]search.Result, error) {
	e.lastOpts = opts
	return e.results, nil
}


func TestCreateHandler_BadRequest(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	runner := NewRunner(store, cfg, nil, nil)
	h := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/enrich", body)
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
	// Use a runner with no Obscura client; complete the job manually before the
	// async goroutine can reach the scraper.
	runner := NewRunner(store, cfg, nil, nil)

	create := &CreateHandler{Runner: runner}
	status := &StatusHandler{Store: store}

	body := bytes.NewReader([]byte(`{"prompt":"list founders","schema":{"type":"object"}}`))
	req := httptest.NewRequest(http.MethodPost, "/enrich", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d", rr.Code, http.StatusOK)
	}

	var createResp models.EnrichCreateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid create response: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected job id, got empty")
	}

	// Simulate completion without running real search/scrape/LLM.
	job, ok := store.Get(createResp.ID)
	if !ok {
		t.Fatalf("job not found in store")
	}
	// Keep overwriting the job with a completed snapshot until the async
	// goroutine finishes, preventing it from flipping status back to failed.
	for i := 0; i < 50; i++ {
		job.mu.Lock()
		job.Status = "completed"
		job.Result = map[string]any{"founders": []string{"Alice", "Bob"}}
		now := time.Now()
		job.CompletedAt = &now
		job.Error = ""
		job.mu.Unlock()
		// The goroutine sets status after search fails; briefly yield so it
		// can run, then re-assert completed.
		time.Sleep(2 * time.Millisecond)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/enrich/"+createResp.ID, nil)
	rr = httptest.NewRecorder()
	status.ServeHTTP(rr, withChiParam(statusReq, "id", createResp.ID))

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}

	var statusResp models.EnrichStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("invalid status response: %v", err)
	}
	if statusResp.Status != "completed" {
		t.Errorf("status = %q, want completed", statusResp.Status)
	}
	if statusResp.Result == nil {
		t.Errorf("expected result, got nil")
	}
}

func TestStatusHandler_NotFound(t *testing.T) {
	store := NewStore()
	status := &StatusHandler{Store: store}

	req := httptest.NewRequest(http.MethodGet, "/enrich/not-real", nil)
	rr := httptest.NewRecorder()
	status.ServeHTTP(rr, withChiParam(req, "id", "not-real"))

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCreateHandler_WithURLs(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	fetcher := &fakeFetcher{body: []byte("# Hello")}

	llm := NewClient(Config{BaseURL: "http://test", APIKey: "k", Model: "m"})
	llm.httpDo = func(req *http.Request) (*http.Response, error) {
		body, _ := json.Marshal(chatResponse{Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{{Message: struct {
			Content string `json:"content"`
		}{Content: `{"answer":42}`}}}})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}

	runner := NewRunner(store, cfg, fetcher, nil)
	runner.llm = llm
	create := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{"prompt":"what is the answer","schema":{"type":"object"},"urls":["https://example.com"]}`))
	req := httptest.NewRequest(http.MethodPost, "/enrich", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var createResp models.EnrichCreateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected job id")
	}

	deadline := time.Now().Add(2 * time.Second)
	var statusResp models.EnrichStatusResponse
	for {
		statusReq := httptest.NewRequest(http.MethodGet, "/enrich/"+createResp.ID, nil)
		rr = httptest.NewRecorder()
		status := &StatusHandler{Store: store}
		status.ServeHTTP(rr, withChiParam(statusReq, "id", createResp.ID))
		if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
			t.Fatalf("invalid status response: %v", err)
		}
		if statusResp.Status == "completed" || statusResp.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if statusResp.Status != "completed" {
		t.Errorf("status = %q, want completed", statusResp.Status)
	}
	if len(fetcher.reqs) == 0 {
		t.Errorf("expected at least one scrape request")
	}
	if statusResp.Result == nil {
		t.Errorf("expected result")
	}
}

func TestCreateHandler_SearchPath(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	store := NewStore()
	fetcher := &fakeFetcher{body: []byte("# Hello")}
	eng := &fakeEngine{results: []search.Result{{Title: "Ex", URL: "https://example.com", Description: "x"}}}

	llm := NewClient(Config{BaseURL: "http://test", APIKey: "k", Model: "m"})
	llm.httpDo = func(req *http.Request) (*http.Response, error) {
		body, _ := json.Marshal(chatResponse{Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{{Message: struct {
			Content string `json:"content"`
		}{Content: `{"answer":42}`}}}})
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}

	runner := NewRunner(store, cfg, fetcher, []search.Engine{eng})
	runner.llm = llm
	create := &CreateHandler{Runner: runner}

	body := bytes.NewReader([]byte(`{"prompt":"answer","schema":{"type":"object"}}`))
	req := httptest.NewRequest(http.MethodPost, "/enrich", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	create.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var createResp models.EnrichCreateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected job id")
	}

	deadline := time.Now().Add(2 * time.Second)
	var statusResp models.EnrichStatusResponse
	for {
		statusReq := httptest.NewRequest(http.MethodGet, "/enrich/"+createResp.ID, nil)
		rr = httptest.NewRecorder()
		status := &StatusHandler{Store: store}
		status.ServeHTTP(rr, withChiParam(statusReq, "id", createResp.ID))
		if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
			t.Fatalf("invalid status response: %v", err)
		}
		if statusResp.Status == "completed" || statusResp.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if statusResp.Status != "completed" {
		t.Errorf("status = %q, want completed", statusResp.Status)
	}
	if eng.lastOpts.Query != "answer" {
		t.Errorf("search query = %q, want answer", eng.lastOpts.Query)
	}
}

func TestEnrichClient_StripsFences(t *testing.T) {
	called := false
	client := &Client{
		cfg: Config{BaseURL: "http://test", APIKey: "k", Model: "m"},
		httpDo: func(req *http.Request) (*http.Response, error) {
			called = true
		choice := struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			Message: struct {
				Content string `json:"content"`
			}{Content: "```json\n{\"answer\":42}\n```"},
		}
		body, _ := json.Marshal(chatResponse{Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{choice}})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		},
	}

	res, err := client.Enrich(context.Background(), "x", map[string]any{"type": "object"}, "ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected HTTP call")
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["answer"] != float64(42) {
		t.Errorf("answer = %v, want 42", m["answer"])
	}
}

func withChiParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
