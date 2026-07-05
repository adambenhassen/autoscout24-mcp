package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// HTTPFetcher fetches pages with plain HTTP and browser-like headers.
type HTTPFetcher struct {
	client *http.Client
	last   time.Time
}

func NewHTTPFetcher() *HTTPFetcher {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err) // cookiejar.New(nil) cannot fail per its contract
	}
	return &HTTPFetcher{client: &http.Client{Jar: jar}}
}

func (f *HTTPFetcher) Get(ctx context.Context, url string) (*Page, error) {
	// polite jitter between consecutive requests: 300-800ms
	if since := time.Since(f.last); since < 300*time.Millisecond {
		select {
		case <-time.After(300*time.Millisecond - since + time.Duration(rand.IntN(500))*time.Millisecond): //nolint:gosec // politeness jitter, not security-sensitive
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.last = time.Now()

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
	p := &Page{URL: resp.Request.URL.String(), Status: resp.StatusCode, Body: body}
	switch {
	case p.Status == http.StatusNotFound || p.Status == http.StatusGone:
		return nil, fmt.Errorf("%s: %w", url, ErrNotFound)
	case IsBlocked(p):
		return nil, fmt.Errorf("%s: %w", url, ErrBlocked)
	case p.Status >= 400:
		return nil, fmt.Errorf("%s: unexpected status %d", url, p.Status)
	}
	return p, nil
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
	return false
}
