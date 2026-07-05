package config_test

import (
	"testing"
	"time"

	"github.com/adam/autoscout24-mcp/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != "de" || cfg.Timeout != 30*time.Second {
		t.Fatalf("bad defaults: %+v", cfg)
	}
	if len(cfg.Fetchers) != 2 || cfg.Fetchers[0] != "http" || cfg.Fetchers[1] != "camoufox" {
		t.Fatalf("bad fetchers: %v", cfg.Fetchers)
	}
}

func TestLoadRejectsBadMarket(t *testing.T) {
	env := map[string]string{"AS24_MARKET": "xx"}
	if _, err := config.Load(func(k string) string { return env[k] }); err == nil {
		t.Fatal("want error for bad market")
	}
}

func TestLoadRejectsBadFetcher(t *testing.T) {
	env := map[string]string{"AS24_FETCHERS": "http,teleport"}
	if _, err := config.Load(func(k string) string { return env[k] }); err == nil {
		t.Fatal("want error for bad fetcher")
	}
}

func TestLoadOverrides(t *testing.T) {
	env := map[string]string{
		"AS24_MARKET":   "it",
		"AS24_FETCHERS": "http,crw",
		"AS24_TIMEOUT":  "5s",
		"CRW_URL":       "http://localhost:3002",
	}
	cfg, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != "it" || cfg.Timeout != 5*time.Second || cfg.CRWURL != "http://localhost:3002" {
		t.Fatalf("bad overrides: %+v", cfg)
	}
	if len(cfg.Fetchers) != 2 || cfg.Fetchers[1] != "crw" {
		t.Fatalf("bad fetchers: %v", cfg.Fetchers)
	}
}
