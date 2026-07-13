# syntax=docker/dockerfile:1
# ---- build the Go binary + fetch the playwright-go driver ----
FROM golang:1.26-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Bake the playwright-go driver (node + playwright 1.60.0) into the image so the
# runtime never fetches it on first use. The version MUST match the go.mod
# playwright-go dependency (v0.6000.0 → driver 1.60.0) AND the runtime python
# playwright below: playwright refuses a Connect across a version skew.
# PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD keeps this to the driver only — we connect to
# camoufox's own patched Firefox, never a plain Playwright browser.
# Ordered before `COPY . .` so a source change doesn't bust this (slow) layer.
RUN PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 go run github.com/playwright-community/playwright-go/cmd/playwright@v0.6000.0 install firefox
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoscout24-mcp ./cmd/autoscout24-mcp
# camoufox-smoke: a one-shot connect check used only by the smoke image stage.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /camoufox-smoke ./cmd/camoufox-smoke

# ---- runtime base: python + camoufox stealth browser + driver + binary ----
# Shared by both the shippable `runtime` image and the CI-only `smoke` image.
FROM python:3.14-slim-trixie AS runtime-base
ENV PYTHONUNBUFFERED=1 \
    AS24_CAMOUFOX_CMD="xvfb-run -a camoufox-server"

# camoufox runs Firefox headful for stealth (headless is more bot-detectable), so
# it needs an X server: xvfb provides a virtual display and AS24_CAMOUFOX_CMD wraps
# the launch in xvfb-run. tini is PID 1 to reap the camoufox/Firefox process tree
# the server relaunches across idle cycles.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates xvfb xauth tini \
    && rm -rf /var/lib/apt/lists/*

# camoufox is pinned to a git commit for a reproducible launcher: the published
# camoufox 0.4.11 ships an older Firefox and cannot host playwright 1.60. The
# python playwright pin MUST match the playwright-go driver (v0.6000.0 → 1.60.0)
# — the camoufox server and the Go client must agree on the playwright protocol.
# `install-deps firefox` apt-installs the Firefox system libs. git is needed only
# to clone camoufox for the git+https pip install, so it is purged in the same
# layer and never ships.
# NOTE: the git pin fixes the *launcher* only — `camoufox fetch` below still
# downloads the newest stable camoufox browser release at build time, so the
# shipped Firefox version can change on a cache-busted rebuild.
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && pip install --no-cache-dir \
        "playwright==1.60.0" \
        "camoufox[geoip] @ git+https://github.com/daijro/camoufox.git@f342c20dd23736b210f4d5fa4d8b073ee877c9d6#subdirectory=pythonlib" \
    && playwright install-deps firefox \
    && apt-get purge -y --auto-remove git \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip

# Download camoufox's patched Firefox into the image. `camoufox fetch` swallows
# download/sync failures and still exits 0, which would silently bake an image
# with no browser; assert the install so a failed fetch fails the build loudly.
# Ordered before the shim below so editing the shim doesn't re-run this large
# download.
RUN camoufox fetch \
    && python -c "from camoufox.pkgman import installed_verstr; print('camoufox browser:', installed_verstr())"

# Playwright 1.50 removed the private lib/browserServerImpl.js that camoufox's
# launcher imports, so the 1.60 driver lacks it; write a shim back into the driver
# so `camoufox server` starts. The shim also strips camoufox's null-valued
# top-level options (e.g. proxy: null), which 1.60's launch validators reject
# while older drivers tolerated.
RUN python <<'PY'
from pathlib import Path
import subprocess

from playwright._impl._driver import compute_driver_executable

nodejs, cli = compute_driver_executable()
driver_package = Path(cli).parent
shim = driver_package / "lib" / "browserServerImpl.js"
shim.write_text("""'use strict';

const playwright = require('..');
const RealBrowserServerLauncherImpl = playwright.firefox._serverLauncher.constructor;

class BrowserServerLauncherImpl extends RealBrowserServerLauncherImpl {
    async launchServer(options = {}) {
        for (const [key, value] of Object.entries(options)) {
            if (value === null)
                delete options[key];
        }
        return await super.launchServer(options);
    }
}

module.exports = { BrowserServerLauncherImpl };
""")
subprocess.run(
    [
        nodejs,
        "-e",
        "const { BrowserServerLauncherImpl } = require('./lib/browserServerImpl.js');"
        "const launcher = new BrowserServerLauncherImpl('firefox');"
        "if (typeof launcher.launchServer !== 'function') process.exit(1);",
    ],
    cwd=driver_package,
    check=True,
)
PY

# Launch wrapper for the camoufox sidecar. Run headful (stealth) but exclude the
# uBlock Origin default addon: it is useless for scraping and its filter lists
# change the pages AutoScout24 serves. The bare `camoufox server` CLI takes no
# flags and loads every default addon, so wrap launch_server(), which accepts
# exclude_addons. A script (not an inline command) because AS24_CAMOUFOX_CMD is
# split on spaces.
COPY <<'EOF' /usr/local/bin/camoufox-server
#!/bin/sh
exec python -c "from camoufox.server import launch_server; from camoufox.addons import DefaultAddons; launch_server(headless=False, exclude_addons=[DefaultAddons.UBO])"
EOF
RUN chmod +x /usr/local/bin/camoufox-server

# playwright-go driver (1.60.0), where playwright.Run() looks for it at runtime.
COPY --from=build /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=build /autoscout24-mcp /usr/local/bin/autoscout24-mcp

# ---- CI-only target: prove camoufox actually connects (not shipped to prod) ----
# Declared BEFORE `runtime` so it is never `docker build`'s default (last) stage.
# Build with `--target smoke` and run it: it launches the camoufox sidecar,
# connects the playwright-go driver, and navigates a page — a real Connect that
# fails on a version skew. The release and camoufox-smoke workflows build this.
FROM runtime-base AS smoke
COPY --from=build /camoufox-smoke /usr/local/bin/camoufox-smoke
ENTRYPOINT ["tini", "--", "camoufox-smoke"]

# ---- release image: the shippable MCP server ----
# Last stage on purpose, so a bare `docker build .` (no --target) produces this,
# not the smoke image. The release workflow also passes --target runtime.
FROM runtime-base AS runtime
ENTRYPOINT ["tini", "--", "autoscout24-mcp"]
