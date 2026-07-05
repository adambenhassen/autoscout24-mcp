package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CRWFetcher scrapes via a Firecrawl-compatible HTTP API (crw).
type CRWFetcher struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewCRWFetcher(baseURL, apiKey string) *CRWFetcher {
	return &CRWFetcher{baseURL: baseURL, apiKey: apiKey, client: http.DefaultClient}
}

type crwResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Data    struct {
		HTML     string `json:"html"`
		Metadata struct {
			StatusCode int    `json:"statusCode"`
			URL        string `json:"url"`
		} `json:"metadata"`
	} `json:"data"`
}

func (f *CRWFetcher) Get(ctx context.Context, url string) (*Page, error) {
	payload, err := json.Marshal(map[string]any{
		"url": url, "formats": []string{"html"}, "renderJs": true,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/v1/scrape", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	var cr crwResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("crw: decoding response: %w", err)
	}
	if !cr.Success {
		return nil, fmt.Errorf("crw: scrape failed: %s", cr.Error)
	}
	p := &Page{URL: cr.Data.Metadata.URL, Status: cr.Data.Metadata.StatusCode, Body: []byte(cr.Data.HTML)}
	if p.URL == "" {
		p.URL = url
	}
	switch {
	case p.Status == http.StatusNotFound || p.Status == http.StatusGone:
		return nil, fmt.Errorf("%s: %w", url, ErrNotFound)
	case IsBlocked(p):
		return nil, fmt.Errorf("%s: %w (even via crw)", url, ErrBlocked)
	}
	return p, nil
}
