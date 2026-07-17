package startpage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"nexora-crawl/search"
)

const (
	engineName      = "startpage"
	baseURL         = "https://www.startpage.com"
	searchURL       = baseURL + "/sp/search"
	defaultLocale   = "en-us"
	defaultLanguage = "english"
	scCodeTTL       = 60 * time.Minute
)

var timeRangeMap = map[string]string{
	"day":   "d",
	"week":  "w",
	"month": "m",
	"year":  "y",
}

var safeSearchMap = map[int]string{
	0: "none",
	1: "moderate",
	2: "heavy",
}

// Engine searches Startpage's web category.
type Engine struct {
	cache  *search.SharedCache
	client *search.HTTPClient
}

// New creates a Startpage search engine.
func New(cache *search.SharedCache, client *search.HTTPClient) *Engine {
	return &Engine{cache: cache, client: client}
}

// Name returns the engine identifier.
func (e *Engine) Name() string { return engineName }

// Search queries Startpage and returns normalized web results.
func (e *Engine) Search(ctx context.Context, opts search.Options) ([]search.Result, error) {
	opts.Page = search.ClampPage(opts.Page)
	opts.Locale = search.NormalizeLocale(opts.Locale)

	scCode, err := e.getSCCode(ctx)
	if err != nil {
		return nil, err
	}

	form, headers, err := e.buildRequest(opts, scCode)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.RawPost(ctx, searchURL, search.FormEncode(form), headers)
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

func (e *Engine) buildRequest(opts search.Options, scCode string) (map[string]string, http.Header, error) {
	headers := make(http.Header)
	headers.Set("Referer", baseURL+"/")
	headers.Set("Origin", baseURL)
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	headers.Set("Accept-Language", "en-US,en;q=0.5")
	headers.Set("User-Agent", search.DefaultUserAgent)
	headers.Set("Content-Type", "application/x-www-form-urlencoded")

	lang := e.mapLanguage(opts.Locale)
	safe := search.MapSafeSearch(opts.SafeSearch, safeSearchMap)

	args := map[string]string{
		"query":    opts.Query,
		"cat":      "web",
		"t":        "device",
		"sc":       scCode,
		"abd":      "1",
		"abe":      "1",
		"qsr":      "all",
		"qadf":     safe,
		"language": lang,
		"lui":      lang,
	}
	if opts.TimeRange != "" {
		args["with_date"] = timeRangeMap[opts.TimeRange]
	}

	if opts.Page > 1 {
		args["page"] = strconv.Itoa(opts.Page)
		args["segment"] = "startpage.udog"
	}

	return args, headers, nil
}

func (e *Engine) getSCCode(ctx context.Context) (string, error) {
	if v, ok := e.cache.Get(engineName, "sc"); ok && v != "" {
		return v, nil
	}

	headers := make(http.Header)
	headers.Set("Accept-Language", "en-US,en;q=0.5")
	headers.Set("User-Agent", search.DefaultUserAgent)
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.client.Get(ctx, baseURL+"/", headers)
	if err != nil {
		return "", fmt.Errorf("%w: failed to fetch startpage home: %v", search.ErrBlocked, err)
	}
	body, err := search.ReadBody(resp)
	if err != nil {
		return "", err
	}

	if strings.Contains(body, "/sp/captcha") {
		return "", search.ErrCaptcha
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return "", err
	}
	sc, ok := doc.Find("form#search input[name='sc']").Attr("value")
	if !ok || sc == "" {
		return "", fmt.Errorf("%w: could not extract sc code", search.ErrBlocked)
	}

	e.cache.Set(engineName, "sc", sc, scCodeTTL)
	return sc, nil
}

func (e *Engine) mapLanguage(locale string) string {
	mappings := map[string]string{
		"en-us": "english",
		"en-gb": "english_uk",
		"en-ca": "english",
		"id-id": "indonesian",
		"fr-fr": "francais",
		"de-de": "deutsch",
		"es-es": "espanol",
		"it-it": "italiano",
		"ja-jp": "japanese",
		"pt-br": "portuguese",
		"ru-ru": "russian",
		"nl-nl": "nederlands",
	}
	if v, ok := mappings[locale]; ok {
		return v
	}
	return defaultLanguage
}

func detectCaptcha(body string) error {
	if strings.Contains(body, "/sp/captcha") || strings.Contains(strings.ToLower(body), "captcha") {
		return search.ErrCaptcha
	}
	return nil
}

var propsRe = regexp.MustCompile(`React\.createElement\(UIStartpage\.AppSerpWeb,\s*\{`)

func parseResults(body string) ([]search.Result, error) {
	m := propsRe.FindStringIndex(body)
	if m == nil {
		return nil, search.ErrNoResults
	}

	start := m[1]
	end := findPropsEnd(body, start)
	if end <= start {
		return nil, fmt.Errorf("%w: could not locate end of AppSerpWeb props", search.ErrNoResults)
	}

	propsJSON := body[start-1 : end+1]
	propsJSON = strings.TrimSpace(propsJSON)

	var props struct {
		Render struct {
			Presenter struct {
				Regions struct {
					Mainline []struct {
						DisplayType string `json:"display_type"`
						Results     []struct {
							Title       string `json:"title"`
							ClickURL    string `json:"clickUrl"`
							Description string `json:"description"`
						} `json:"results"`
					} `json:"mainline"`
				} `json:"regions"`
			} `json:"presenter"`
		} `json:"render"`
	}
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		return nil, fmt.Errorf("%w: failed to parse AppSerpWeb props: %v", search.ErrNoResults, err)
	}

	var results []search.Result
	for _, region := range props.Render.Presenter.Regions.Mainline {
		if region.DisplayType != "web-google" {
			continue
		}
		for _, r := range region.Results {
			if r.ClickURL == "" || r.Title == "" {
				continue
			}
			results = append(results, search.Result{
				Title:       stripTags(r.Title),
				URL:         r.ClickURL,
				Description: stripTags(r.Description),
			})
		}
	}

	return results, nil
}

func findPropsEnd(text string, start int) int {
	idx := strings.Index(text[start:], "})\n    )\n")
	if idx < 0 {
		return -1
	}
	return start + idx
}

func stripTags(s string) string {
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br />", " ")
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
