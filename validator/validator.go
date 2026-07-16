package validator

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL checks that target is an absolute http(s) URL and is not a
// private / loopback / link-local / file / javascript / data URI.
func ValidateURL(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("URL must be absolute")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q (only http/https)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL missing host")
	}

	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	ips, err := net.LookupIP(host)
	if err == nil {
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
				return fmt.Errorf("forbidden IP address for host %q", host)
			}
		}
	}

	return nil
}
