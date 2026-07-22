package scraper

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"nexora-crawl/cleaner"
	"nexora-crawl/config"
	"nexora-crawl/extractor"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
)

// Options carries the common scrape options used by both sync and async paths.
type Options struct {
	Formats          []string
	OnlyMainContent  bool
	OnlyCleanContent bool
	IncludeTags      []string
	ExcludeTags      []string
	WaitFor          int
	Timeout          int
	Mobile           bool
	Proxy            string
	BlockAds         bool
	Actions          []models.V2ScrapeAction
}

// V2Scraper performs a single-URL scrape using the Obscura CLI.
type V2Scraper struct {
	Config  *config.Config
	Client  obscura.Fetcher
	Cleaner *cleaner.Client
}

// NewV2 returns a scraper bound to the given config and Obscura client.
func NewV2(cfg *config.Config, client obscura.Fetcher) *V2Scraper {
	return &V2Scraper{
		Config: cfg,
		Client: client,
		Cleaner: cleaner.NewClient(cleaner.Config{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
		}),
	}
}

// Run fetches and extracts content for one URL.
func (s *V2Scraper) Run(ctx context.Context, url string, opts Options) (*models.V2ScrapeData, error) {
	formats := normalizeFormats(opts.Formats)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = int(s.Config.Timeout.Milliseconds())
	}
	wait := opts.WaitFor / 1000
	if opts.WaitFor > 0 && wait < 1 {
		wait = 1
	}

	userAgent := ""
	if opts.Mobile {
		userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	}

	preEval := buildActionsEval(opts.Actions)

	data, err := s.Client.Fetch(ctx, models.FetchRequest{
		URL:       url,
		Dump:      "markdown",
		Timeout:   timeout / 1000,
		Wait:      wait,
		Eval:      preEval,
		Proxy:     strings.TrimSpace(opts.Proxy),
		Stealth:   opts.BlockAds,
		UserAgent: userAgent,
	})
	if err != nil {
		return nil, err
	}

	md := extractor.ResolveMarkdownURLs(string(data), url)
	if opts.OnlyMainContent {
		md, _ = extractor.ApplyFilters(md, []string{"nav", "header", "footer", "aside", "[role='navigation']"})
	}
	if len(opts.ExcludeTags) > 0 {
		md, _ = extractor.ApplyFilters(md, opts.ExcludeTags)
	}
	if len(opts.IncludeTags) > 0 {
		md, _ = extractor.ApplyIncludeFilters(md, opts.IncludeTags)
	}
	md = extractor.CompressMarkdownWhitespace(md)

	if opts.OnlyCleanContent {
		if s.Cleaner == nil {
			return nil, errors.New("LLM not provided")
		}
		cleaned, cleanErr := s.Cleaner.CleanMarkdown(ctx, md)
		if cleanErr != nil {
			return nil, cleanErr
		}
		md = cleaned
	}

	html := ""
	if hasFormat(formats, "html") || hasFormat(formats, "text") || hasFormat(formats, "links") {
		htmlBytes, htmlErr := s.Client.Fetch(ctx, models.FetchRequest{
			URL:       url,
			Dump:      "html",
			Timeout:   timeout / 1000,
			Wait:      wait,
			Eval:      preEval,
			Proxy:     strings.TrimSpace(opts.Proxy),
			Stealth:   opts.BlockAds,
			UserAgent: userAgent,
		})
		if htmlErr == nil {
			html = string(htmlBytes)
			if opts.OnlyMainContent {
				html, _ = extractor.ApplyFilters(html, []string{"nav", "header", "footer", "aside", "[role='navigation']"})
			}
			if len(opts.ExcludeTags) > 0 {
				html, _ = extractor.ApplyFilters(html, opts.ExcludeTags)
			}
			if len(opts.IncludeTags) > 0 {
				html, _ = extractor.ApplyIncludeFilters(html, opts.IncludeTags)
			}
		}
	}

	htmlForMeta := html
	if htmlForMeta == "" {
		htmlBytes, htmlErr := s.Client.Fetch(ctx, models.FetchRequest{
			URL:       url,
			Dump:      "html",
			Timeout:   timeout / 1000,
			Wait:      wait,
			Eval:      preEval,
			Proxy:     strings.TrimSpace(opts.Proxy),
			Stealth:   opts.BlockAds,
			UserAgent: userAgent,
		})
		if htmlErr == nil {
			htmlForMeta = string(htmlBytes)
			if opts.OnlyMainContent {
				htmlForMeta, _ = extractor.ApplyFilters(htmlForMeta, []string{"nav", "header", "footer", "aside", "[role='navigation']"})
			}
			if len(opts.ExcludeTags) > 0 {
				htmlForMeta, _ = extractor.ApplyFilters(htmlForMeta, opts.ExcludeTags)
			}
			if len(opts.IncludeTags) > 0 {
				htmlForMeta, _ = extractor.ApplyIncludeFilters(htmlForMeta, opts.IncludeTags)
			}
		}
	}
	metadata := extractor.ExtractMetadata(firstNonEmpty(htmlForMeta, md), url, 200, "text/html")

	out := &models.V2ScrapeData{Metadata: metadata}
	for _, f := range formats {
		switch f {
		case "markdown":
			out.Markdown = md
		case "html":
			out.HTML = html
		case "text":
			out.Text = extractor.ExtractText(html)
		case "links":
			out.Links = extractor.ExtractLinks(html, url)
		}
	}
	return out, nil
}

func normalizeFormats(formats []string) []string {
	if len(formats) == 0 {
		return []string{"markdown"}
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(formats))
	for _, f := range formats {
		f = strings.ToLower(strings.TrimSpace(f))
		switch f {
		case "markdown", "html", "text", "links":
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	if len(out) == 0 {
		return []string{"markdown"}
	}
	return out
}

func hasFormat(formats []string, name string) bool {
	for _, f := range formats {
		if f == name {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// buildActionsEval converts a list of actions into a single JS expression.
func buildActionsEval(actions []models.V2ScrapeAction) string {
	if len(actions) == 0 {
		return ""
	}
	var parts []string
	for _, a := range actions {
		switch a.Type {
		case "wait":
			ms := a.Milliseconds
			if ms <= 0 {
				ms = 1000
			}
			parts = append(parts, "await new Promise(r => setTimeout(r, "+strconv.Itoa(ms)+"));")
		case "click":
			if a.Selector == "" {
				continue
			}
			parts = append(parts, "document.querySelector('"+strings.ReplaceAll(strings.ReplaceAll(a.Selector, `\`, `\\`), `'`, `\'`)+"')?.click();")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "(async () => { " + strings.Join(parts, " ") + " })();"
}
