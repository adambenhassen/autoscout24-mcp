// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

var validMarkets = []string{"de", "com", "it", "fr", "nl", "at", "be", "es"}
var validFetchers = []string{"http", "camoufox", "crw"}

type Config struct {
	Market      string
	Fetchers    []string
	HTTPAddr    string
	CamoufoxCmd string
	CRWURL      string
	CRWAPIKey   string
	Timeout     time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Market:      "de",
		Fetchers:    []string{"http", "camoufox"},
		HTTPAddr:    getenv("AS24_HTTP_ADDR"),
		CamoufoxCmd: getenv("AS24_CAMOUFOX_CMD"),
		CRWURL:      getenv("CRW_URL"),
		CRWAPIKey:   getenv("CRW_API_KEY"),
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
