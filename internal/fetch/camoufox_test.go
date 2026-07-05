package fetch_test

import (
	"context"
	"strings"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

func TestCamoufoxUnavailableError(t *testing.T) {
	f := fetch.NewCamoufoxFetcher("/nonexistent/binary --definitely-missing")
	_, err := f.Get(context.Background(), "https://x.test")
	if err == nil || !strings.Contains(err.Error(), "camoufox unavailable") {
		t.Fatalf("want unavailable error, got %v", err)
	}
	// error must be sticky (Once)
	_, err2 := f.Get(context.Background(), "https://x.test")
	if err2 == nil || !strings.Contains(err2.Error(), "camoufox unavailable") {
		t.Fatalf("want sticky error, got %v", err2)
	}
}
