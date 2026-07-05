package fetch_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

func TestCRWFetcherOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/scrape" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["url"] != "https://x.test/a" {
			t.Errorf("bad url in body: %v", body["url"])
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"html":     `<html><script id="__NEXT_DATA__">{}</script>via crw</html>`,
				"metadata": map[string]any{"statusCode": 200, "url": "https://x.test/a"},
			},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	p, err := fetch.NewCRWFetcher(srv.URL, "k", 0).Get(context.Background(), "https://x.test/a")
	if err != nil || !strings.Contains(string(p.Body), "via crw") || p.Status != 200 {
		t.Fatalf("got %+v, %v", p, err)
	}
}

func TestCRWFetcherBlockedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"html": "denied", "metadata": map[string]any{"statusCode": 403}},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	_, err := fetch.NewCRWFetcher(srv.URL, "k", 0).Get(context.Background(), "https://x.test/a")
	if !errors.Is(err, fetch.ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

func TestCRWFetcherFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "boom"}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	if _, err := fetch.NewCRWFetcher(srv.URL, "k", 0).Get(context.Background(), "https://x.test/a"); err == nil {
		t.Fatal("want error")
	}
}
