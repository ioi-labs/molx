package middleware

import (
	"net/http"
	"strings"

	"nexora-crawl/config"
)

// APIKey returns a middleware that validates the Authorization: Bearer header.
func APIKey(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.APIKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			token := strings.TrimSpace(auth[len(prefix):])
			apiKey := strings.TrimSpace(cfg.APIKey)
			if token == "" || !strings.EqualFold(token, apiKey) {
				WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
