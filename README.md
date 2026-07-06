<div align="center">

<img src="docs/banner.svg" alt="autoscout24-mcp banner" width="100%">

# autoscout24-mcp

**MCP server for AutoScout24 — search cars, analyze prices, and look up dealers from any LLM.**

[![Go Reference](https://pkg.go.dev/badge/github.com/adambenhassen/autoscout24-mcp.svg)](https://pkg.go.dev/github.com/adambenhassen/autoscout24-mcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/adambenhassen/autoscout24-mcp)](go.mod)
[![License](https://img.shields.io/github/license/adambenhassen/autoscout24-mcp)](LICENSE)

</div>

MCP server for [AutoScout24](https://www.autoscout24.de) (European car marketplace: `.de`, `.com`, `.it`, …), written in Go. Lets LLMs search car listings, fetch full listing details, run market price analysis, and look up dealers. Data comes from the site's embedded `__NEXT_DATA__` JSON with a configurable anti-bot escalation chain: plain HTTP → Camoufox stealth browser.

## Table of Contents

- [Tools](#tools)
- [Installation](#installation)
- [Configuration](#configuration)
- [Development](#development)
- [Contributing](#contributing)
- [Disclaimer](#disclaimer)

## Tools

| Tool | Description |
|------|-------------|
| `search_listings` | Search by make, model, price/mileage/year/power ranges, fuel, gearbox, body, zip+radius, sort, page |
| `get_listing` | Full details by listing ID or URL: specs, equipment, price vs. market median, images, seller contact |
| `price_analysis` | Price distribution (min/p25/median/avg/p75/max) over up to 100 matching listings + AS24 price-rating breakdown |
| `get_dealer` | Dealer profile (name, address, rating) and current inventory |
| `list_makes_models` | Valid make/model names for search inputs |

## Installation

Requires Go 1.26+.

```bash
go install github.com/adambenhassen/autoscout24-mcp/cmd/autoscout24-mcp@latest
```

Register with Claude Code:

```bash
claude mcp add autoscout24 -- autoscout24-mcp
```

Or add to any MCP client config:

```json
{
  "mcpServers": {
    "autoscout24": {
      "command": "autoscout24-mcp"
    }
  }
}
```

### Docker

A container image with the Camoufox stealth browser baked in is published to GHCR — Python, camoufox (its patched Firefox), and the Playwright driver are all bundled, so the `http → camoufox` escalation chain works out of the box:

```bash
# streamable HTTP transport
docker run --rm -e AS24_HTTP_ADDR=:8080 -p 8080:8080 ghcr.io/adambenhassen/autoscout24-mcp

# stdio transport (for an MCP client that spawns the container)
docker run --rm -i ghcr.io/adambenhassen/autoscout24-mcp
```

Configure it with the same `AS24_*` environment variables as below.

## Configuration

All configuration is via environment variables:

| Env var | Default | Meaning |
|---------|---------|---------|
| `AS24_MARKET` | `de` | Market TLD: `de`, `com`, `it`, `fr`, `nl`, `at`, `es` |
| `AS24_FETCHERS` | `http,camoufox` | Ordered escalation chain; any of `http`, `camoufox` |
| `AS24_HTTP_ADDR` | (unset) | If set (e.g. `:8080`), also serves MCP streamable HTTP; stdio is always on |
| `AS24_CAMOUFOX_CMD` | `uvx camoufox server` | Command to launch the Camoufox sidecar |
| `AS24_TIMEOUT` | `30s` | Per-request timeout, applied to each fetch stage |

Fetch stages escalate automatically on block signals (403/429, challenge pages) and remember blocks per host for 10 minutes. An unconfigured stage that is reached returns an error telling you what to enable.

### Enabling Camoufox

```bash
pip install "camoufox[geoip]"
python -m camoufox fetch   # downloads the stealth browser
```

The server launches `camoufox server` lazily on the first blocked request and reuses one browser instance. Override the launch command with `AS24_CAMOUFOX_CMD`.

## Development

```bash
go test ./...                                # unit tests (fixture-based parsers)
go test -tags integration -run Integration . # live-site integration tests
golangci-lint run
```

Parsers are tested against captured fixtures in [`internal/parser/testdata/`](internal/parser/testdata/) (see its README for the documented JSON paths). Re-capture after site changes with `go run ./cmd/capture-fixtures <url> <name>.html`.

## Contributing

Issues and pull requests are welcome. Before opening a PR, make sure `go test ./...` and `golangci-lint run` pass.

## Disclaimer

This project scrapes publicly visible pages. Respect AutoScout24's terms of service and rate limits; the HTTP fetcher adds politeness jitter between requests. Use responsibly.
