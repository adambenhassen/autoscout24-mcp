package as24

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/adambenhassen/autoscout24-mcp/internal/parser"
)

// SearchParams are the supported search filters. Zero values mean "not set".
type SearchParams struct {
	Make        string `json:"make,omitempty"`
	Model       string `json:"model,omitempty"`
	PriceFrom   int    `json:"price_from,omitempty"`
	PriceTo     int    `json:"price_to,omitempty"`
	MileageFrom int    `json:"mileage_from,omitempty"`
	MileageTo   int    `json:"mileage_to,omitempty"`
	YearFrom    int    `json:"year_from,omitempty"`
	YearTo      int    `json:"year_to,omitempty"`
	PowerFromKW int    `json:"power_from_kw,omitempty"`
	PowerToKW   int    `json:"power_to_kw,omitempty"`
	Fuel        string `json:"fuel,omitempty"`
	Gearbox     string `json:"gearbox,omitempty"`
	Body        string `json:"body,omitempty"`
	Zip         string `json:"zip,omitempty"`
	ZipRadiusKM int    `json:"zip_radius_km,omitempty"`
	Sort        string `json:"sort,omitempty"` // "price" | "mileage" | "year" | "standard"
	Desc        bool   `json:"desc,omitempty"`
	Page        int    `json:"page,omitempty"`
}

// BuildSearchURL renders SearchParams into an AutoScout24 /lst URL.
func BuildSearchURL(marketCode string, p SearchParams) string {
	base := marketFor(marketCode).host + "/lst"
	if p.Make != "" {
		base += "/" + slugify(p.Make)
		if p.Model != "" {
			base += "/" + slugify(p.Model)
		}
	}
	q := url.Values{}
	setInt := func(key, val string) {
		if val != "0" {
			q.Set(key, val)
		}
	}
	setInt("pricefrom", strconv.Itoa(p.PriceFrom))
	setInt("priceto", strconv.Itoa(p.PriceTo))
	setInt("kmfrom", strconv.Itoa(p.MileageFrom))
	setInt("kmto", strconv.Itoa(p.MileageTo))
	setInt("fregfrom", strconv.Itoa(p.YearFrom))
	setInt("fregto", strconv.Itoa(p.YearTo))
	setInt("powerfrom", strconv.Itoa(p.PowerFromKW))
	setInt("powerto", strconv.Itoa(p.PowerToKW))
	if p.PowerFromKW > 0 || p.PowerToKW > 0 {
		q.Set("powertype", "kw")
	}
	if p.Fuel != "" {
		q.Set("fuel", p.Fuel)
	}
	if p.Gearbox != "" {
		q.Set("gear", p.Gearbox)
	}
	if p.Body != "" {
		q.Set("body", p.Body)
	}
	if p.Zip != "" {
		q.Set("zip", p.Zip)
	}
	setInt("zipr", strconv.Itoa(p.ZipRadiusKM))
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	if p.Desc {
		q.Set("desc", "1")
	}
	setInt("page", strconv.Itoa(p.Page))
	if enc := q.Encode(); enc != "" {
		return base + "?" + enc
	}
	return base
}

func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

// Search runs a listing search.
func (s *Service) Search(ctx context.Context, p SearchParams) (*parser.SearchResult, error) {
	page, err := s.fetcher.Get(ctx, BuildSearchURL(s.market, p))
	if err != nil {
		return nil, err
	}
	res, err := parser.ParseSearch(page.Body, s.mkt.host)
	if err != nil {
		return nil, err
	}
	if p.Page > 0 {
		res.Page = p.Page
	}
	return res, nil
}
