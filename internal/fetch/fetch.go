package fetch

import "context"

// Page is a fetched document.
type Page struct {
	URL    string // final URL after redirects
	Status int
	Body   []byte
}

// Fetcher retrieves a page by URL.
type Fetcher interface {
	Get(ctx context.Context, url string) (*Page, error)
}
