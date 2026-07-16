package handlers

import (
	"testing"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

func TestMapResultsLimit(t *testing.T) {
	h := &SearchHandler{Config: &config.Config{}}
	searx := make([]models.SearXNGResult, 20)
	for i := 0; i < 20; i++ {
		searx[i] = models.SearXNGResult{URL: "https://example.com/" + string(rune('a'+i))}
	}

	cases := []struct {
		limit int
		want  int
	}{
		{0, defaultSearchLimit},
		{5, 5},
		{50, 20},
		{200, defaultSearchLimit},
	}

	for _, c := range cases {
		got := h.mapResults(searx, models.SearchRequest{Limit: c.limit})
		if len(got) != c.want {
			t.Errorf("limit=%d: got %d results, want %d", c.limit, len(got), c.want)
		}
	}
}

func TestMapResultsTrim(t *testing.T) {
	h := &SearchHandler{Config: &config.Config{}}
	searx := []models.SearXNGResult{{
		Title:   "  Title  ",
		URL:     "https://example.com",
		Content: "  snippet  ",
	}}
	got := h.mapResults(searx, models.SearchRequest{})
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Title != "Title" || got[0].Description != "snippet" {
		t.Errorf("trim failed: title=%q description=%q", got[0].Title, got[0].Description)
	}
}
