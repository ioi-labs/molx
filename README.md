<p align="center">
  <img src="assets/molx.png" alt="Molx - self-hosted Firecrawl alternative" width="160">
</p>

<h1 align="center">Molx — Self-Hosted Firecrawl Alternative</h1>

<p align="center">
  Open-source web scraping API that turns any page into clean markdown, HTML, text, links, or structured JSON.<br>
  Scrape, search, batch-process, and enrich content with LLMs — on your own infrastructure.
</p>

<p align="center">
  <img src="https://img.shields.io/github/v/release/ioi-labs/molx?logo=github" alt="GitHub release">
  <img src="https://img.shields.io/github/license/ioi-labs/molx" alt="License">
  <img src="https://img.shields.io/badge/Go-1.23%2B-blue?logo=go" alt="Go version">
</p>

---

## What is Molx?

**Molx is a self-hosted, open-source alternative to Firecrawl.** It is a simple HTTP service that fetches web pages through a headless browser, extracts clean structured data, and returns it as markdown, HTML, plain text, links, or metadata.

Molx can also search the web natively — no external SearXNG instance required — and optionally scrape every search result. With the enrich endpoint, you can pipe scraped content into any OpenAI-compatible LLM and extract structured JSON using a JSON Schema.

Built on top of [Obscura](https://github.com/berstend/obscura) for headless browser rendering. Web search runs through Brave, DuckDuckGo, and Startpage.

---

## Why choose Molx over Firecrawl?

| | Molx | Firecrawl |
|---|---|---|
| **Hosting** | Self-hosted on your own server or cloud | Managed SaaS |
| **Data privacy** | Your data never leaves your infrastructure | Sent to vendor infrastructure |
| **Cost control** | No per-credit billing; run on your own hardware | Usage-based paid plans |
| **Web search** | Native Brave, DuckDuckGo, Startpage support | Available on some plans |
| **LLM enrichment** | Any OpenAI-compatible provider | Built-in or provider-specific |
| **Deployment** | Single Docker container or Go binary | Managed only |
| **Observability** | OpenTelemetry built in | Vendor dashboards |

If you are looking for a **Firecrawl alternative** that you can host yourself, Molx gives you the same core capabilities with full control over cost, privacy, and uptime.

---

## Features

- **Scrape one page** — fetch rendered content as markdown, HTML, text, or links.
- **Batch scrape** — process many URLs in the background and poll for results.
- **Search the web** — query Brave, DuckDuckGo, and Startpage natively and optionally scrape each result page.
- **Enrich with LLM** — search or scrape pages, then ask an OpenAI-compatible LLM to extract structured data from the content.
- **API key protection** — lock the API with a bearer token.
- **OpenTelemetry support** — send traces to any OTLP-compatible backend (optional).

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

### Run with Docker Compose

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and set API_KEY, then start the stack
docker compose up -d
```

### Run from source

You need Go 1.23 or later and the Obscura binaries for your platform.

```bash
# Copy Obscura into the deps folder
make deps

# Start the server
API_KEY=your-secret-key go run .
```

---

## Authentication

If `API_KEY` is set, every scrape and search request must include the token in the `Authorization` header:

```bash
Authorization: Bearer your-secret-key
```

`GET /health` and `GET /reference` do not need a token.

---

## API endpoints

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

### Enrich with LLM

Use this endpoint when you want structured data extracted from web content. Molx searches the web or scrapes the URLs you provide, then sends the combined content to an OpenAI-compatible LLM.

Requirements:

- `LLM_BASE_URL`
- `LLM_API_KEY`
- `LLM_MODEL`

Start an enrichment job:

```bash
curl -X POST http://localhost:8080/enrich \
  -H "Authorization: Bearer your-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Extract the company name, CEO, and funding amount from each page",
    "schema": {
      "type": "object",
      "properties": {
        "company": { "type": "string" },
        "ceo": { "type": "string" },
        "funding": { "type": "string" }
      }
    },
    "urls": ["https://example.com"]
  }'
```

If `urls` is omitted, Molx searches the web using your prompt and scrapes the top results.

Response:

```json
{
  "success": true,
  "id": "a1b2c3d4",
  "url": "http://localhost:8080/enrich/a1b2c3d4",
  "status": "pending"
}
```

Poll the job URL to get the result:

```bash
curl http://localhost:8080/enrich/a1b2c3d4 \
  -H "Authorization: Bearer your-secret-key"
```

Response:

```json
{
  "id": "a1b2c3d4",
  "status": "completed",
  "prompt": "Extract the company name, CEO, and funding amount from each page",
  "schema": { ... },
  "urls": ["https://example.com"],
  "result": {
    "company": "Example Inc",
    "ceo": "Jane Doe",
    "funding": "$10M"
  },
  "createdAt": "2026-07-23T06:00:00Z",
  "expiresAt": "2026-07-24T06:00:00Z"
}
```

---

## Configuration

Set these environment variables to configure the server.

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
| `LLM_MODEL` | empty | Model name, e.g. `gpt-4o-mini`. Required for `onlyCleanContent` |

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

## Roadmap

Planned features, in rough order:

- **Optional Redis cache** — cache scrape results and job status without making Redis a hard dependency. Molx keeps working as a single binary when Redis is not configured.

- **Optional Postgres persistence** — store batch and enrich jobs in Postgres so they survive restarts and redeploys. In-memory mode stays available for simple, single-node deployments.

- **Advanced proxy integration** — support rotating proxies, proxy pools, and per-domain proxy rules for scraping at scale.

- **Pluggable scrapers** — let users ship custom scraper plugins as executables (for example, Go binaries backed by Playwright) and map them to specific domains. Molx runs matching plugins under its internal engine. Plugins are loaded at startup, so a restart is needed after adding or updating one.

---

## FAQ

### Is Molx a Firecrawl alternative?

Yes. Molx provides the same core capabilities as Firecrawl — scrape, search, batch-process, and LLM-enrich web content — but it is fully self-hosted and open source.

### Can I self-host Molx?

Yes. Molx runs as a single Docker container or as a standalone Go binary. You control your own data, costs, and uptime.

### Does Molx require SearXNG or another search aggregator?

No. Molx searches the web natively through Brave, DuckDuckGo, and Startpage. No external SearXNG instance is required.

### What LLM providers work with Molx?

Any provider with an OpenAI-compatible chat completions API, such as OpenAI, Groq, Together AI, or a self-hosted vLLM instance.

### How is Molx different from Firecrawl?

Molx is self-hosted, open source, and gives you native web search, OpenTelemetry observability, and the freedom to plug in any LLM provider. Firecrawl is a managed SaaS with usage-based pricing.

### Is Molx free to use?

Yes. Molx is released under the MIT License. You pay only for the infrastructure you run it on.

---

## Contributing

See [CONTRIBUTION.md](CONTRIBUTION.md) for setup instructions, code style, and how to open a pull request.

---

## Sponsor

This project is supported by:

> **[Your company name here]**
>
> Interested in sponsoring Molx? Open an issue or email us at hi@ioi.co.id.

---

## License

MIT
