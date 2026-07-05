package fetch_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

func TestCamoufoxUnavailableError(t *testing.T) {
	f := fetch.NewCamoufoxFetcher("/nonexistent/binary --definitely-missing", 0)
	_, err := f.Get(context.Background(), "https://x.test")
	// ErrUnavailable so the escalation chain skips to the next stage.
	if !errors.Is(err, fetch.ErrUnavailable) || !strings.Contains(err.Error(), "camoufox") {
		t.Fatalf("want camoufox ErrUnavailable, got %v", err)
	}
	// error must be sticky (Once)
	_, err2 := f.Get(context.Background(), "https://x.test")
	if !errors.Is(err2, fetch.ErrUnavailable) {
		t.Fatalf("want sticky ErrUnavailable, got %v", err2)
	}
}

func TestCamoufoxEmptyCmdUnavailable(t *testing.T) {
	f := fetch.NewCamoufoxFetcher("   ", 0) // whitespace-only: must not panic
	if _, err := f.Get(context.Background(), "https://x.test"); !errors.Is(err, fetch.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable for empty cmd, got %v", err)
	}
}
