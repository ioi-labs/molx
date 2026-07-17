package search

import "errors"

// Common errors returned by search engines.
var (
	ErrCaptcha   = errors.New("captcha challenge detected")
	ErrBlocked   = errors.New("request blocked by the search engine")
	ErrNoResults = errors.New("no results found")
	ErrInvalid   = errors.New("invalid search options")
)
