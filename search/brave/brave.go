package brave

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"nexora-crawl/search"
)

const (
	engineName    = "brave"
	baseURL       = "https://search.brave.com"
	searchPath    = "/search"
	defaultLocale = "en-us"
	scCodeTTL     = 60 * time.Minute
)

var timeRangeMap = map[string]string{
	"day":   "pd",
	"week":  "pw",
	"month": "pm",
	"year":  "py",
}

var safeSearchMap = map[int]string{
	0: "off",
	1: "moderate",
	2: "strict",
}

// Engine searches Brave's web category.
type Engine struct {
	client *search.HTTPClient
}

// New creates a Brave search engine.
func New(client *search.HTTPClient) *Engine {
	return &Engine{client: client}
}

// Name returns the engine identifier.
func (e *Engine) Name() string { return engineName }

// Search queries Brave and returns normalized web results.
func (e *Engine) Search(ctx context.Context, opts search.Options) ([]search.Result, error) {
	opts.Page = search.ClampPage(opts.Page)
	opts.Locale = search.NormalizeLocale(opts.Locale)

	u, headers, err := e.buildURL(opts)
	if err != nil {
		return nil, err
	}

	// Prime the cookie jar with anti-bot cookies.
	base, _ := url.Parse(baseURL)
	e.client.SetCookies(base, e.cookies(opts))

	resp, err := e.client.Get(ctx, u, headers)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", search.ErrBlocked, err)
	}
	body, err := search.ReadBody(resp)
	if err != nil {
		return nil, err
	}

	if err := detectCaptcha(body); err != nil {
		return nil, err
	}

	slog.Debug("brave search body summary", "status", resp.StatusCode, "len", len(body), "summary", search.SummaryBody(body))

	results, err := parseResults(body)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, search.ErrNoResults
	}

	limit := search.ClampLimit(opts.Limit)
	if limit > len(results) {
		limit = len(results)
	}
	return results[:limit], nil
}

func (e *Engine) buildURL(opts search.Options) (string, http.Header, error) {
	headers := make(http.Header)
	headers.Set("Referer", baseURL+"/")
	headers.Set("Accept-Encoding", "gzip, deflate")

	args := url.Values{}
	args.Set("q", opts.Query)
	args.Set("source", "web")
	args.Set("spellcheck", "0")

	if opts.Page > 1 {
		args.Set("offset", strconvItoa(opts.Page-1))
	}
	if opts.TimeRange != "" {
		if code, ok := timeRangeMap[opts.TimeRange]; ok {
			args.Set("tf", code)
		}
	}

	u := baseURL + searchPath + "?" + args.Encode()
	return u, headers, nil
}

func (e *Engine) cookies(opts search.Options) []*http.Cookie {
	region := e.mapRegion(opts.Locale)
	safe := search.MapSafeSearch(opts.SafeSearch, safeSearchMap)
	return []*http.Cookie{
		{Name: "safesearch", Value: safe},
		{Name: "useLocation", Value: "0"},
		{Name: "summarizer", Value: "0"},
		{Name: "country", Value: strings.Split(region, "-")[1]},
		{Name: "ui_lang", Value: region},
	}
}

func (e *Engine) mapRegion(locale string) string {
	// Brave UI locales are limited. Return a "lang-COUNTRY" tag.
	mappings := map[string]string{
		"en-us": "en-us",
		"en-gb": "en-gb",
		"en-ca": "en-ca",
		"id-id": "en-us", // Brave has limited locale support
		"fr-fr": "fr-fr",
		"de-de": "de-de",
		"es-es": "es-es",
		"it-it": "it-it",
		"ja-jp": "ja-jp",
		"pt-br": "pt-br",
		"ru-ru": "en-us",
		"zh-cn": "en-us",
		"nl-nl": "nl-nl",
	}
	if v, ok := mappings[locale]; ok {
		return v
	}
	return defaultLocale
}

func detectCaptcha(body string) error {
	lower := strings.ToLower(body)
	if strings.Contains(lower, "verify you are human") ||
		strings.Contains(lower, "captcha") && strings.Contains(lower, "brave") {
		return search.ErrCaptcha
	}
	return nil
}

func parseResults(body string) ([]search.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []search.Result
	// Brave wraps each web result in <div class="snippet ..." data-type="web">.
	doc.Find(`div[class*="snippet"][data-type="web"]`).Each(func(_ int, s *goquery.Selection) {
		// Find the first real outbound link inside the snippet.
		var href, title string
		s.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			if href != "" {
				return
			}
			h, ok := a.Attr("href")
			if !ok {
				return
			}
			h = strings.TrimSpace(h)
			if !strings.HasPrefix(h, "http") {
				return
			}
			href = h
			title = strings.TrimSpace(a.Find(".search-snippet-title").Text())
			if title == "" {
				title = strings.TrimSpace(a.Find(".title").Text())
			}
			if title == "" {
				title = strings.TrimSpace(a.Text())
			}
		})
		if href == "" || title == "" {
			return
		}

		content := strings.TrimSpace(s.Find("div[class*="+"content"+"]").Text())
		if content == "" {
			// Try generic snippet content.
			content = strings.TrimSpace(s.Find(".snippet-content, .generic-snippet").Text())
		}

		results = append(results, search.Result{
			Title:       title,
			URL:         href,
			Description: content,
		})
	})

	return results, nil
}

func strconvItoa(n int) string {
	if n < 0 {
		return "0"
	}
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
