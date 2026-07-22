package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

// fakeFetcher is a stub Obscura fetcher for handler tests.
// ponytail: tiny in-file fake, no mock framework.
type fakeFetcher struct {
	body      []byte
	htmlBody  []byte
	err       error
	lastReq   models.FetchRequest
	callCount int
}

func (f *fakeFetcher) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	f.lastReq = req
	f.callCount++
	if req.Dump == "html" && len(f.htmlBody) > 0 {
		return f.htmlBody, f.err
	}
	return f.body, f.err
}

func TestFetchHandler_MissingURL(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &FetchHandler{Config: cfg, Client: &fakeFetcher{}}

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/fetch", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFetchHandler_InvalidURL(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &FetchHandler{Config: cfg, Client: &fakeFetcher{}}

	body := bytes.NewReader([]byte(`{"url":"http://localhost:8080"}`))
	req := httptest.NewRequest(http.MethodPost, "/fetch", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFetchHandler_HappyPath(t *testing.T) {
	fetcher := &fakeFetcher{body: []byte(`{"title":"hi"}`)}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &FetchHandler{Config: cfg, Client: fetcher}

	body := bytes.NewReader([]byte(`{"url":"https://example.com","timeout":5}`))
	req := httptest.NewRequest(http.MethodPost, "/fetch", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if fetcher.callCount != 1 {
		t.Errorf("fetcher calls = %d, want 1", fetcher.callCount)
	}
	if fetcher.lastReq.URL != "https://example.com" {
		t.Errorf("fetcher url = %q, want https://example.com", fetcher.lastReq.URL)
	}
	if fetcher.lastReq.Timeout != 5 {
		t.Errorf("fetcher timeout = %d, want 5", fetcher.lastReq.Timeout)
	}

	var resp models.FetchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected ok=true")
	}
	if resp.URL != "https://example.com" {
		t.Errorf("response url = %q", resp.URL)
	}
}

func TestFetchHandler_ErrorPath(t *testing.T) {
	fetcher := &fakeFetcher{err: context.DeadlineExceeded}
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &FetchHandler{Config: cfg, Client: fetcher}

	body := bytes.NewReader([]byte(`{"url":"https://example.com"}`))
	req := httptest.NewRequest(http.MethodPost, "/fetch", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp models.FetchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.OK {
		t.Errorf("expected ok=false")
	}
	if resp.Error == "" {
		t.Errorf("expected error message")
	}
}
