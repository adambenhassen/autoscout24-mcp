package fetch_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

func TestHTTPFetcherGetOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept-Language") == "" {
			t.Error("missing browser-like headers")
		}
		if _, err := w.Write([]byte("<html>ok</html>")); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	p, err := fetch.NewHTTPFetcher().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != 200 || string(p.Body) != "<html>ok</html>" {
		t.Fatalf("bad page: %+v", p)
	}
}

func TestHTTPFetcher404(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := fetch.NewHTTPFetcher().Get(context.Background(), srv.URL)
	if !errors.Is(err, fetch.ErrNotFound) {
		t.Fatalf("want fetch.ErrNotFound, got %v", err)
	}
}

func TestHTTPFetcher403IsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	_, err := fetch.NewHTTPFetcher().Get(context.Background(), srv.URL)
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
	if fetch.IsBlocked(&fetch.Page{Status: 200, Body: []byte("<html>normal</html>")}) {
		t.Error("false positive")
	}
}
