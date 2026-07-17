package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port              string
	ObscuraBinaryPath string
	OTelEndpoint      string
	APIKey            string
	Timeout           time.Duration
	AllowedOrigins    []string

	SearchEngines      []string
	SearchTimeout      time.Duration
	SearchDefaultLimit int
	Proxy              string
}

func Load() *Config {
	return &Config{
		Port:              getEnv("NEXORA_CRAWL_PORT", "8080"),
		ObscuraBinaryPath: getEnv("NEXORA_CRAWL_OBSCURA_BIN", "deps/obscura"),
		OTelEndpoint:      getEnv("NEXORA_CRAWL_OTEL_ENDPOINT", ""),
		APIKey:            getEnv("NEXORA_CRAWL_API_KEY", ""),
		Timeout:           parseDurationMs(getEnv("NEXORA_CRAWL_TIMEOUT_MS", "60000")),
		AllowedOrigins:    splitOrigins(getEnv("NEXORA_CRAWL_ALLOWED_ORIGIN", "")),

		SearchEngines:      splitTrim(getEnv("NEXORA_CRAWL_SEARCH_ENGINES", "duckduckgo,brave,startpage")),
		SearchTimeout:      parseDurationMs(getEnv("NEXORA_CRAWL_SEARCH_TIMEOUT_MS", "30000")),
		SearchDefaultLimit: parseInt(getEnv("NEXORA_CRAWL_SEARCH_DEFAULT_LIMIT", "10")),
		Proxy:              getEnv("NEXORA_CRAWL_PROXY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func parseDurationMs(s string) time.Duration {
	ms, err := strconv.Atoi(s)
	if err != nil || ms <= 0 {
		return 60 * time.Second
	}
	return time.Duration(ms) * time.Millisecond
}

func splitOrigins(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitTrim(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 10
	}
	return n
}
