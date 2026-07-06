// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// validMarkets are the supported flat-structure markets. Belgium (.be) is
// excluded: it requires a /nl/ or /fr/ locale path prefix this scheme lacks.
var validMarkets = []string{"de", "com", "it", "fr", "nl", "at", "es"}
var validFetchers = []string{"http", "camoufox"}

type Config struct {
	Market      string
	Fetchers    []string
	HTTPAddr    string
	CamoufoxCmd string
	Timeout     time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Market:      "de",
		Fetchers:    []string{"http", "camoufox"},
		HTTPAddr:    getenv("AS24_HTTP_ADDR"),
		CamoufoxCmd: getenv("AS24_CAMOUFOX_CMD"),
		Timeout:     30 * time.Second,
	}
	if m := getenv("AS24_MARKET"); m != "" {
		if !slices.Contains(validMarkets, m) {
			return Config{}, fmt.Errorf("invalid AS24_MARKET %q, valid: %v", m, validMarkets)
		}
		cfg.Market = m
	}
	if f := getenv("AS24_FETCHERS"); f != "" {
		cfg.Fetchers = strings.Split(f, ",")
		for _, name := range cfg.Fetchers {
			if !slices.Contains(validFetchers, name) {
				return Config{}, fmt.Errorf("invalid fetcher %q, valid: %v", name, validFetchers)
			}
		}
	}
	if tstr := getenv("AS24_TIMEOUT"); tstr != "" {
		d, err := time.ParseDuration(tstr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AS24_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
	}
	return cfg, nil
}
