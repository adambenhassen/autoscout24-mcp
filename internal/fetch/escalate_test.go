package fetch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

type fakeFetcher struct {
	calls int
	err   error
}

func (f *fakeFetcher) Get(_ context.Context, url string) (*fetch.Page, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &fetch.Page{URL: url, Status: 200, Body: []byte("ok")}, nil
}

func TestEscalateOnBlock(t *testing.T) {
	blocked := &fakeFetcher{err: fetch.ErrBlocked}
	ok := &fakeFetcher{}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "http", Fetcher: blocked}, {Name: "camoufox", Fetcher: ok}}, time.Minute)
	p, err := e.Get(context.Background(), "https://x.test/a")
	if err != nil || string(p.Body) != "ok" {
		t.Fatalf("got %v, %v", p, err)
	}
	// cooldown: second request skips the blocked stage
	if _, err := e.Get(context.Background(), "https://x.test/b"); err != nil {
		t.Fatal(err)
	}
	if blocked.calls != 1 {
		t.Fatalf("blocked stage called %d times, want 1 (cooldown)", blocked.calls)
	}
}

func TestCooldownExpiry(t *testing.T) {
	blocked := &fakeFetcher{err: fetch.ErrBlocked}
	ok := &fakeFetcher{}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "http", Fetcher: blocked}, {Name: "camoufox", Fetcher: ok}}, time.Nanosecond)
	if _, err := e.Get(context.Background(), "https://x.test/a"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := e.Get(context.Background(), "https://x.test/b"); err != nil {
		t.Fatal(err)
	}
	if blocked.calls != 2 {
		t.Fatalf("blocked stage called %d times, want 2 (cooldown expired)", blocked.calls)
	}
}

func TestNilStageUnavailableError(t *testing.T) {
	// When no stage can operate (unavailable + unconfigured), the error is
	// ErrUnavailable and names the unconfigured stage — not a false "blocked".
	unavailable := &fakeFetcher{err: fetch.ErrUnavailable}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "camoufox", Fetcher: unavailable}, {Name: "crw", Fetcher: nil}}, time.Minute)
	_, err := e.Get(context.Background(), "https://x.test/a")
	if !errors.Is(err, fetch.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if errors.Is(err, fetch.ErrBlocked) {
		t.Fatalf("must not mislabel unavailable stages as blocked: %v", err)
	}
}

func TestBlockedWinsOverUnavailable(t *testing.T) {
	// A genuine anti-bot block is the accurate, actionable error; it must win
	// over a later unavailable/unconfigured stage.
	blocked := &fakeFetcher{err: fetch.ErrBlocked}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "http", Fetcher: blocked}, {Name: "camoufox", Fetcher: nil}}, time.Minute)
	_, err := e.Get(context.Background(), "https://x.test/a")
	if !errors.Is(err, fetch.ErrBlocked) {
		t.Fatalf("want ErrBlocked to win, got %v", err)
	}
}

func TestUnavailableStageEscalates(t *testing.T) {
	// A stage that can't operate (ErrUnavailable) must be skipped, not abort the chain.
	unavailable := &fakeFetcher{err: fetch.ErrUnavailable}
	ok := &fakeFetcher{}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "camoufox", Fetcher: unavailable}, {Name: "crw", Fetcher: ok}}, time.Minute)
	p, err := e.Get(context.Background(), "https://x.test/a")
	if err != nil || string(p.Body) != "ok" {
		t.Fatalf("got %v, %v", p, err)
	}
}

func TestNilStageSkipsToNext(t *testing.T) {
	// An unconfigured (nil) stage before a working one must be skipped.
	ok := &fakeFetcher{}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "crw", Fetcher: nil}, {Name: "http", Fetcher: ok}}, time.Minute)
	p, err := e.Get(context.Background(), "https://x.test/a")
	if err != nil || string(p.Body) != "ok" {
		t.Fatalf("got %v, %v", p, err)
	}
}

func TestNonBlockErrorNoEscalation(t *testing.T) {
	nf := &fakeFetcher{err: fetch.ErrNotFound}
	next := &fakeFetcher{}
	e := fetch.NewEscalating([]fetch.Stage{{Name: "http", Fetcher: nf}, {Name: "camoufox", Fetcher: next}}, time.Minute)
	if _, err := e.Get(context.Background(), "https://x.test/a"); !errors.Is(err, fetch.ErrNotFound) {
		t.Fatal("want ErrNotFound passthrough")
	}
	if next.calls != 0 {
		t.Fatal("must not escalate on non-block errors")
	}
}
