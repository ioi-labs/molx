package startpage

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
	if len(got) < 5 {
		t.Fatalf("expected at least 5 results, got %d", len(got))
	}
	for _, r := range got {
		if r.Title == "" || r.URL == "" {
			t.Errorf("empty title or url in result: %+v", r)
		}
	}
}

func TestMapLanguage(t *testing.T) {
	e := New(nil, nil)
	cases := map[string]string{
		"en-us": "english",
		"en-gb": "english_uk",
		"id-id": "indonesian",
		"fr-fr": "francais",
		"xx-xx": "english",
	}
	for in, want := range cases {
		if got := e.mapLanguage(in); got != want {
			t.Errorf("mapLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}
