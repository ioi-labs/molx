package searchfactory

import (
	"log/slog"

	"nexora-crawl/search"
	"nexora-crawl/search/brave"
	"nexora-crawl/search/duckduckgo"
	"nexora-crawl/search/startpage"
)

// BuildEngines creates the configured engine instances from their names.
func BuildEngines(cache *search.SharedCache, client *search.HTTPClient, names []string) []search.Engine {
	var engines []search.Engine
	for _, n := range names {
		switch n {
		case "duckduckgo":
			engines = append(engines, duckduckgo.New(cache, client))
		case "brave":
			engines = append(engines, brave.New(client))
		case "startpage":
			engines = append(engines, startpage.New(cache, client))
		default:
			slog.Warn("unknown search engine", "name", n)
		}
	}
	return engines
}
