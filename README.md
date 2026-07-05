# autoscout24-mcp

MCP server for AutoScout24 (European marketplace: autoscout24.de/.com/.it/…), written in Go. Lets LLMs search car listings, fetch full listing details, run market price analysis, and look up dealers. Data comes from the site's embedded `__NEXT_DATA__` JSON with a configurable anti-bot escalation chain: plain HTTP → Camoufox stealth browser → crw scraping service.

## Tools

| Tool | Description |
|------|-------------|
| `search_listings` | Search by make, model, price/mileage/year/power ranges, fuel, gearbox, body, zip+radius, sort, page |
| `get_listing` | Full details by listing ID or URL: specs, equipment, price vs. market median, images, seller contact |
| `price_analysis` | Price distribution (min/p25/median/avg/p75/max) over up to 100 matching listings + AS24 price-rating breakdown |
| `get_dealer` | Dealer profile (name, address, rating) and current inventory |
| `list_makes_models` | Valid make/model names for search inputs |

## Install

```bash
go install github.com/adam/autoscout24-mcp/cmd/autoscout24-mcp@latest
```

Register with Claude Code:

```bash
claude mcp add autoscout24 -- autoscout24-mcp
```

## Configuration

| Env var | Default | Meaning |
|---------|---------|---------|
| `AS24_MARKET` | `de` | Market TLD: `de`, `com`, `it`, `fr`, `nl`, `at`, `be`, `es` |
| `AS24_FETCHERS` | `http,camoufox` | Ordered escalation chain; any of `http`, `camoufox`, `crw` |
| `AS24_HTTP_ADDR` | (unset) | If set (e.g. `:8080`), also serves MCP streamable HTTP; stdio is always on |
| `AS24_CAMOUFOX_CMD` | `uvx camoufox server` | Command to launch the Camoufox sidecar |
| `CRW_URL` / `CRW_API_KEY` | (unset) | Firecrawl-compatible scrape API endpoint + key for the `crw` stage |
| `AS24_TIMEOUT` | `30s` | Per-request timeout |

Fetch stages escalate automatically on block signals (403/429, challenge pages) and remember blocks per host for 10 minutes. An unconfigured stage that is reached returns an error telling you what to enable.

### Enabling Camoufox

```bash
pip install "camoufox[geoip]"
python -m camoufox fetch   # downloads the stealth browser
```

The server launches `camoufox server` lazily on the first blocked request and reuses one browser instance. Override the launch command with `AS24_CAMOUFOX_CMD`.

### Enabling crw

Point `CRW_URL` (and `CRW_API_KEY` if required) at any Firecrawl-compatible `/v1/scrape` endpoint and add `crw` to `AS24_FETCHERS`, e.g. `AS24_FETCHERS=http,camoufox,crw`.

## Development

```bash
go test ./...                                # unit tests (fixture-based parsers)
go test -tags integration -run Integration . # live-site integration tests
golangci-lint run
```

Parsers are tested against captured fixtures in `internal/parser/testdata/` (see its README for the documented JSON paths). Re-capture after site changes with `go run ./cmd/capture-fixtures <url> <name>.html`.

## Disclaimer

This project scrapes publicly visible pages. Respect AutoScout24's terms of service and rate limits; the HTTP fetcher adds politeness jitter between requests. Use responsibly.
