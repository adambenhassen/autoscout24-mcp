// Command capture-fixtures downloads live AutoScout24 pages into internal/parser/testdata.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: capture-fixtures <url> <outfile>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(url, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := fetch.NewHTTPFetcher(0).Get(ctx, url)
	if err != nil {
		return err
	}
	out := filepath.Join("internal", "parser", "testdata", filepath.Base(name))
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(out, p.Body, 0o644); err != nil { //nolint:gosec // test fixture, world-readable is fine
		return err
	}
	fmt.Printf("saved %d bytes to %s\n", len(p.Body), out)
	return nil
}
