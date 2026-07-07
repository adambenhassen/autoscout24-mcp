# syntax=docker/dockerfile:1
# ---- build the Go binary + fetch the matching playwright-go driver ----
FROM golang:1.26-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Download the playwright-go driver (node + playwright 1.60.0) so the runtime can
# start playwright and connect to the camoufox browser. The version MUST match
# the go.mod playwright-go dependency (v0.6000.0 → driver 1.60.0) AND the runtime
# python playwright below: playwright refuses a Connect across a version skew.
# PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD keeps this to the driver only — we connect to
# camoufox's own patched Firefox, never a plain Playwright browser.
# Ordered before `COPY . .` so a source change doesn't bust this (slow) layer.
RUN PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 go run github.com/playwright-community/playwright-go/cmd/playwright@v0.6000.0 install firefox
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoscout24-mcp ./cmd/autoscout24-mcp
# camoufox-smoke: a one-shot connect check used only by the smoke image stage.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /camoufox-smoke ./cmd/camoufox-smoke

# ---- runtime: python + camoufox stealth browser + the driver + the binary ----
FROM python:3.14-slim-trixie AS runtime
ENV PYTHONUNBUFFERED=1 \
    AS24_CAMOUFOX_CMD="/usr/local/bin/camoufox-server"

# camoufox (a patched Firefox) plus its browser + geoip data. camoufox is pinned
# for reproducible builds; its pyproject caps playwright at <1.61. playwright is
# pinned to 1.60.0 to match the playwright-go driver above — the go/python
# playwright versions must be identical or Connect fails with a version-mismatch
# handshake. The playwright CLI apt-installs the Firefox system libs via
# install-deps. camoufox runs as root: Firefox (unlike Chromium) does not refuse
# its sandbox as root, so no extra user is needed, and `camoufox fetch` can write
# its geoip db into site-packages.
RUN pip install --no-cache-dir "camoufox[geoip]==0.4.11" "playwright==1.60.0" \
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

# playwright-go driver (1.60.0), where playwright.Run() looks for it at runtime.
COPY --from=build /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=build /autoscout24-mcp /usr/local/bin/autoscout24-mcp

ENTRYPOINT ["tini", "--", "autoscout24-mcp"]

# ---- CI-only target: prove camoufox actually connects (not shipped to prod) ----
# Extends the runtime image with the smoke binary. Build with `--target smoke`
# and run it: it launches the camoufox sidecar, connects the playwright-go driver,
# and navigates a page — a real Connect that fails on a Go/python/browser version
# skew. The default (release) build targets `runtime`, so this never ships.
FROM runtime AS smoke
COPY --from=build /camoufox-smoke /usr/local/bin/camoufox-smoke
ENTRYPOINT ["tini", "--", "camoufox-smoke"]
