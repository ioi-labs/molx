package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

func TestNormalizeFormats(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, []string{"markdown"}},
		{[]string{"markdown"}, []string{"markdown"}},
		{[]string{"html", "text"}, []string{"html", "text"}},
		{[]string{"HTML", "LINKS", "invalid"}, []string{"html", "links"}},
		{[]string{"invalid"}, []string{"markdown"}},
	}
	for _, c := range cases {
		got := normalizeFormats(c.in)
		if len(got) != len(c.want) {
			t.Errorf("normalizeFormats(%v) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("normalizeFormats(%v) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestBuildActionsEval(t *testing.T) {
	actions := []models.V2ScrapeAction{
		{Type: "wait", Milliseconds: 500},
		{Type: "click", Selector: "#btn"},
	}
	got := buildActionsEval(actions)
	if !bytes.Contains([]byte(got), []byte("await new Promise")) {
		t.Errorf("expected wait promise, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("#btn")) {
		t.Errorf("expected click selector, got %q", got)
	}
}

func TestV2ScrapeHandler_BadRequest(t *testing.T) {
	cfg := &config.Config{Timeout: 60 * time.Second}
	h := &V2ScrapeHandler{Config: cfg, Client: nil}
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/v2/scrape", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp["ok"] != false {
		t.Errorf("expected ok=false for bad request")
	}
}
