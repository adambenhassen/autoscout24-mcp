package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

// Seller describes the party offering a listing.
type Seller struct {
	Name     string  `json:"name,omitempty"`
	Type     string  `json:"type,omitempty"` // "dealer" | "private"
	Contact  string  `json:"contact,omitempty"`
	Phone    string  `json:"phone,omitempty"`
	DealerID string  `json:"dealer_id,omitempty"`
	InfoPage string  `json:"info_page,omitempty"`
	Rating   float64 `json:"rating,omitempty"`
}

// ListingDetails is the full detail view of a listing.
type ListingDetails struct {
	Listing

	Description     string   `json:"description,omitempty"`
	BodyType        string   `json:"body_type,omitempty"`
	Color           string   `json:"color,omitempty"`
	Doors           int      `json:"doors,omitempty"`
	Seats           int      `json:"seats,omitempty"`
	PreviousOwners  int      `json:"previous_owners,omitempty"`
	MarketMedianEUR int      `json:"market_median_eur,omitempty"`
	PriceCategory   int      `json:"price_category"` // 0 = cheapest bucket vs. market
	Equipment       []string `json:"equipment,omitempty"`
	Images          []string `json:"images,omitempty"`
	Seller          Seller   `json:"seller"`
}

type rawListingPage struct {
	Props struct {
		PageProps struct {
			ListingDetails *rawListingDetails `json:"listingDetails"`
		} `json:"pageProps"`
	} `json:"props"`
}

type rawListingDetails struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
	Prices      struct {
		Public struct {
			PriceRaw int `json:"priceRaw"`
			Category int `json:"category"`
			Median   int `json:"median"`
		} `json:"public"`
	} `json:"prices"`
	Vehicle struct {
		Make                     string `json:"make"`
		Model                    string `json:"model"`
		ModelVersionInput        string `json:"modelVersionInput"`
		FirstRegistrationDateRaw string `json:"firstRegistrationDateRaw"`
		MileageInKmRaw           int    `json:"mileageInKmRaw"`
		RawPowerInKw             int    `json:"rawPowerInKw"`
		PrimaryFuel              struct {
			Formatted string `json:"formatted"`
		} `json:"primaryFuel"`
		TransmissionType string          `json:"transmissionType"`
		BodyType         string          `json:"bodyType"`
		BodyColor        string          `json:"bodyColor"`
		NumberOfDoors    int             `json:"numberOfDoors"`
		NumberOfSeats    int             `json:"numberOfSeats"`
		NoOfPrevOwners   int             `json:"noOfPreviousOwners"`
		Equipment        json.RawMessage `json:"equipment"`
	} `json:"vehicle"`
	Location struct {
		CountryCode string `json:"countryCode"`
		Zip         string `json:"zip"`
		City        string `json:"city"`
	} `json:"location"`
	Seller struct {
		ID          json.Number `json:"id"`
		Type        string      `json:"type"`
		CompanyName string      `json:"companyName"`
		ContactName string      `json:"contactName"`
		Links       struct {
			InfoPage string `json:"infoPage"`
		} `json:"links"`
		Phones []struct {
			FormattedNumber string `json:"formattedNumber"`
		} `json:"phones"`
	} `json:"seller"`
	Ratings struct {
		RatingsStars float64 `json:"ratingsStars"`
	} `json:"ratings"`
	WebPage string `json:"webPage"`
}

// ParseListing parses a listing detail page. base is the market host used to
// absolutize the listing's own URL.
func ParseListing(html []byte, base string) (*ListingDetails, error) {
	data, err := ExtractNextData(html)
	if err != nil {
		return nil, err
	}
	var raw rawListingPage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("listing payload: %w: %w", fetch.ErrParse, err)
	}
	r := raw.Props.PageProps.ListingDetails
	if r == nil || r.ID == "" {
		return nil, fmt.Errorf("listing payload missing listingDetails: %w", fetch.ErrParse)
	}
	d := &ListingDetails{
		Listing: Listing{
			ID:         r.ID,
			URL:        absoluteURL(base, r.WebPage),
			Title:      strings.TrimSpace(strings.Join([]string{r.Vehicle.Make, r.Vehicle.Model, r.Vehicle.ModelVersionInput}, " ")),
			PriceEUR:   r.Prices.Public.PriceRaw,
			MileageKM:  r.Vehicle.MileageInKmRaw,
			FirstReg:   r.Vehicle.FirstRegistrationDateRaw,
			Fuel:       r.Vehicle.PrimaryFuel.Formatted,
			Gearbox:    r.Vehicle.TransmissionType,
			PowerKW:    r.Vehicle.RawPowerInKw,
			SellerType: strings.ToLower(r.Seller.Type),
		},
		Description:     r.Description,
		BodyType:        r.Vehicle.BodyType,
		Color:           r.Vehicle.BodyColor,
		Doors:           r.Vehicle.NumberOfDoors,
		Seats:           r.Vehicle.NumberOfSeats,
		PreviousOwners:  r.Vehicle.NoOfPrevOwners,
		MarketMedianEUR: r.Prices.Public.Median,
		PriceCategory:   r.Prices.Public.Category,
		Equipment:       flattenEquipment(r.Vehicle.Equipment),
		Images:          r.Images,
		Seller: Seller{
			Name:     r.Seller.CompanyName,
			Type:     strings.ToLower(r.Seller.Type),
			Contact:  r.Seller.ContactName,
			DealerID: r.Seller.ID.String(),
			InfoPage: r.Seller.Links.InfoPage,
			Rating:   r.Ratings.RatingsStars,
		},
	}
	if len(r.Seller.Phones) > 0 {
		d.Seller.Phone = r.Seller.Phones[0].FormattedNumber
	}
	if r.Location.City != "" {
		d.Location = strings.TrimSpace(r.Location.Zip + " " + r.Location.City + ", " + r.Location.CountryCode)
	}
	return d, nil
}

// flattenEquipment tolerates the varying equipment shapes AS24 uses:
// map of category -> []string or []{ "label": ... }, possibly empty.
func flattenEquipment(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var byCategory map[string][]json.RawMessage
	if err := json.Unmarshal(raw, &byCategory); err != nil {
		return nil
	}
	var out []string
	for _, items := range byCategory {
		for _, item := range items {
			var s string
			if err := json.Unmarshal(item, &s); err == nil {
				out = append(out, s)
				continue
			}
			var labeled struct {
				Label string `json:"label"`
			}
			if err := json.Unmarshal(item, &labeled); err == nil && labeled.Label != "" {
				out = append(out, labeled.Label)
			}
		}
	}
	return out
}
