package search

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"time"

	"golang.org/x/net/publicsuffix"
)

// HTTPClient performs HTTP requests with a shared cookie jar and anti-bot
// headers. It is the faithful Go equivalent of SearXNG's network layer.
type HTTPClient struct {
	client    *http.Client
	proxy     *url.URL
	userAgent string
}

// NewHTTPClient returns a client configured with the optional proxy and static
// user agent.
func NewHTTPClient(proxyRaw string, timeout time.Duration) (*HTTPClient, error) {
	proxy, err := ParseProxy(proxyRaw)
	if err != nil {
		return nil, err
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}

	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxy),
		DisableKeepAlives: false,
		ForceAttemptHTTP2: false,
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			Jar:       jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		proxy:     proxy,
		userAgent: DefaultUserAgent,
	}, nil
}

// DefaultAntiBotHeaders returns the headers common to all search requests.
func (c *HTTPClient) DefaultAntiBotHeaders() http.Header {
	h := make(http.Header)
	h.Set("User-Agent", c.userAgent)
	h.Set("Accept", DefaultAccept)
	h.Set("Accept-Language", DefaultAcceptLanguage)
	// Do not send Accept-Encoding manually: net/http auto-adds gzip and will
	// transparently decompress the response. Setting it ourselves disables that.
	h.Set("DNT", "1")
	h.Set("Connection", "keep-alive")
	h.Set("Upgrade-Insecure-Requests", "1")
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "none")
	h.Set("Sec-Fetch-User", "?1")
	return h
}

// Do executes an HTTP request, merging the provided headers with defaults.
func (c *HTTPClient) Do(ctx context.Context, method, urlStr string, body []byte, extraHeaders http.Header) (*http.Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	for key, vals := range c.DefaultAntiBotHeaders() {
		if _, ok := extraHeaders[key]; ok {
			continue
		}
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}
	for key, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Set(key, v)
		}
	}

	return c.client.Do(req)
}

// Get is a convenience GET wrapper.
func (c *HTTPClient) Get(ctx context.Context, urlStr string, extraHeaders http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, urlStr, nil, extraHeaders)
}

// Post is a convenience POST wrapper.
func (c *HTTPClient) Post(ctx context.Context, urlStr string, body []byte, extraHeaders http.Header) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, urlStr, body, extraHeaders)
}

// RawPost performs a POST using only the provided headers (no anti-bot defaults).
func (c *HTTPClient) RawPost(ctx context.Context, urlStr string, body []byte, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, vals := range headers {
		for _, v := range vals {
			req.Header.Set(key, v)
		}
	}
	return c.client.Do(req)
}

// SetCookies injects cookies into the jar for the given URL.
func (c *HTTPClient) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.client.Jar.SetCookies(u, cookies)
}

// ReadBody reads the response body and closes it.
func ReadBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// FormEncode encodes a form map into URL-encoded bytes with sorted keys for
// deterministic output.
func FormEncode(data map[string]string) []byte {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	form := url.Values{}
	for _, k := range keys {
		form.Set(k, data[k])
	}
	return []byte(form.Encode())
}
