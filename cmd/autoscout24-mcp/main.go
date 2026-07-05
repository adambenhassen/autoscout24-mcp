// Command autoscout24-mcp serves AutoScout24 data over MCP.
package main

import (
	"fmt"
	"os"

	"github.com/adam/autoscout24-mcp/internal/config"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = cfg // wired to the MCP server in a later task
}
