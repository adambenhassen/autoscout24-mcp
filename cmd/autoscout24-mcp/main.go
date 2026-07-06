// Command autoscout24-mcp serves AutoScout24 data over MCP (stdio, and
// optionally streamable HTTP when AS24_HTTP_ADDR is set).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adambenhassen/autoscout24-mcp/internal/as24"
	"github.com/adambenhassen/autoscout24-mcp/internal/config"
	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
	"github.com/adambenhassen/autoscout24-mcp/internal/mcpserver"
)

const blockCooldown = 10 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	var camoufox *fetch.CamoufoxFetcher
	stages := make([]fetch.Stage, 0, len(cfg.Fetchers))
	for _, name := range cfg.Fetchers {
		switch name {
		case "http":
			stages = append(stages, fetch.Stage{Name: name, Fetcher: fetch.NewHTTPFetcher(cfg.Timeout)})
		case "camoufox":
			camoufox = fetch.NewCamoufoxFetcher(cfg.CamoufoxCmd, cfg.Timeout)
			stages = append(stages, fetch.Stage{Name: name, Fetcher: camoufox})
		case "crw":
			if cfg.CRWURL == "" {
				stages = append(stages, fetch.Stage{Name: name, Fetcher: nil}) // unconfigured: instructive error if reached
				continue
			}
			stages = append(stages, fetch.Stage{Name: name, Fetcher: fetch.NewCRWFetcher(cfg.CRWURL, cfg.CRWAPIKey, cfg.Timeout)})
		}
	}
	svc := as24.New(fetch.NewEscalating(stages, blockCooldown), cfg.Market)
	server := mcpserver.New(svc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer func() {
		if camoufox != nil {
			if cerr := camoufox.Close(); cerr != nil {
				log.Printf("camoufox shutdown: %v", cerr)
			}
		}
	}()

	httpErr := make(chan error, 1)
	if cfg.HTTPAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		httpSrv := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			log.Printf("streamable HTTP transport on %s", cfg.HTTPAddr)
			if serr := httpSrv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
				httpErr <- serr
				stop() // unblock the stdio Run below so the process can exit non-zero
			}
		}()
		defer func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if serr := httpSrv.Shutdown(shutCtx); serr != nil {
				log.Printf("http shutdown: %v", serr)
			}
		}()
	}

	err = server.Run(ctx, &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		return err
	}
	// With no stdin (e.g. containers) the stdio transport ends immediately; if the
	// HTTP transport is enabled, keep serving until a shutdown signal — but if the
	// HTTP server itself failed (e.g. the address was already bound), surface that
	// error so the process exits non-zero instead of parking with no live transport.
	if cfg.HTTPAddr != "" {
		<-ctx.Done()
		select {
		case serr := <-httpErr:
			return serr
		default:
		}
	}
	return nil
}
