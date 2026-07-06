package as24

import "strings"

// market holds the market-specific host and URL path segments. AutoScout24
// localizes the listing and dealer path segments per market (e.g. /angebote/
// in .de but /annunci/ in .it), so ID/slug lookups must use the right ones.
type market struct {
	host       string // e.g. https://www.autoscout24.de
	listingSeg string // path segment for a single listing, e.g. angebote
	dealerSeg  string // path segment for a dealer page, e.g. haendler
}

// markets are the flat-structure European markets (verified against the live
// site). Belgium (.be) is intentionally absent: it requires a /nl/ or /fr/
// locale prefix on every path, which this flat URL scheme does not support.
var markets = map[string]market{
	"de":  {"https://www.autoscout24.de", "angebote", "haendler"},
	"com": {"https://www.autoscout24.com", "offers", "dealerinfo"},
	"it":  {"https://www.autoscout24.it", "annunci", "concessionari"},
	"fr":  {"https://www.autoscout24.fr", "offres", "garages"},
	"es":  {"https://www.autoscout24.es", "anuncios", "profesionales"},
	"nl":  {"https://www.autoscout24.nl", "aanbod", "autobedrijven"},
	"at":  {"https://www.autoscout24.at", "angebote", "haendler"},
}

// marketFor resolves a market code, falling back to .de for unknown codes
// (config validation rejects unknown codes before this is reached).
func marketFor(code string) market {
	if m, ok := markets[code]; ok {
		return m
	}
	return markets["de"]
}

// isKnownHost reports whether host (an autoscout24 hostname, optionally with a
// leading www.) belongs to a supported market. Used to reject arbitrary URLs.
func isKnownHost(host string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	for _, m := range markets {
		if strings.TrimPrefix(strings.TrimPrefix(m.host, "https://"), "www.") == host {
			return true
		}
	}
	return false
}
