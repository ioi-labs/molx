package brave

import (
	"testing"
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

	if len(got) == 0 {
		t.Fatalf("expected results, got none")
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(got))
	}
	for _, r := range got {
		if r.Title == "" || r.URL == "" {
			t.Errorf("empty title or url in result: %+v", r)
		}
	}

}

func TestMapRegion(t *testing.T) {
	e := New(nil)
	cases := map[string]string{
		"en-us": "en-us",
		"en-gb": "en-gb",
		"id-id": "en-us",
		"xx-xx": "en-us",
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
