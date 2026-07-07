//go:build e2e

package main_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestE2ESmoke exercises the shipped artifact end to end: it builds the binary,
// runs it as a process with stdin closed (as a container does) and only the
// streamable HTTP transport enabled, then drives a real MCP handshake over HTTP
// and asserts every tool is registered. It stops short of calling a tool — that
// would hit the live AutoScout24 site and make the smoke test flaky. The value
// here is proving the process boots, survives stdin EOF, and serves MCP.
func TestE2ESmoke(t *testing.T) {
	addr := freeAddr(t)
	endpoint := "http://" + addr

	bin := filepath.Join(t.TempDir(), "autoscout24-mcp")
	build := exec.Command("go", "build", "-o", bin, "./cmd/autoscout24-mcp")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build binary: %v", err)
	}

	srv := exec.Command(bin)
	srv.Env = append(os.Environ(),
		"AS24_HTTP_ADDR="+addr,
		"AS24_FETCHERS=http", // no camoufox binary in CI; boot never needs it
	)
	srv.Stdin = nil // closed stdin: the stdio transport hits EOF immediately
	srv.Stdout, srv.Stderr = os.Stderr, os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() { stopProcess(srv) })

	waitPort(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-smoke", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("mcp connect %s: %v", endpoint, err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	want := map[string]bool{
		"search_listings": false, "get_listing": false, "price_analysis": false,
		"get_dealer": false, "list_makes_models": false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// freeAddr reserves a loopback port and hands back its address. There is a small
// window between closing the listener and the server binding it; the waitPort
// poll below absorbs the normal startup delay.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func waitPort(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("server never listened on %s", addr)
}

func stopProcess(srv *exec.Cmd) {
	_ = srv.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _ = srv.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = srv.Process.Kill()
		<-done
	}
}
