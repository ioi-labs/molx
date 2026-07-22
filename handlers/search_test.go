package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/models"
	"nexora-crawl/scraper"
	"nexora-crawl/search"
)

// fakeEngine is a stub search engine for handler tests.
// ponytail: in-file stub, records options so parameter wiring is verified.
type fakeEngine struct {
	name    string
	results []search.Result
	lastOpts search.Options
}

func (e *fakeEngine) Name() string { return e.name }
func (e *fakeEngine) Search(ctx context.Context, opts search.Options) ([]search.Result, error) {
	e.lastOpts = opts
	return e.results, nil
}

func newFakeScraper(t *testing.T) *scraper.V2Scraper {
	t.Helper()
	cfg := &config.Config{Timeout: 60 * time.Second}
	fetcher := &fakeFetcher{
		body:     []byte("# Hello\n\n[link](https://example.com/page)"),
		htmlBody: []byte("<html><body><h1>Hello</h1><a href='/page'>page</a></body></html>"),
	}
	return scraper.NewV2(cfg, fetcher)
}

func TestV2SearchHandler_MissingQuery(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &V2SearchHandler{Config: cfg, Engines: []search.Engine{&fakeEngine{name: "fake"}}}

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/search", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestV2SearchHandler_NoEngines(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &V2SearchHandler{Config: cfg, Engines: nil}

	body := bytes.NewReader([]byte(`{"query":"golang"}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/search", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestV2SearchHandler_HappyPath(t *testing.T) {
	eng := &fakeEngine{
		name: "fake",
		results: []search.Result{
			{Title: "Go", URL: "https://go.dev", Description: "go site"},
			{Title: "Example", URL: "https://example.com", Description: "example"},
		},
	}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &V2SearchHandler{Config: cfg, Engines: []search.Engine{eng}, Scraper: newFakeScraper(t)}

	body := bytes.NewReader([]byte(`{"query":"golang","limit":2,"language":"en-us","time_range":"week","safesearch":1}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/search", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if eng.lastOpts.Query != "golang" {
		t.Errorf("query = %q, want golang", eng.lastOpts.Query)
	}
	if eng.lastOpts.Locale != "en-us" {
		t.Errorf("locale = %q, want en-us", eng.lastOpts.Locale)
	}
	if eng.lastOpts.TimeRange != "week" {
		t.Errorf("time_range = %q, want week", eng.lastOpts.TimeRange)
	}
	if eng.lastOpts.SafeSearch != 1 {
		t.Errorf("safesearch = %d, want 1", eng.lastOpts.SafeSearch)
	}
	if eng.lastOpts.Limit != 2 {
		t.Errorf("limit = %d, want 2", eng.lastOpts.Limit)
	}

	var resp models.V2SearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true")
	}
	if len(resp.Data) != 2 {
		t.Errorf("results = %d, want 2", len(resp.Data))
	}
}

func TestV2SearchHandler_ScrapeOptions(t *testing.T) {
	eng := &fakeEngine{
		name: "fake",
		results: []search.Result{
			{Title: "Example", URL: "https://example.com", Description: "example"},
		},
	}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &V2SearchHandler{Config: cfg, Engines: []search.Engine{eng}, Scraper: newFakeScraper(t)}

	body := bytes.NewReader([]byte(`{"query":"test","scrapeOptions":{"formats":["markdown","text","links"],"onlyMainContent":true,"excludeTags":["aside"],"waitFor":1000,"timeout":5000,"mobile":true,"proxy":"http://proxy.example.com:8080","blockAds":true}}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/search", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp models.V2SearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("results = %d, want 1", len(resp.Data))
	}
	item := resp.Data[0]
	if item.Markdown == "" {
		t.Errorf("expected markdown populated")
	}
	if item.Text == "" {
		t.Errorf("expected text populated")
	}
	if len(item.Links) == 0 {
		t.Errorf("expected links populated")
	}
}

func TestLegacySearchHandler_MissingQuery(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &LegacySearchHandler{Config: cfg, Engines: []search.Engine{&fakeEngine{name: "fake"}}}

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestLegacySearchHandler_GETParams(t *testing.T) {
	eng := &fakeEngine{
		name: "fake",
		results: []search.Result{
			{Title: "Go", URL: "https://go.dev", Description: "go site"},
		},
	}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &LegacySearchHandler{Config: cfg, Engines: []search.Engine{eng}}

	q := url.Values{}
	q.Set("q", "golang")
	q.Set("language", "en-us")
	q.Set("time_range", "month")
	q.Set("safesearch", "2")
	q.Set("pageno", "3")
	q.Set("limit", "5")
	req := httptest.NewRequest(http.MethodGet, "/search?"+q.Encode(), nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if eng.lastOpts.Query != "golang" {
		t.Errorf("query = %q", eng.lastOpts.Query)
	}
	if eng.lastOpts.Locale != "en-us" {
		t.Errorf("locale = %q", eng.lastOpts.Locale)
	}
	if eng.lastOpts.TimeRange != "month" {
		t.Errorf("time_range = %q", eng.lastOpts.TimeRange)
	}
	if eng.lastOpts.SafeSearch != 2 {
		t.Errorf("safesearch = %d", eng.lastOpts.SafeSearch)
	}
	if eng.lastOpts.Page != 3 {
		t.Errorf("page = %d, want 3", eng.lastOpts.Page)
	}
	if eng.lastOpts.Limit != 5 {
		t.Errorf("limit = %d, want 5", eng.lastOpts.Limit)
	}

	var resp models.SearXNGResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("results = %d, want 1", len(resp.Results))
	}
}

func TestLegacySearchHandler_POSTForm(t *testing.T) {
	eng := &fakeEngine{
		name: "fake",
		results: []search.Result{
			{Title: "Go", URL: "https://go.dev", Description: "go site"},
		},
	}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &LegacySearchHandler{Config: cfg, Engines: []search.Engine{eng}}

	form := url.Values{}
	form.Set("q", "golang post")
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if eng.lastOpts.Query != "golang post" {
		t.Errorf("query = %q, want 'golang post'", eng.lastOpts.Query)
	}
}
