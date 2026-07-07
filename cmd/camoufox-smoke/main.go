// Command camoufox-smoke verifies the camoufox fetch stage from inside the
// runtime image: it launches the camoufox sidecar, connects the playwright-go
// driver to it, navigates to a page, and reports the result. A version skew
// between the Go driver and the python playwright/camoufox surfaces here as a
// Connect failure — the exact risk a playwright bump carries — so this is the
// check that proves the browser stack actually works, not just that the pins
// look compatible on paper.
//
// It fetches SMOKE_URL (default https://example.com), a neutral always-up page,
// so it exercises launch + Connect + navigate without depending on a live
// AutoScout24 fetch, which CI datacenter IPs can get anti-bot blocked on. Point
// SMOKE_URL at an AutoScout24 URL to additionally test the real target.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

func main() {
	url := getenv("SMOKE_URL", "https://example.com")
	cmd := getenv("AS24_CAMOUFOX_CMD", "/usr/local/bin/camoufox-server")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	f := fetch.NewCamoufoxFetcher(cmd, 60*time.Second)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "camoufox close: %v\n", cerr)
		}
	}()

	page, err := f.Get(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "camoufox smoke FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("camoufox smoke OK: %s -> status %d, %d bytes\n", page.URL, page.Status, len(page.Body))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
