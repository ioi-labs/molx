# Molx

A self-hostable alternative to Firecrawl. Molx is a simple HTTP service that turns web pages into clean, structured data. You give it a URL, and it returns markdown, HTML, plain text, links, or metadata. It can also search the web and optionally scrape every result.

Built on top of [Obscura](https://github.com/berstend/obscura) for headless browser rendering. Web search is performed natively through Brave, DuckDuckGo, and Startpage (no external SearXNG instance required).

Source code: [https://github.com/ioi-labs/molx](https://github.com/ioi-labs/molx)

Interactive API documentation is available at `/reference` once the server is running.

---

## What it does

- **Scrape one page**: fetch rendered content as markdown, HTML, text, or links.
- **Batch scrape**: process many URLs in the background and poll for results.
- **Search the web**: query Brave, DuckDuckGo, and Startpage natively and optionally scrape each result page.
- **API key protection**: lock the API with a bearer token.
- **OpenTelemetry support**: send traces to any OTLP-compatible backend (optional).

---

## Quick start

### Run with Docker

The easiest way to run the server is with the published container image.

```bash
docker run -d \
  -p 8080:8080 \
  -e API_KEY=your-secret-key \
  ghcr.io/ioi-labs/molx:latest
```

Then open `http://localhost:8080/reference` for the interactive API docs.

### Run from source

You need Go 1.23 or later and the Obscura binaries for your platform.

```bash
# Copy Obscura into the deps folder
make deps

# Start the server
API_KEY=your-secret-key go run .
```

---

## Configuration

Set these environment variables to configure the server.

| Variable | Default | Purpose |
|----------|---------|---------|
| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | `8080` | HTTP port |
| `API_KEY` | empty | Bearer token required by all scrape/search endpoints |
| `OBSCURA_BIN` | `deps/obscura` | Path to the Obscura binary |
| `TIMEOUT_MS` | `60000` | Default timeout for Obscura calls |
| `SEARCH_ENGINES` | `duckduckgo,brave,startpage` | Comma-separated native engines |
| `SEARCH_TIMEOUT_MS` | `30000` | Timeout per engine query |
| `SEARCH_DEFAULT_LIMIT` | `10` | Default number of search results |
| `PROXY` | empty | Optional proxy for search and scrape |
| `OTEL_ENDPOINT` | empty | OTLP endpoint for traces |
| `ALLOWED_ORIGIN` | empty | CORS origins, comma separated |
| `LLM_BASE_URL` | empty | OpenAI-compatible chat completions base URL (e.g. `https://api.openai.com`) |
| `LLM_API_KEY` | empty | API key for the LLM provider |
| `LLM_MODEL` | empty | Model name, e.g. `gpt-4o-mini`. Required for `onlyCleanContent`. |

---

## Authentication

If `API_KEY` is set, every scrape and search request must include the token in the `Authorization` header:

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
    "onlyMainContent": true,
    "onlyCleanContent": true
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
      "onlyMainContent": true,
      "onlyCleanContent": true
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

The CI workflow downloads the Obscura binaries automatically from the upstream release. For local builds, place the correct Linux Obscura binaries under:

```
deps/obscura/linux/amd64/obscura
deps/obscura/linux/amd64/obscura-worker
deps/obscura/linux/arm64/obscura
deps/obscura/linux/arm64/obscura-worker
```

Then build locally:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t molx:latest .
```

---

## Sponsor

This project is supported by:

> **[Your company name here]**
>
> Interested in sponsoring Molx? Open an issue or email us at hi@ioi.co.id.

---

## License

MIT
