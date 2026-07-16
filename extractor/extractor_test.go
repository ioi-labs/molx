package extractor

import (
	"strings"
	"testing"
)

func TestExtractMetadata(t *testing.T) {
	html := `<!doctype html>
<html lang="en">
  <head>
    <title>Example Domain</title>
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" href="/favicon.ico" />
  </head>
  <body><h1>Hello</h1></body>
</html>`

	m := ExtractMetadata(html, "https://example.com")
	if m.Title != "Example Domain" {
		t.Errorf("title = %q, want %q", m.Title, "Example Domain")
	}
	if m.Language != "en" {
		t.Errorf("language = %q, want %q", m.Language, "en")
	}
	if m.Viewport != "width=device-width, initial-scale=1" {
		t.Errorf("viewport = %q", m.Viewport)
	}
	if m.Favicon != "/favicon.ico" {
		t.Errorf("favicon = %q", m.Favicon)
	}
	if m.SourceURL != "https://example.com" {
		t.Errorf("sourceURL = %q", m.SourceURL)
	}
}

func TestExtractText(t *testing.T) {
	html := `<html><body>
  <script>alert('x')</script>
  <p>Hello World</p>
</body></html>`
	text := ExtractText(html)
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected Hello World, got %q", text)
	}
	if strings.Contains(text, "alert") {
		t.Errorf("script content should be removed, got %q", text)
	}
}

func TestExtractLinks(t *testing.T) {
	html := `<html><body>
  <a href="/about">About</a>
  <a href="https://external.com/page">External</a>
  <a href="#section">Skip</a>
  <a href="javascript: void(0)">JS</a>
</body></html>`

	links := ExtractLinks(html, "https://example.com")
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(links), links)
	}
	if links[0] != "https://example.com/about" {
		t.Errorf("first link = %q", links[0])
	}
	if links[1] != "https://external.com/page" {
		t.Errorf("second link = %q", links[1])
	}
}

func TestApplyFiltersExclude(t *testing.T) {
	html := `<html><body>
  <header>Header</header>
  <main>Main Content</main>
  <footer>Footer</footer>
</body></html>`

	out, err := ApplyFilters(html, []string{"header", "footer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Header") {
		t.Errorf("header should be removed")
	}
	if strings.Contains(out, "Footer") {
		t.Errorf("footer should be removed")
	}
	if !strings.Contains(out, "Main Content") {
		t.Errorf("main content should remain")
	}
}
