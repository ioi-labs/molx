package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nexora-crawl/config"
	"nexora-crawl/extractor"
	"nexora-crawl/middleware"
	"nexora-crawl/models"
	"nexora-crawl/obscura"
	"nexora-crawl/validator"
)

type V2ScrapeHandler struct {
	Config *config.Config
	Client *obscura.Client
}

func (h *V2ScrapeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req models.V2ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.execute(w, r, &req); err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, err.Error())
	}
}

func (h *V2ScrapeHandler) execute(w http.ResponseWriter, r *http.Request, req *models.V2ScrapeRequest) error {
	targetURL := req.TargetURL()
	if targetURL == "" {
		return fmt.Errorf("missing url")
	}
	if err := validator.ValidateURL(targetURL); err != nil {
		return err
	}

	formats := normalizeFormats(req.Formats)
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = int(h.Config.Timeout.Milliseconds())
	}
	wait := req.WaitFor / 1000
	if req.WaitFor > 0 && wait < 1 {
		wait = 1
	}

	userAgent := ""
	if req.Mobile {
		userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	}

	preEval := buildActionsEval(req.Actions)

	ctx, cancel := context.WithTimeout(r.Context(), h.Config.Timeout)
	defer cancel()

	data, err := h.Client.Fetch(ctx, models.FetchRequest{
		URL:       targetURL,
		Dump:      "markdown",
		Timeout:   timeout / 1000,
		Wait:      wait,
		Eval:      preEval,
		Proxy:     strings.TrimSpace(req.Proxy),
		Stealth:   req.BlockAds,
		UserAgent: userAgent,
	})

	resp := models.V2ScrapeResponse{Success: err == nil}
	if err != nil {
		resp.Error = err.Error()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return nil
	}

	md := extractor.ResolveMarkdownURLs(string(data), targetURL)
	if req.OnlyMainContent {
		md, _ = extractor.ApplyFilters(md, []string{"nav", "header", "footer", "aside", "[role='navigation']"})
	}
	if len(req.ExcludeTags) > 0 {
		md, _ = extractor.ApplyFilters(md, req.ExcludeTags)
	}
	md = extractor.CompressMarkdownWhitespace(md)

	html := ""
	if hasFormat(formats, "html") || hasFormat(formats, "text") || hasFormat(formats, "links") {
		// We fetched markdown above; for other formats we need the rendered HTML.
		htmlBytes, htmlErr := h.Client.Fetch(ctx, models.FetchRequest{
			URL:       targetURL,
			Dump:      "html",
			Timeout:   timeout / 1000,
			Wait:      wait,
			Eval:      preEval,
			Proxy:     strings.TrimSpace(req.Proxy),
			Stealth:   req.BlockAds,
			UserAgent: userAgent,
		})
		if htmlErr == nil {
			html = string(htmlBytes)
			if req.OnlyMainContent {
				html, _ = extractor.ApplyFilters(html, []string{"nav", "header", "footer", "aside", "[role='navigation']"})
			}
			if len(req.ExcludeTags) > 0 {
				html, _ = extractor.ApplyFilters(html, req.ExcludeTags)
			}
		}
	}

	htmlForMeta := html
	if htmlForMeta == "" {
		// ponytail: re-fetch html once for metadata when only markdown was requested.
		htmlBytes, htmlErr := h.Client.Fetch(ctx, models.FetchRequest{
			URL:       targetURL,
			Dump:      "html",
			Timeout:   timeout / 1000,
			Wait:      wait,
			Eval:      preEval,
			Proxy:     strings.TrimSpace(req.Proxy),
			Stealth:   req.BlockAds,
			UserAgent: userAgent,
		})
		if htmlErr == nil {
			htmlForMeta = string(htmlBytes)
		}
	}
	metadata := extractor.ExtractMetadata(firstNonEmpty(htmlForMeta, md), targetURL, 200, "text/html")

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
			out.Links = extractor.ExtractLinks(html, targetURL)
		}
	}

	resp.Data = out
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
	return nil
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
