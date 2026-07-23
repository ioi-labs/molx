package duckduckgo

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"molx/search"
)

const (
	engineName     = "duckduckgo"
	baseURL        = "https://html.duckduckgo.com/html/"
	defaultLocale  = "wt-wt"
	maxQueryLength = 499
	vqdTTL         = 60 * time.Minute
)

var timeRangeMap = map[string]string{
	"day":   "d",
	"week":  "w",
	"month": "m",
	"year":  "y",
}

// Engine searches DuckDuckGo using the no-JS HTML endpoint.
type Engine struct {
	cache  *search.SharedCache
	client *search.HTTPClient
}

// New creates a DuckDuckGo search engine.
func New(cache *search.SharedCache, client *search.HTTPClient) *Engine {
	return &Engine{cache: cache, client: client}
}

// Name returns the engine identifier.
func (e *Engine) Name() string { return engineName }

// Search queries DuckDuckGo and returns normalized results.
func (e *Engine) Search(ctx context.Context, opts search.Options) ([]search.Result, error) {
	if len(opts.Query) > maxQueryLength {
		return nil, fmt.Errorf("%w: query exceeds %d characters", search.ErrInvalid, maxQueryLength)
	}

	opts.Page = search.ClampPage(opts.Page)
	opts.Locale = search.NormalizeLocale(opts.Locale)

	form, headers, err := e.buildRequest(ctx, opts)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Post(ctx, baseURL, search.FormEncode(form), headers)
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

	// Persist vqd for follow-up pages.
	if opts.Page == 1 {
		if vqd := extractVQD(body); vqd != "" {
			e.cacheVQD(opts.Query, vqd)
		}
	}

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

func (e *Engine) buildRequest(_ context.Context, opts search.Options) (map[string]string, http.Header, error) {
	headers := make(http.Header)
	headers.Set("Referer", baseURL)
	headers.Set("Origin", "https://html.duckduckgo.com")
	headers.Set("Sec-Fetch-Site", "same-origin")

	locale := e.mapRegion(opts.Locale)
	form := map[string]string{
		"q":  opts.Query,
		"kl": locale,
	}

	if opts.Page == 1 {
		form["b"] = ""
	} else {
		vqd, ok := e.getVQD(opts.Query)
		if !ok || vqd == "" {
			// Without a vqd we must not hit DDG or the IP will be flagged.
			return nil, nil, fmt.Errorf("%w: vqd missing for page %d", search.ErrBlocked, opts.Page)
		}
		form["vqd"] = vqd
		form["nextParams"] = ""
		form["api"] = "d.js"
		form["o"] = "json"
		form["v"] = "l"

		offset := 10 + (opts.Page-2)*15
		form["dc"] = strconv.Itoa(offset + 1)
		form["s"] = strconv.Itoa(offset)
	}

	if locale == defaultLocale {
		form["kl"] = defaultLocale
	}

	if opts.TimeRange != "" {
		if code, ok := timeRangeMap[opts.TimeRange]; ok {
			form["df"] = code
		}
	}

	switch opts.SafeSearch {
	case 2:
		form["kp"] = "1"
	case 1:
		form["kp"] = "-1"
	default:
		form["kp"] = "-2"
	}

	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return form, headers, nil
}

// mapRegion converts a normalized locale (e.g. en-us) to a DDG region code.
func (e *Engine) mapRegion(locale string) string {
	mappings := map[string]string{
		"en-us": "us-en",
		"en-gb": "uk-en",
		"en-ca": "ca-en",
		"id-id": "id-en",
		"fr-fr": "fr-fr",
		"de-de": "de-de",
		"es-es": "es-es",
		"it-it": "it-it",
		"ja-jp": "jp-jp",
		"ko-kr": "kr-kr",
		"pt-br": "br-pt",
		"ru-ru": "ru-ru",
		"zh-cn": "cn-zh",
		"zh-tw": "tw-tzh",
		"nl-nl": "nl-nl",
		"pl-pl": "pl-pl",
		"tr-tr": "tr-tr",
	}
	if v, ok := mappings[locale]; ok {
		return v
	}
	return defaultLocale
}

func (e *Engine) cacheVQD(query, vqd string) {
	key := search.SecretHash(query, search.DefaultUserAgent)
	e.cache.Set(engineName, "vqd::"+key, vqd, vqdTTL)
}

func (e *Engine) getVQD(query string) (string, bool) {
	key := search.SecretHash(query, search.DefaultUserAgent)
	return e.cache.Get(engineName, "vqd::"+key)
}

func detectCaptcha(body string) error {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return err
	}
	if doc.Find("form#challenge-form").Length() > 0 {
		return search.ErrCaptcha
	}
	if strings.Contains(strings.ToLower(body), "your ip address is") &&
		strings.Contains(strings.ToLower(body), "your user agent") {
		return search.ErrBlocked
	}
	return nil
}

func extractVQD(body string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return ""
	}
	if v, ok := doc.Find("input[name='vqd']").Attr("value"); ok {
		return v
	}
	return ""
}

func parseResults(body string) ([]search.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []search.Result
	doc.Find(`#links div.web-result`).Each(func(_ int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("h2 a").Text())
		href, ok := s.Find("h2 a").Attr("href")
		if !ok || title == "" {
			return
		}
		content := strings.TrimSpace(s.Find("a.result__snippet").Text())

		u := strings.TrimSpace(href)
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		if _, err := search.ParseProxy(u); err != nil {
			return
		}

		results = append(results, search.Result{
			Title:       title,
			URL:         u,
			Description: content,
		})
	})

	return results, nil
}
