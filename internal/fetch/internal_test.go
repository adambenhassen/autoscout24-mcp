package fetch

import "testing"

func TestParseWSEndpoint(t *testing.T) {
	line := "camoufox server listening on ws://127.0.0.1:9222/abc"
	got, ok := parseWSEndpoint(line)
	if !ok || got != "ws://127.0.0.1:9222/abc" {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := parseWSEndpoint("no endpoint here"); ok {
		t.Fatal("false positive")
	}
}
