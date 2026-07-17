package duckduckgo

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"nexora-crawl/search"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := testdata.ReadFile(name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseResults(t *testing.T) {
	body := readFixture(t, "testdata/firstpage.html")
	got, err := parseResults(body)
	if err != nil {
		t.Fatalf("parseResults: %v", err)
	}

	want := []search.Result{
		{
			Title:       "Canceling in-progress operations - The Go Programming Language",
			URL:         "https://go.dev/doc/database/cancel-operations",
			Description: "You can manage in-progress operations by using Go context.Context.",
		},
		{
			Title:       "context package - context - Go Packages",
			URL:         "https://pkg.go.dev/context",
			Description: "The WithCancel, WithDeadline, and WithTimeout functions take a Context (the parent) and return a derived Context (the child) and a CancelFunc.",
		},
	}

	if len(got) == 0 {
		t.Fatalf("expected results, got none")
	}
	if diff := cmp.Diff(want, got[:min(len(want), len(got))]); diff != "" {
		t.Errorf("parseResults mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractVQD(t *testing.T) {
	body := readFixture(t, "testdata/firstpage.html")
	if v := extractVQD(body); v == "" {
		t.Errorf("expected non-empty vqd")
	}
}

func TestDetectCaptcha(t *testing.T) {
	if err := detectCaptcha("<form id='challenge-form'></form>"); err != search.ErrCaptcha {
		t.Errorf("expected ErrCaptcha, got %v", err)
	}
	if err := detectCaptcha("<div>normal results</div>"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMapRegion(t *testing.T) {
	e := New(nil, nil)
	cases := map[string]string{
		"en-us": "us-en",
		"en-gb": "uk-en",
		"id-id": "id-en",
		"xx-xx": "wt-wt",
	}
	for in, want := range cases {
		if got := e.mapRegion(in); got != want {
			t.Errorf("mapRegion(%q) = %q, want %q", in, got, want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
