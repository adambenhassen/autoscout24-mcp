// Package as24 implements AutoScout24 operations on top of the fetch layer.
package as24

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
	"github.com/adambenhassen/autoscout24-mcp/internal/parser"
)

type Service struct {
	fetcher fetch.Fetcher
	market  string // market code, e.g. "de"
	mkt     market // resolved host and path segments
}

func New(f fetch.Fetcher, marketCode string) *Service {
	return &Service{fetcher: f, market: marketCode, mkt: marketFor(marketCode)}
}

// Listing fetches full details for a listing ID (GUID) or autoscout24 URL.
func (s *Service) Listing(ctx context.Context, idOrURL string) (*parser.ListingDetails, error) {
	target, err := s.resolveTarget(idOrURL, s.mkt.listingSeg, "listing", "id")
	if err != nil {
		return nil, err
	}
	p, err := s.fetcher.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	return parser.ParseListing(p.Body, s.mkt.host)
}

// Dealer fetches a dealer profile (with inventory) by autoscout24 dealer page URL or slug.
func (s *Service) Dealer(ctx context.Context, slugOrURL string) (*parser.Dealer, error) {
	target, err := s.resolveTarget(slugOrURL, s.mkt.dealerSeg, "dealer", "slug")
	if err != nil {
		return nil, err
	}
	p, err := s.fetcher.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	return parser.ParseDealer(p.Body, s.mkt.host)
}

// resolveTarget turns a raw ID/slug or a full autoscout24 URL into a fetchable
// target. seg is the market path segment (listingSeg/dealerSeg); kind and idWord
// name the resource in error messages ("listing"/"id", "dealer"/"slug"). A raw
// ID/slug becomes host/seg/<escaped>, which 308-redirects to the canonical URL.
func (s *Service) resolveTarget(idOrURL, seg, kind, idWord string) (string, error) {
	if idOrURL == "" {
		return "", fmt.Errorf("%s %s or url required", kind, idWord)
	}
	if strings.HasPrefix(idOrURL, "http") {
		u, err := url.Parse(idOrURL)
		if err != nil {
			return "", fmt.Errorf("invalid %s url: %w", kind, err)
		}
		if !isKnownHost(u.Hostname()) {
			return "", fmt.Errorf("not an autoscout24 url: %s", idOrURL)
		}
		return idOrURL, nil
	}
	return s.mkt.host + "/" + seg + "/" + url.PathEscape(idOrURL), nil
}

// MakesModels returns valid make names, and model names for the given make,
// extracted from the search page taxonomy payload.
func (s *Service) MakesModels(ctx context.Context, makeName string) (map[string][]string, error) {
	searchURL := BuildSearchURL(s.market, SearchParams{Make: makeName})
	p, err := s.fetcher.Get(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	data, err := parser.ExtractNextData(p.Body)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Props struct {
			PageProps struct {
				Taxonomy struct {
					Makes map[string]struct {
						Label string `json:"label"`
					} `json:"makes"`
					Models map[string][]struct {
						Label  string `json:"label"`
						MakeID int    `json:"makeId"`
					} `json:"models"`
				} `json:"taxonomy"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("taxonomy payload: %w: %w", fetch.ErrParse, err)
	}
	tax := raw.Props.PageProps.Taxonomy
	if len(tax.Makes) == 0 {
		return nil, fmt.Errorf("taxonomy missing makes: %w", fetch.ErrParse)
	}
	out := make(map[string][]string, len(tax.Makes))
	for _, m := range tax.Makes {
		out[m.Label] = nil
	}
	for _, models := range tax.Models {
		for _, mo := range models {
			label := makeLabelByID(tax.Makes, mo.MakeID)
			if label != "" {
				out[label] = append(out[label], mo.Label)
			}
		}
	}
	return out, nil
}

func makeLabelByID(makes map[string]struct {
	Label string `json:"label"`
}, id int) string {
	if m, ok := makes[strconv.Itoa(id)]; ok {
		return m.Label
	}
	return ""
}
