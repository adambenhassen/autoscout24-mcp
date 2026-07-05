package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

// Listing is one search result.
type Listing struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	PriceEUR    int    `json:"price_eur"`
	PriceRating string `json:"price_rating,omitempty"` // e.g. "toolow-price", "good-price"
	MileageKM   int    `json:"mileage_km"`
	FirstReg    string `json:"first_registration,omitempty"` // "MM-YYYY"
	Fuel        string `json:"fuel,omitempty"`
	Gearbox     string `json:"gearbox,omitempty"`
	PowerKW     int    `json:"power_kw,omitempty"`
	Location    string `json:"location,omitempty"`
	SellerType  string `json:"seller_type,omitempty"` // "dealer" | "private"
}

// SearchResult is one page of search results.
type SearchResult struct {
	Listings   []Listing `json:"listings"`
	TotalCount int       `json:"total_count"`
	Page       int       `json:"page"`
	PageCount  int       `json:"page_count"`
}

// rawSearchListing mirrors the fields we use from props.pageProps.listings[].
type rawSearchListing struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Price struct {
		PriceRaw int `json:"priceRaw"`
	} `json:"price"`
	Vehicle struct {
		Make              string `json:"make"`
		Model             string `json:"model"`
		ModelVersionInput string `json:"modelVersionInput"`
		Fuel              string `json:"fuel"`
		Transmission      string `json:"transmission"`
	} `json:"vehicle"`
	Location struct {
		CountryCode string `json:"countryCode"`
		Zip         string `json:"zip"`
		City        string `json:"city"`
	} `json:"location"`
	Seller struct {
		Type string `json:"type"`
	} `json:"seller"`
	Tracking struct {
		FirstRegistration string `json:"firstRegistration"`
		Mileage           string `json:"mileage"`
		PriceLabel        string `json:"priceLabel"`
	} `json:"tracking"`
	VehicleDetails []struct {
		Data     string `json:"data"`
		IconName string `json:"iconName"`
	} `json:"vehicleDetails"`
}

type rawSearchPage struct {
	Props struct {
		PageProps struct {
			Listings        []rawSearchListing `json:"listings"`
			NumberOfResults *int               `json:"numberOfResults"`
			NumberOfPages   int                `json:"numberOfPages"`
			PageQuery       map[string]string  `json:"pageQuery"`
		} `json:"pageProps"`
	} `json:"props"`
}

// ParseSearch parses a /lst search results page. base is the market host
// (e.g. https://www.autoscout24.it) used to absolutize relative listing URLs.
func ParseSearch(html []byte, base string) (*SearchResult, error) {
	data, err := ExtractNextData(html)
	if err != nil {
		return nil, err
	}
	var raw rawSearchPage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("search payload: %w: %w", fetch.ErrParse, err)
	}
	pp := raw.Props.PageProps
	if pp.NumberOfResults == nil {
		return nil, fmt.Errorf("search payload missing numberOfResults: %w", fetch.ErrParse)
	}
	res := &SearchResult{
		Listings:   make([]Listing, 0, len(pp.Listings)),
		TotalCount: *pp.NumberOfResults,
		Page:       1,
		PageCount:  pp.NumberOfPages,
	}
	if p, err := strconv.Atoi(pp.PageQuery["page"]); err == nil {
		res.Page = p
	}
	for i := range pp.Listings {
		res.Listings = append(res.Listings, mapSearchListing(&pp.Listings[i], base))
	}
	return res, nil
}

func mapSearchListing(r *rawSearchListing, base string) Listing {
	l := Listing{
		ID:          r.ID,
		URL:         absoluteURL(base, r.URL),
		Title:       strings.TrimSpace(strings.Join([]string{r.Vehicle.Make, r.Vehicle.Model, r.Vehicle.ModelVersionInput}, " ")),
		PriceEUR:    r.Price.PriceRaw,
		PriceRating: strings.TrimSpace(r.Tracking.PriceLabel),
		FirstReg:    r.Tracking.FirstRegistration,
		Fuel:        r.Vehicle.Fuel,
		Gearbox:     r.Vehicle.Transmission,
		SellerType:  strings.ToLower(r.Seller.Type),
	}
	if km, err := strconv.Atoi(r.Tracking.Mileage); err == nil {
		l.MileageKM = km
	}
	for _, d := range r.VehicleDetails {
		if d.IconName == "speedometer" {
			l.PowerKW = leadingInt(d.Data)
			break
		}
	}
	if r.Location.City != "" {
		l.Location = strings.TrimSpace(r.Location.Zip + " " + r.Location.City + ", " + r.Location.CountryCode)
	}
	return l
}

// leadingInt parses the integer prefix of strings like "140 kW (190 PS)".
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return n
}

// absoluteURL turns a site-relative path into an absolute URL under base
// (the market host). Already-absolute URLs are returned unchanged.
func absoluteURL(base, u string) string {
	if strings.HasPrefix(u, "/") {
		return base + u
	}
	return u
}
