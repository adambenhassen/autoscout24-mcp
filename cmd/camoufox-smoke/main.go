// Command camoufox-smoke verifies the camoufox fetch stage from inside the
// runtime image: it launches the camoufox sidecar, connects the playwright-go
// driver to it, navigates to a page, and reports the result. A version skew
// between the Go driver and the python playwright/camoufox surfaces here as a
// launch/Connect failure — or as a broken first protocol command after the
// Connect — so this is the check that proves the browser stack actually works,
// not just that the pins look compatible on paper.
//
// It fetches SMOKE_URL (default https://example.com), a neutral always-up page,
// so it exercises launch + Connect + navigate without depending on a live
// AutoScout24 fetch, which CI datacenter IPs can get anti-bot blocked on. Point
// SMOKE_URL at an AutoScout24 URL to additionally exercise the real target (the
// content verdict is printed, not asserted).
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "camoufox smoke FAILED: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	url := cmp.Or(os.Getenv("SMOKE_URL"), "https://example.com")
	// AS24_CAMOUFOX_CMD is supplied by the runtime image's ENV; leaving it empty
	// falls back to the fetcher's own default so the smoke launches camoufox
	// exactly as the server binary would on the same host.
	cmd := os.Getenv("AS24_CAMOUFOX_CMD")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	f := fetch.NewCamoufoxFetcher(cmd, 60*time.Second)
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "camoufox close: %v\n", cerr)
		}
	}()

	page, err := f.Get(ctx, url)
	switch {
	case err == nil:
		fmt.Printf("camoufox smoke OK: %s -> status %d, %d bytes\n", page.URL, page.Status, len(page.Body))
		return nil
	case errors.Is(err, fetch.ErrBlocked), errors.Is(err, fetch.ErrNotFound):
		// The page was fetched and its content read; only the AutoScout24-tuned
		// content heuristic fired (expected on a neutral URL like example.com).
		// That still proves launch + Connect + navigate + read all worked.
		fmt.Printf("camoufox smoke OK: fetched %s (content verdict: %v)\n", url, err)
		return nil
	default:
		// ErrUnavailable is the launch/Connect failure this check exists to catch;
		// every other error (NewPage, Goto, Content, context deadline) is a
		// post-connect failure where no page came back. Both mean the stack is
		// broken — fail so the weekly smoke and the release gate go red.
		return err
	}
}
