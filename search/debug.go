package search

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DebugResponseBody returns the first n bytes of a response body for debugging.
func DebugResponseBody(resp *http.Response, max int) string {
	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	if len(body) > max {
		return string(body[:max])
	}
	return string(body)
}

// SummaryBody returns a short diagnostic string from an HTML body.
func SummaryBody(body string) string {
	body = strings.ToLower(body)
	snippets := []string{"captcha", "challenge", "blocked", "verify you are human", "rate limit", "too many requests"}
	for _, s := range snippets {
		if strings.Contains(body, s) {
			return fmt.Sprintf("body contains %q", s)
		}
	}
	return ""
}
