package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// minInterval is the minimum spacing between consecutive requests.
const minInterval = 300 * time.Millisecond

// HTTPFetcher fetches pages with plain HTTP and browser-like headers.
type HTTPFetcher struct {
	client *http.Client

	mu   sync.Mutex // guards last; also reserves request slots under concurrency
	last time.Time
}

// NewHTTPFetcher builds an HTTP fetcher. A zero timeout means no client timeout.
func NewHTTPFetcher(timeout time.Duration) *HTTPFetcher {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err) // cookiejar.New(nil) cannot fail per its contract
	}
	return &HTTPFetcher{client: &http.Client{Jar: jar, Timeout: timeout}}
}

// throttle reserves the next polite request slot and waits for it, so
// concurrent callers space out rather than racing on last.
func (f *HTTPFetcher) throttle(ctx context.Context) error {
	f.mu.Lock()
	now := time.Now()
	prev := f.last // restore target if we roll back a canceled reservation
	var wait time.Duration
	if next := f.last.Add(minInterval); next.After(now) {
		wait = next.Sub(now) + time.Duration(rand.IntN(500))*time.Millisecond //nolint:gosec // politeness jitter, not security-sensitive
	}
	reserved := now.Add(wait)
	f.last = reserved // reserve the slot before releasing the lock
	f.mu.Unlock()

	if wait == 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		// Canceled before sending: release our reservation so it doesn't make
		// later real requests wait behind a request that never happened. Only
		// roll back if no one queued behind us in the meantime, and restore the
		// prior reservation (not now) — otherwise a still-pending earlier slot is
		// lost and the next request can fire within minInterval of it.
		f.mu.Lock()
		if f.last.Equal(reserved) {
			f.last = prev
		}
		f.mu.Unlock()
		return ctx.Err()
	}
}

func (f *HTTPFetcher) Get(ctx context.Context, url string) (p *Page, err error) {
	if terr := f.throttle(ctx); terr != nil {
		return nil, terr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	p = &Page{URL: resp.Request.URL.String(), Status: resp.StatusCode, Body: body}
	if serr := classifyStatus(p, url, ""); serr != nil {
		return nil, serr
	}
	if p.Status >= 400 {
		return nil, fmt.Errorf("%s: unexpected status %d", url, p.Status)
	}
	return p, nil
}

// classifyStatus maps a fetched page to a sentinel error, or nil if it looks
// like real content. via labels the stage in the block message (e.g.
// " (even via camoufox)"); pass "" for the primary stage.
func classifyStatus(p *Page, url, via string) error {
	switch {
	case p.Status == http.StatusNotFound || p.Status == http.StatusGone:
		return fmt.Errorf("%s: %w", url, ErrNotFound)
	case IsBlocked(p):
		return fmt.Errorf("%s: %w%s", url, ErrBlocked, via)
	}
	return nil
}

var blockMarkers = [][]byte{
	[]byte("Just a moment..."),
	[]byte("cf-challenge"),
	[]byte("Access to this page has been denied"),
	[]byte("px-captcha"),
}

// IsBlocked reports whether a page looks like an anti-bot block or challenge.
func IsBlocked(p *Page) bool {
	if p.Status == http.StatusForbidden || p.Status == http.StatusTooManyRequests {
		return true
	}
	for _, m := range blockMarkers {
		if bytes.Contains(p.Body, m) {
			return true
		}
	}
	// Soft block: every AutoScout24 content page embeds __NEXT_DATA__. A 200
	// response without it is an interstitial/challenge, not real content — so
	// escalate rather than surfacing it downstream as an opaque parse failure.
	if p.Status == http.StatusOK && !bytes.Contains(p.Body, []byte("__NEXT_DATA__")) {
		return true
	}
	return false
}
