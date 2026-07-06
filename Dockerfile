# ---- build the Go binary + fetch the matching playwright-go driver ----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoscout24-mcp ./cmd/autoscout24-mcp
# Download the playwright-go driver (node + playwright 1.60.0) so the runtime can
# start playwright and connect to the camoufox browser. Pinned to the go.mod
# playwright-go version; the firefox download here is discarded (camoufox ships
# its own patched Firefox) — we keep only ~/.cache/ms-playwright-go.
RUN go run github.com/playwright-community/playwright-go/cmd/playwright@v0.6000.0 install firefox

# ---- runtime: python + camoufox stealth browser + the driver + the binary ----
FROM python:3.12-slim-bookworm
ENV PYTHONUNBUFFERED=1 \
    AS24_CAMOUFOX_CMD="python -m camoufox server"

# Firefox runtime system libraries (camoufox is a patched Firefox), camoufox
# itself, and its browser + geoip data. The pip playwright package is used only
# to apt-install the Firefox deps, then removed. camoufox runs as root: Firefox
# (unlike Chromium) does not refuse its sandbox as root, so no extra user is
# needed, and `camoufox fetch` can write its geoip db into site-packages.
RUN pip install --no-cache-dir playwright \
    && playwright install-deps firefox \
    && pip uninstall -y playwright \
    && pip install --no-cache-dir "camoufox[geoip]" \
    && python -m camoufox fetch \
    && rm -rf /var/lib/apt/lists/* /root/.cache/pip

# playwright-go driver (v1.60.0), where playwright.Run() looks for it at runtime.
COPY --from=build /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=build /autoscout24-mcp /usr/local/bin/autoscout24-mcp

ENTRYPOINT ["autoscout24-mcp"]
