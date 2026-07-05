package fetch_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

// okBody is a minimal AutoScout24-shaped page: the __NEXT_DATA__ marker keeps
// it from tripping the soft-block detector.
const okBody = `<html><script id="__NEXT_DATA__" type="application/json">{}</script>ok</html>`

func TestHTTPFetcherGetOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept-Language") == "" {
			t.Error("missing browser-like headers")
		}
		if _, err := w.Write([]byte(okBody)); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	p, err := fetch.NewHTTPFetcher(0).Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != 200 || !strings.Contains(string(p.Body), "ok") {
		t.Fatalf("bad page: %+v", p)
	}
}

func TestHTTPFetcherConcurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(okBody)); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	f := fetch.NewHTTPFetcher(0)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := f.Get(context.Background(), srv.URL); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestHTTPFetcher404(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := fetch.NewHTTPFetcher(0).Get(context.Background(), srv.URL)
	if !errors.Is(err, fetch.ErrNotFound) {
		t.Fatalf("want fetch.ErrNotFound, got %v", err)
	}
}

func TestHTTPFetcher403IsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	_, err := fetch.NewHTTPFetcher(0).Get(context.Background(), srv.URL)
	if !errors.Is(err, fetch.ErrBlocked) {
		t.Fatalf("want fetch.ErrBlocked, got %v", err)
	}
}

func TestIsBlockedChallengeMarkers(t *testing.T) {
	for _, body := range []string{
		"<title>Just a moment...</title>",
		"cf-challenge-running",
		"Access to this page has been denied",
	} {
		if !fetch.IsBlocked(&fetch.Page{Status: 200, Body: []byte(body)}) {
			t.Errorf("want blocked for %q", body)
		}
	}
	if fetch.IsBlocked(&fetch.Page{Status: 200, Body: []byte(okBody)}) {
		t.Error("false positive on a normal page")
	}
	// A 200 page missing __NEXT_DATA__ is a soft block and must be flagged.
	if !fetch.IsBlocked(&fetch.Page{Status: 200, Body: []byte("<html>interstitial</html>")}) {
		t.Error("want soft-block for 200 page without __NEXT_DATA__")
	}
}
