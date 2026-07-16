package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"nexora-crawl/models"
)

// ApplyFilters removes excludeTags selectors from the document.
func ApplyFilters(html string, excludeTags []string) (string, error) {
	if len(excludeTags) == 0 {
		return html, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html, err
	}
	selector := strings.Join(excludeTags, ", ")
	doc.Find(selector).Remove()
	return doc.Html()
}

// ExtractMetadata pulls page-level metadata from the rendered HTML.
func ExtractMetadata(html, sourceURL string) models.V2ScrapeMetadata {
	m := models.V2ScrapeMetadata{
		SourceURL: sourceURL,
		URL:       sourceURL,
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return m
	}

	m.Title = strings.TrimSpace(doc.Find("title").First().Text())
	if lang, exists := doc.Find("html").Attr("lang"); exists {
		m.Language = lang
	}
	m.Viewport, _ = doc.Find("meta[name='viewport']").Attr("content")

	favicon, _ := doc.Find("link[rel='icon']").Attr("href")
	if favicon == "" {
		favicon, _ = doc.Find("link[rel='shortcut icon']").Attr("href")
	}
	if favicon == "" {
		favicon, _ = doc.Find("link[rel='apple-touch-icon']").Attr("href")
	}
	m.Favicon = favicon

	return m
}

// ExtractText returns readable plain text from the HTML body.
func ExtractText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	doc.Find("script, style, noscript").Remove()
	return strings.TrimSpace(doc.Find("body").Text())
}

// ExtractLinks returns absolute http(s) links from anchor tags.
func ExtractLinks(html, baseURL string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	base, _ := url.Parse(baseURL)
	seen := make(map[string]struct{})
	var out []string
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			return
		}
		abs := href
		if base != nil && !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			if ref, err := url.Parse(href); err == nil {
				abs = base.ResolveReference(ref).String()
			}
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	})
	return out
}
