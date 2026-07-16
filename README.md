# Nexora Crawl

A self-hostable alternative to Firecrawl. Nexora Crawl is a simple HTTP service that turns web pages into clean, structured data. You give it a URL, and it returns markdown, HTML, plain text, links, or metadata. It can also search the web and optionally scrape every result.

Built on top of [Obscura](https://github.com/berstend/obscura) for headless browser rendering and [SearXNG](https://github.com/searxng/searxng) for web search.

Source code: [https://github.com/ioi-labs/nexora-crawl](https://github.com/ioi-labs/nexora-crawl)

---

## What it does

- **Scrape one page**: fetch rendered content as markdown, HTML, text, or links.
- **Batch scrape**: process many URLs in the background and poll for results.
- **Search the web**: query SearXNG and optionally scrape each result page.
- **API key protection**: lock the API with a bearer token.
- **OpenTelemetry support**: send traces to any OTLP-compatible backend (optional).

---

## Quick start

### Run with Docker

The easiest way to run the server is with the published container image.

```bash
docker run -d \
  -p 8080:8080 \
  -e NEXORA_CRAWL_API_KEY=your-secret-key \
  -e NEXORA_CRAWL_SEARXNG_URL=https://your-searxng-instance.example.com \
  ghcr.io/ioi-labs/nexora-crawl:latest
```

Then open `http://localhost:8080/reference` for the interactive API docs.

### Run from source

You need Go 1.23 or later and the Obscura binaries for your platform.

```bash
# Copy Obscura into the deps folder
make deps

# Start the server
NEXORA_CRAWL_API_KEY=your-secret-key go run .
```

---

## Configuration

Set these environment variables to configure the server.

| Variable | Default | Purpose |
|----------|---------|---------|
| `NEXORA_CRAWL_PORT` | `8080` | HTTP port |
| `NEXORA_CRAWL_API_KEY` | empty | Bearer token required by all scrape/search endpoints |
| `NEXORA_CRAWL_OBSCURA_BIN` | `deps/obscura` | Path to the Obscura binary |
| `NEXORA_CRAWL_TIMEOUT_MS` | `60000` | Default timeout for Obscura calls |
| `NEXORA_CRAWL_SEARXNG_URL` | empty | SearXNG instance URL for `/search` |
| `NEXORA_CRAWL_OTEL_ENDPOINT` | empty | OTLP endpoint for traces |
| `NEXORA_CRAWL_ALLOWED_ORIGIN` | empty | CORS origins, comma separated |

---

## Authentication

If `NEXORA_CRAWL_API_KEY` is set, every scrape and search request must include the token in the `Authorization` header:

```bash
Authorization: Bearer your-secret-key
```

`GET /health` and `GET /reference` do not need a token.

---

## Endpoints

### Health check

```bash
curl http://localhost:8080/health
```

Response:

```json
{ "status": "ok" }
```

### Scrape a single page

```bash
curl -X POST http://localhost:8080/v2/scrape \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "formats": ["markdown"],
    "onlyMainContent": true
  }'
```

Response:

```json
{
  "success": true,
  "data": {
    "markdown": "# Example Domain\n\nThis domain...",
    "metadata": {
      "title": "Example Domain",
      "sourceURL": "https://example.com"
    }
  }
}
```

### Search the web

```bash
curl -X POST http://localhost:8080/search \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "golang context cancel",
    "limit": 5,
    "scrapeOptions": {
      "formats": ["markdown"],
      "onlyMainContent": true
    }
  }'
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "title": "...",
      "url": "https://...",
      "description": "...",
      "markdown": "..."
    }
  ]
}
```

### Batch scrape

```bash
curl -X POST http://localhost:8080/v2/batch/scrape \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "urls": ["https://example.com", "https://example.org"]
  }'
```

The response includes a job URL. Poll it to get the results.

---

## Building the Docker image

The Dockerfile supports two architectures: `linux/amd64` and `linux/arm64`.

Before building, place the correct Linux Obscura binaries under:

```
deps/obscura/linux/amd64/obscura
deps/obscura/linux/amd64/obscura-worker
deps/obscura/linux/arm64/obscura
deps/obscura/linux/arm64/obscura-worker
```

Then build locally:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t nexora-crawl:latest .
```

---

## Sponsor

This project is supported by:

> **[Your company name here]**
>
> Interested in sponsoring Nexora Crawl? Open an issue or email us at hi@ioi.co.id.

---

## License

MIT
