package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"nexora-crawl/config"
	"nexora-crawl/middleware"
)

// SearXNGSearchHandler proxies GET and POST /search to a configured SearXNG
// instance using the native SearXNG API parameters and response format.
type SearXNGSearchHandler struct {
	Config *config.Config
}

func (h *SearXNGSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Config.SearXNGURL == "" {
		middleware.WriteJSONError(w, http.StatusServiceUnavailable, "search not configured")
		return
	}

	searxURL, err := h.buildSearXNGURL(r)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	method := http.MethodGet
	var body io.Reader
	contentType := ""
	if r.Method == http.MethodPost {
		method = http.MethodPost
		body = r.Body
		contentType = r.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/x-www-form-urlencoded"
		}
	}

	client := &http.Client{}
	proxyReq, err := http.NewRequestWithContext(r.Context(), method, searxURL, body)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("proxy request failed: %v", err))
		return
	}
	proxyReq.Header.Set("Accept", "application/json")
	proxyReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NexoraCrawl/1.0)")
	if contentType != "" {
		proxyReq.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		middleware.WriteJSONError(w, http.StatusBadGateway, fmt.Sprintf("searxng failed: %v", err))
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *SearXNGSearchHandler) buildSearXNGURL(r *http.Request) (string, error) {
	u, err := url.Parse(h.Config.SearXNGURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	if r.Method == http.MethodGet {
		for key, values := range r.URL.Query() {
			for _, v := range values {
				q.Add(key, v)
			}
		}
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return "", err
		}
		for key, values := range r.PostForm {
			for _, v := range values {
				q.Add(key, v)
			}
		}
	}

	if q.Get("format") == "" {
		q.Set("format", "json")
	}
	if q.Get("q") == "" {
		return "", fmt.Errorf("missing query parameter q")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}
