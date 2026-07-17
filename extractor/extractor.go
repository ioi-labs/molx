package extractor

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"nexora-crawl/models"
)

// ponytail: helper to pick first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ponytail: resolve a possibly-relative URL against the base.
func resolveURL(base *url.URL, href string) string {
	if base == nil || href == "" {
		return href
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// markdownLinkRe matches Markdown image/link URLs: [](url) or ![alt](url).
var markdownLinkRe = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)]+)\)`)

// ponytail: resolve relative URLs inside Markdown image/link syntax.
func ResolveMarkdownURLs(md, sourceURL string) string {
	base, err := url.Parse(sourceURL)
	if err != nil {
		return md
	}
	return markdownLinkRe.ReplaceAllStringFunc(md, func(match string) string {
		parts := markdownLinkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		urlPart := parts[2]
		resolved := resolveURL(base, urlPart)
		if resolved == urlPart {
			return match
		}
		prefix := ""
		if strings.HasPrefix(match, "!") {
			prefix = "!"
		}
		return prefix + "[" + parts[1] + "](" + resolved + ")"
	})
}

// CompressMarkdownWhitespace removes excessive blank lines, leading indentation,
// and trailing spaces while preserving paragraph/list breaks.
// ponytail: heuristic post-process; replace with a proper md formatter if it
// breaks tables, nested lists, or code blocks on a specific page.
func CompressMarkdownWhitespace(md string) string {
	// Step 1: collapse runs of blank lines, trim leading indentation and trailing spaces.
	lines := strings.Split(md, "\n")
	var out []string
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isBlank := trimmed == ""
		if isBlank {
			if prevBlank {
				continue
			}
			prevBlank = true
			out = append(out, "")
			continue
		}
		prevBlank = false
		// ponytail: keep list indentation minimal. Lines starting with - or * keep a single leading space if originally indented.
		if strings.HasPrefix(line, "  -") || strings.HasPrefix(line, "  *") || strings.HasPrefix(line, "\t-") || strings.HasPrefix(line, "\t*") {
			out = append(out, " "+trimmed)
			continue
		}
		out = append(out, trimmed)
	}
	// Drop leading/trailing blank lines.
	start := 0
	for start < len(out) && out[start] == "" {
		start++
	}
	end := len(out)
	for end > start && out[end-1] == "" {
		end--
	}
	md = strings.Join(out[start:end], "\n")

	// Step 2: join short label/value pairs emitted by Obscura as separate lines.
	md = compactLabelValuePairs(md)
	return md
}

// compactLabelValuePairs joins "Label\n\nValue" when both are short plain tokens.
// ponytail: heuristic for DOM-to-md; disable if it hurts readability on a page.
func compactLabelValuePairs(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			out = append(out, line)
			i++
			continue
		}
		// Find the next non-blank line to pair with this one.
		nextIdx := i + 1
		for nextIdx < len(lines) && lines[nextIdx] == "" {
			nextIdx++
		}
		// Only join when current line is a single-word label and the next is a
		// short numeric-looking value. This avoids collapsing headings/paragraphs.
		if nextIdx < len(lines) && isCompactableLabel(line) && isCompactableValue(lines[nextIdx]) {
			joined := line + " " + lines[nextIdx]
			if len(joined) <= 40 {
				out = append(out, joined)
				// Preserve one blank line after the value if another block follows.
				blockEnd := nextIdx + 1
				for blockEnd < len(lines) && lines[blockEnd] == "" {
					blockEnd++
				}
				if blockEnd < len(lines) {
					out = append(out, "")
				}
				i = blockEnd
				continue
			}
		}
		out = append(out, line)
		i++
	}
	// Re-collapse any double blanks created by skipped empty lines.
	return collapseBlankLines(strings.Join(out, "\n"))
}

func isCompactableLabel(s string) bool {
	if s == "" || len(s) > 15 {
		return false
	}
	if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, ">") {
		return false
	}
	if strings.ContainsAny(s, "[]()!#*-_`\"<>") || strings.ContainsRune(s, '.') {
		return false
	}
	// Labels are typically capitalized single words or short acronyms.
	if strings.Contains(s, " ") {
		return false
	}
	return true
}

func isCompactableValue(s string) bool {
	if s == "" || len(s) > 15 {
		return false
	}
	if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, ">") {
		return false
	}
	if strings.ContainsAny(s, "[]()!#*-_`\"<>") || strings.ContainsRune(s, '.') {
		return false
	}
	// Values must contain a digit, a percent, or a currency symbol to avoid joining sentences.
	if !strings.ContainsAny(s, "0123456789%$€£¥") {
		return false
	}
	return true
}

func collapseBlankLines(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	prevBlank := false
	for _, line := range lines {
		isBlank := line == ""
		if isBlank {
			if !prevBlank {
				out = append(out, "")
			}
			prevBlank = true
			continue
		}
		prevBlank = false
		out = append(out, line)
	}
	start := 0
	for start < len(out) && out[start] == "" {
		start++
	}
	end := len(out)
	for end > start && out[end-1] == "" {
		end--
	}
	return strings.Join(out[start:end], "\n")
}

// ApplyFilters removes excludeTags selectors from the document.
// It accepts either HTML or Markdown; Markdown is treated as a raw fragment.
func ApplyFilters(input string, excludeTags []string) (string, error) {
	if len(excludeTags) == 0 {
		return input, nil
	}
	// ponytail: markdown from obscura is a flat fragment, wrap it so goquery can parse it.
	html := input
	if !strings.Contains(html, "</html>") && !strings.Contains(html, "</body>") {
		html = "<body>" + html + "</body>"
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return input, err
	}
	selector := strings.Join(excludeTags, ", ")
	doc.Find(selector).Remove()
	out, err := doc.Html()
	if err != nil {
		return input, err
	}
	// ponytail: strip the wrapper we added so callers still get markdown.
	if strings.HasPrefix(out, "<html><head></head><body>") {
		out = strings.TrimPrefix(out, "<html><head></head><body>")
		out = strings.TrimSuffix(out, "</body></html>")
	}
	return out, nil
}

// ExtractMetadata pulls page-level metadata from the rendered HTML.
func ExtractMetadata(html, sourceURL string, statusCode int, contentType string) models.V2ScrapeMetadata {
	m := models.V2ScrapeMetadata{
		SourceURL:   sourceURL,
		URL:         sourceURL,
		StatusCode:  statusCode,
		ContentType: contentType,
	}
	base, _ := url.Parse(sourceURL)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return m
	}

	m.Title = strings.TrimSpace(doc.Find("title").First().Text())
	if lang, exists := doc.Find("html").Attr("lang"); exists {
		m.Language = lang
	}
	m.Viewport = metaContent(doc, "viewport", "")
	m.Description = metaContent(doc, "description", "og:description")
	m.Keywords = metaContent(doc, "keywords", "")
	m.Robots = metaContent(doc, "robots", "")
	m.Author = metaContent(doc, "author", "")
	m.GoogleSiteVerification = metaContent(doc, "google-site-verification", "")

	favicon, _ := doc.Find("link[rel='icon']").Attr("href")
	if favicon == "" {
		favicon, _ = doc.Find("link[rel='shortcut icon']").Attr("href")
	}
	if favicon == "" {
		favicon, _ = doc.Find("link[rel='apple-touch-icon']").Attr("href")
	}
	m.Favicon = resolveURL(base, favicon)

	m.OgTitle = metaContent(doc, "", "og:title")
	m.OgDescription = metaContent(doc, "", "og:description")
	m.OgDescriptionAlt = m.OgDescription
	m.OgType = metaContent(doc, "", "og:type")
	m.OgURL = metaContent(doc, "", "og:url")
	m.OgURLAlt = m.OgURL
	m.OgImage = resolveURL(base, metaContent(doc, "", "og:image"))
	m.OgImageURL = m.OgImage
	m.OgSiteName = metaContent(doc, "", "og:site_name")
	m.OgSiteNameAlt = m.OgSiteName
	m.OgLocale = metaContent(doc, "", "og:locale")

	m.TwitterCard = metaContent(doc, "twitter:card", "")
	m.TwitterTitle = metaContent(doc, "twitter:title", "")
	m.TwitterDescription = metaContent(doc, "twitter:description", "")
	m.TwitterImage = resolveURL(base, metaContent(doc, "twitter:image", ""))

	canonical, _ := doc.Find("link[rel='canonical']").Attr("href")
	m.Canonical = resolveURL(base, canonical)

	return m
}

func metaContent(doc *goquery.Document, name, prop string) string {
	if name != "" {
		if v, ok := doc.Find("meta[name='" + name + "']").Attr("content"); ok {
			return v
		}
	}
	if prop != "" {
		if v, ok := doc.Find("meta[property='" + prop + "']").Attr("content"); ok {
			return v
		}
	}
	return ""
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
