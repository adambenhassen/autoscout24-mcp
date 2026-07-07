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

# ---- runtime: python + camoufox (Firefox 150) stealth browser + driver + binary ----
FROM python:3.14-slim-trixie AS runtime
ENV PYTHONUNBUFFERED=1 \
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 \
    AS24_CAMOUFOX_CMD="xvfb-run -a camoufox server"

# camoufox runs Firefox headful for stealth (headless is more bot-detectable), so
# it needs an X server: xvfb provides a virtual display and AS24_CAMOUFOX_CMD wraps
# `camoufox server` in xvfb-run. git is needed for the pip git install below; tini
# is PID 1 to reap the camoufox/Firefox process tree the server relaunches across
# idle cycles.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates git xvfb xauth tini \
    && rm -rf /var/lib/apt/lists/*

# camoufox is pinned to a git commit (the Firefox 150 launcher) for reproducible
# builds: the published camoufox 0.4.11 ships an older Firefox and cannot host
# playwright 1.60. The python playwright pin MUST match the playwright-go driver
# (v0.6000.0 → 1.60.0) — the camoufox server and the Go client must agree on the
# playwright protocol. `install-deps firefox` apt-installs the Firefox system libs.
RUN pip install --no-cache-dir \
        "playwright==1.60.0" \
        "camoufox[geoip] @ git+https://github.com/daijro/camoufox.git@f342c20dd23736b210f4d5fa4d8b073ee877c9d6#subdirectory=pythonlib" \
    && playwright install-deps firefox \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip

# Playwright 1.60 removed the private lib/browserServerImpl.js that camoufox's
# launcher imports; write a shim back into the driver so `camoufox server` starts.
# The shim also strips camoufox's null-valued top-level options (e.g. proxy: null),
# which 1.60's launch validators reject while older drivers tolerated.
RUN python <<'PY'
from pathlib import Path
import subprocess

from playwright._impl._driver import compute_driver_executable

nodejs, cli = compute_driver_executable()
if isinstance(nodejs, tuple):
    nodejs = nodejs[0]
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

# Download camoufox's patched Firefox 150 into the image.
RUN camoufox fetch

# playwright-go driver (1.60.0), where playwright.Run() looks for it at runtime.
COPY --from=build /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=build /autoscout24-mcp /usr/local/bin/autoscout24-mcp

ENTRYPOINT ["tini", "--", "autoscout24-mcp"]

# ---- CI-only target: prove camoufox actually connects (not shipped to prod) ----
# Extends the runtime image with the smoke binary. Build with `--target smoke`
# and run it: it launches the camoufox sidecar, connects the playwright-go driver,
# and navigates a page — a real Connect that fails on a version skew. The default
# (release) build targets `runtime`, so this never ships.
FROM runtime AS smoke
COPY --from=build /camoufox-smoke /usr/local/bin/camoufox-smoke
ENTRYPOINT ["tini", "--", "camoufox-smoke"]
