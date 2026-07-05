// Package as24 implements AutoScout24 operations on top of the fetch layer.
package as24

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/adam/autoscout24-mcp/internal/fetch"
	"github.com/adam/autoscout24-mcp/internal/parser"
)

type Service struct {
	fetcher fetch.Fetcher
	market  string
}

func New(f fetch.Fetcher, market string) *Service {
	return &Service{fetcher: f, market: market}
}

func (s *Service) baseURL() string {
	return "https://www.autoscout24." + s.market
}

// Listing fetches full details for a listing ID (GUID) or autoscout24 URL.
func (s *Service) Listing(ctx context.Context, idOrURL string) (*parser.ListingDetails, error) {
	if idOrURL == "" {
		return nil, errors.New("listing id or url required")
	}
	target := idOrURL
	if strings.HasPrefix(idOrURL, "http") {
		u, err := url.Parse(idOrURL)
		if err != nil {
			return nil, fmt.Errorf("invalid listing url: %w", err)
		}
		host := strings.TrimPrefix(u.Hostname(), "www.")
		if !strings.HasPrefix(host, "autoscout24.") {
			return nil, fmt.Errorf("not an autoscout24 url: %s", idOrURL)
		}
	} else {
		// GUID-only URLs 308-redirect to the canonical listing URL
		target = s.baseURL() + "/angebote/" + url.PathEscape(idOrURL)
	}
	p, err := s.fetcher.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	return parser.ParseListing(p.Body)
}

// Dealer fetches a dealer profile (with inventory) by autoscout24 dealer page URL or slug.
func (s *Service) Dealer(ctx context.Context, slugOrURL string) (*parser.Dealer, error) {
	if slugOrURL == "" {
		return nil, errors.New("dealer slug or url required")
	}
	target := slugOrURL
	if !strings.HasPrefix(slugOrURL, "http") {
		target = s.baseURL() + "/haendler/" + url.PathEscape(slugOrURL)
	}
	p, err := s.fetcher.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	return parser.ParseDealer(p.Body)
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
