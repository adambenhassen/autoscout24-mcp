# syntax=docker/dockerfile:1
# ---- build the Go binary + fetch the matching playwright-go driver ----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoscout24-mcp ./cmd/autoscout24-mcp
# Download the playwright-go driver (node + playwright 1.49.1) so the runtime can
# start playwright and connect to the camoufox browser. The version MUST match
# the go.mod playwright-go dependency (v0.4902.0 → driver 1.49.1) AND the runtime
# python playwright below: playwright refuses a Connect across a version skew.
# PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD keeps this to the driver only — we connect to
# camoufox's own patched Firefox, never a plain Playwright browser.
RUN PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4902.0 install firefox

# ---- runtime: python + camoufox stealth browser + the driver + the binary ----
FROM python:3.12-slim-bookworm
ENV PYTHONUNBUFFERED=1 \
    AS24_CAMOUFOX_CMD="/usr/local/bin/camoufox-server"

# camoufox (a patched Firefox) plus its browser + geoip data. Pin playwright to
# 1.49.1 to match the playwright-go driver above (the go/python playwright
# versions must be identical or Connect fails with a version-mismatch handshake).
# 1.49.1 is also the last line that still ships the node driver's
# lib/browserServerImpl.js, which camoufox's server launcher require()s and
# playwright 1.50+ removed. The playwright CLI apt-installs the Firefox system
# libs via install-deps. camoufox runs as root: Firefox (unlike Chromium) does
# not refuse its sandbox as root, so no extra user is needed, and `camoufox fetch`
# can write its geoip db into site-packages.
RUN pip install --no-cache-dir "camoufox[geoip]" "playwright==1.49.1" \
    && playwright install-deps firefox \
    && python -m camoufox fetch \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip

# Launch wrapper for the camoufox server sidecar. The `server` CLI subcommand
# takes no flags, and it can't run in a container as-is: it defaults to headed
# (no DISPLAY) and pulls the uBlock addon from Mozilla (a download that fails and
# is useless for scraping). launch_server() takes those as kwargs, so wrap it.
# A wrapper (not an inline command) because AS24_CAMOUFOX_CMD is split on spaces.
COPY <<'EOF' /usr/local/bin/camoufox-server
#!/bin/sh
exec python -c "from camoufox.server import launch_server; from camoufox.addons import DefaultAddons; launch_server(headless=True, exclude_addons=[DefaultAddons.UBO])"
EOF
RUN chmod +x /usr/local/bin/camoufox-server

# tini as PID 1 to reap zombies. When the idle timer reaps the camoufox sidecar,
# its node/firefox grandchildren reparent to PID 1; the Go server does not reap
# them, so without an init they pile up as zombies across reap/restart cycles.
RUN apt-get update \
    && apt-get install -y --no-install-recommends tini \
    && rm -rf /var/lib/apt/lists/*

# playwright-go driver (1.49.1), where playwright.Run() looks for it at runtime.
COPY --from=build /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=build /autoscout24-mcp /usr/local/bin/autoscout24-mcp

ENTRYPOINT ["tini", "--", "autoscout24-mcp"]
