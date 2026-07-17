package search

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

// SharedCache holds transient values required by search engines, keyed by a
// stable hash of the query context (query + UA + engine specific salt).
type SharedCache struct {
	store *cache.Cache
}

// NewSharedCache returns a cache with the default 1 hour TTL and 10 minute
// cleanup interval.
func NewSharedCache() *SharedCache {
	return &SharedCache{store: cache.New(time.Hour, 10*time.Minute)}
}

// Set stores a value for the given engine, key and TTL.
func (c *SharedCache) Set(engine, key string, value string, ttl time.Duration) {
	c.store.Set(cacheKey(engine, key), value, ttl)
}

// Get retrieves a cached value.
func (c *SharedCache) Get(engine, key string) (string, bool) {
	v, ok := c.store.Get(cacheKey(engine, key))
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func cacheKey(engine, key string) string {
	return engine + "::" + key
}

// SecretHash builds a cache key from an arbitrary set of fields. It is a stable
// hash meant to keep cache keys short, not cryptographically secure.
func SecretHash(fields ...string) string {
	h := 0
	for _, f := range fields {
		for _, r := range f {
			h = 31*h + int(r)
		}
	}
	return fmt.Sprintf("%x", h)
}

// NormalizeLocale returns a lowercased locale string with underscores replaced
// by hyphens. Empty input yields "en-us".
func NormalizeLocale(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "en-us"
	}
	return strings.ReplaceAll(s, "_", "-")
}

// ParseProxy validates that the proxy string uses a supported scheme.
func ParseProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return u, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", u.Scheme)
	}
}

// ClampPage normalizes pagination to 1-based pages.
func ClampPage(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

// ClampLimit normalizes the requested limit.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

// MapSafeSearch maps a 0-2 safe search level to engine-specific strings.
func MapSafeSearch(level int, mapping map[int]string) string {
	if v, ok := mapping[level]; ok {
		return v
	}
	return mapping[0]
}

// DefaultUserAgent is a static, modern browser user agent. Search engines hash
// behaviour on the user agent, so it must not change per request.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// DefaultAcceptLanguage is a sensible Accept-Language fallback.
const DefaultAcceptLanguage = "en-US,en;q=0.9"

// DefaultAccept is the default HTTP Accept header.
const DefaultAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"

// TimeRange maps common time range names to single-letter codes used by DDG and
// Startpage. Brave uses different codes (pd/pw/pm/py) in its own engine.
var TimeRange = map[string]string{
	"day":   "d",
	"week":  "w",
	"month": "m",
	"year":  "y",
}

// IsKnownTimeRange reports whether the value is a supported time range.
func IsKnownTimeRange(s string) bool {
	_, ok := TimeRange[s]
	return ok
}
