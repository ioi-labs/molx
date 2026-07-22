package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

// fakeFetcher is a stub Obscura fetcher for handler tests.
type fakeFetcher struct {
	body     []byte
	htmlBody []byte
	err      error
	reqs     []models.FetchRequest
	calls    int
}

func (f *fakeFetcher) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	f.reqs = append(f.reqs, req)
	f.calls++
	if req.Dump == "html" && len(f.htmlBody) > 0 {
		return f.htmlBody, f.err
	}
	return f.body, f.err
}

func (f *fakeFetcher) firstReq() models.FetchRequest {
	if len(f.reqs) == 0 {
		return models.FetchRequest{}
	}
	return f.reqs[0]
}

func (f *fakeFetcher) lastReq() models.FetchRequest {
	if len(f.reqs) == 0 {
		return models.FetchRequest{}
	}
	return f.reqs[len(f.reqs)-1]
}

func newTestHandler(t *testing.T) *V2ScrapeHandler {
	t.Helper()
	cfg := &config.Config{Timeout: 60 * time.Second}
	fetcher := &fakeFetcher{
		body:     []byte("# Hello\n\n[link](https://example.com/page)"),
		htmlBody: []byte("<html><body><h1>Hello</h1><a href='/page'>page</a></body></html>"),
	}
	sc := NewV2(cfg, fetcher)
	return &V2ScrapeHandler{Config: cfg, Client: fetcher, Scraper: sc}
}

func TestV2ScrapeHandler_MissingURL(t *testing.T) {
	h := newTestHandler(t)
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestV2ScrapeHandler_InvalidURL(t *testing.T) {
	h := newTestHandler(t)
	body := bytes.NewReader([]byte(`{"url":"http://localhost:8080"}`))
	req := httptest.NewRequest(http.MethodPost, "/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestV2ScrapeHandler_HappyPath(t *testing.T) {
	h := newTestHandler(t)
	body := bytes.NewReader([]byte(`{"url":"https://example.com"}`))
	req := httptest.NewRequest(http.MethodPost, "/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp models.V2ScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true")
	}
	if resp.Data == nil || resp.Data.Markdown == "" {
		t.Errorf("expected markdown data")
	}
	if resp.Data.Metadata.SourceURL != "https://example.com" {
		t.Errorf("sourceURL = %q", resp.Data.Metadata.SourceURL)
	}
}

func TestV2ScrapeHandler_Params(t *testing.T) {
	h := newTestHandler(t)
	body := bytes.NewReader([]byte(`{"urls":["https://example.com"],"formats":["markdown","html","text","links"],"onlyMainContent":true,"excludeTags":["aside"],"waitFor":1500,"timeout":10000,"mobile":true,"proxy":"http://proxy.example.com:8080","blockAds":true,"actions":[{"type":"wait","milliseconds":500},{"type":"click","selector":"#btn"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	fetcher := h.Client.(*fakeFetcher)
	first := fetcher.firstReq()
	if first.Dump != "markdown" {
		t.Errorf("first dump = %q, want markdown", first.Dump)
	}
	if first.Wait != 1 {
		t.Errorf("wait = %d, want 1 (1500ms rounded to 1s)", first.Wait)
	}
	if first.Timeout != 10 {
		t.Errorf("timeout = %d, want 10 (10000ms / 1000)", first.Timeout)
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

	var resp models.V2ScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Data.Markdown == "" {
		t.Errorf("expected markdown")
	}
	if resp.Data.HTML == "" {
		t.Errorf("expected html")
	}
	if resp.Data.Text == "" {
		t.Errorf("expected text")
	}
	if len(resp.Data.Links) == 0 {
		t.Errorf("expected links")
	}
}

func TestV2ScrapeHandler_IncludeTags(t *testing.T) {
	h := newTestHandler(t)
	body := bytes.NewReader([]byte(`{"url":"https://example.com","formats":["html","text","links"],"includeTags":["h1","a"]}`))
	req := httptest.NewRequest(http.MethodPost, "/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp models.V2ScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !strings.Contains(resp.Data.HTML, "<h1>") {
		t.Errorf("expected h1 in html, got %q", resp.Data.HTML)
	}
	if !strings.Contains(resp.Data.HTML, "<a ") {
		t.Errorf("expected link in html, got %q", resp.Data.HTML)
	}
	if strings.Contains(resp.Data.HTML, "<body>") {
		t.Errorf("expected wrapper stripped, got %q", resp.Data.HTML)
	}
	if len(resp.Data.Links) == 0 {
		t.Errorf("expected links from selected anchor")
	}
	if !strings.Contains(resp.Data.Text, "Hello") {
		t.Errorf("expected Hello in text, got %q", resp.Data.Text)
	}
}

func TestV2ScrapeHandler_ErrorPath(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	fetcher := &fakeFetcher{err: context.DeadlineExceeded}
	sc := NewV2(cfg, fetcher)
	h := &V2ScrapeHandler{Config: cfg, Client: fetcher, Scraper: sc}

	body := bytes.NewReader([]byte(`{"url":"https://example.com"}`))
	req := httptest.NewRequest(http.MethodPost, "/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp models.V2ScrapeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Success {
		t.Errorf("expected success=false")
	}
	if resp.Error == "" {
		t.Errorf("expected error message")
	}
}
