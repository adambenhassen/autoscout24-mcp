package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adam/autoscout24-mcp/internal/fetch"
)

// Dealer is a dealer profile with current inventory.
type Dealer struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Address    string    `json:"address,omitempty"`
	Homepage   string    `json:"homepage,omitempty"`
	Rating     float64   `json:"rating,omitempty"`
	Listings   []Listing `json:"listings"`
	TotalCount int       `json:"total_count"`
}

type rawDealerPage struct {
	Props struct {
		PageProps struct {
			DealerInfoPage *struct {
				CustomerID      json.Number `json:"customerId"`
				CustomerName    string      `json:"customerName"`
				HomepageURL     string      `json:"homepageUrl"`
				CustomerAddress struct {
					Country string `json:"country"`
					ZipCode string `json:"zipCode"`
					City    string `json:"city"`
					Street  string `json:"street"`
				} `json:"customerAddress"`
				Ratings struct {
					RatingAverage float64 `json:"ratingAverage"`
				} `json:"ratings"`
			} `json:"dealerInfoPage"`
			Listings        []rawDealerListing `json:"listings"`
			NumberOfResults int                `json:"numberOfResults"`
		} `json:"pageProps"`
	} `json:"props"`
}

type rawDealerListing struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Prices struct {
		Public struct {
			PriceRaw int `json:"priceRaw"`
		} `json:"public"`
	} `json:"prices"`
	Vehicle struct {
		Make              string          `json:"make"`
		Model             string          `json:"model"`
		ModelVersionInput string          `json:"modelVersionInput"`
		MileageInKm       rawFormattedInt `json:"mileageInKm"`
		PowerInKw         rawFormattedInt `json:"powerInKw"`
		PrimaryFuel       struct {
			Formatted string `json:"formatted"`
		} `json:"primaryFuel"`
		TransmissionType struct {
			Formatted string `json:"formatted"`
		} `json:"transmissionType"`
		FirstRegistration struct {
			Raw string `json:"raw"`
		} `json:"firstRegistrationDate"`
	} `json:"vehicle"`
	Location struct {
		CountryCode string `json:"countryCode"`
		Zip         string `json:"zip"`
		City        string `json:"city"`
	} `json:"location"`
}

type rawFormattedInt struct {
	Raw int `json:"raw"`
}

// ParseDealer parses a /haendler dealer info page.
func ParseDealer(html []byte) (*Dealer, error) {
	data, err := ExtractNextData(html)
	if err != nil {
		return nil, err
	}
	var raw rawDealerPage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("dealer payload: %w: %w", fetch.ErrParse, err)
	}
	pp := raw.Props.PageProps
	info := pp.DealerInfoPage
	if info == nil || info.CustomerName == "" {
		return nil, fmt.Errorf("dealer payload missing dealerInfoPage: %w", fetch.ErrParse)
	}
	d := &Dealer{
		ID:         info.CustomerID.String(),
		Name:       info.CustomerName,
		Homepage:   info.HomepageURL,
		Rating:     info.Ratings.RatingAverage,
		TotalCount: pp.NumberOfResults,
		Listings:   make([]Listing, 0, len(pp.Listings)),
	}
	addr := info.CustomerAddress
	if addr.City != "" {
		d.Address = strings.TrimSpace(addr.Street + ", " + addr.ZipCode + " " + addr.City + ", " + addr.Country)
	}
	for i := range pp.Listings {
		r := &pp.Listings[i]
		l := Listing{
			ID:        r.ID,
			URL:       absoluteURL(r.URL),
			Title:     strings.TrimSpace(strings.Join([]string{r.Vehicle.Make, r.Vehicle.Model, r.Vehicle.ModelVersionInput}, " ")),
			PriceEUR:  r.Prices.Public.PriceRaw,
			MileageKM: r.Vehicle.MileageInKm.Raw,
			PowerKW:   r.Vehicle.PowerInKw.Raw,
			FirstReg:  r.Vehicle.FirstRegistration.Raw,
			Fuel:      r.Vehicle.PrimaryFuel.Formatted,
			Gearbox:   r.Vehicle.TransmissionType.Formatted,
		}
		if r.Location.City != "" {
			l.Location = strings.TrimSpace(r.Location.Zip + " " + r.Location.City + ", " + r.Location.CountryCode)
		}
		d.Listings = append(d.Listings, l)
	}
	return d, nil
}
