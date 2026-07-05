package parser_test

import (
	"errors"
	"os"
	"testing"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
	"github.com/adambenhassen/autoscout24-mcp/internal/parser"
)

func TestParseListingFixture(t *testing.T) {
	html, err := os.ReadFile("testdata/listing_page.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parser.ParseListing(html, base)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "37d4ccc4-6999-48ef-a92a-2c37232f177d" {
		t.Fatalf("bad id: %q", d.ID)
	}
	if d.Title != "BMW 320 d xDrive Mild-Hybrid Aut." || d.PriceEUR != 20000 {
		t.Fatalf("bad title/price: %+v", d.Listing)
	}
	if d.MileageKM != 74983 || d.FirstReg != "2024-05-01" || d.PowerKW != 140 {
		t.Fatalf("bad numbers: %+v", d.Listing)
	}
	if d.Fuel != "Diesel" || d.Gearbox != "Automatik" || d.BodyType != "Limousine" {
		t.Fatalf("bad specs: %+v", d)
	}
	if d.Doors != 4 || d.Seats != 5 {
		t.Fatalf("bad doors/seats: %+v", d)
	}
	if d.MarketMedianEUR != 34600 {
		t.Fatalf("bad median: %d", d.MarketMedianEUR)
	}
	if len(d.Images) != 18 {
		t.Fatalf("bad images: %d", len(d.Images))
	}
	if d.Seller.Name != "Auto-Kreher GmbH" || d.Seller.Type != "dealer" || d.Seller.DealerID != "16115115" {
		t.Fatalf("bad seller: %+v", d.Seller)
	}
	if d.Description == "" {
		t.Fatal("empty description")
	}
	if d.Location != "09526 Olbernhau, DE" {
		t.Fatalf("bad location: %q", d.Location)
	}
}

func TestParseListingJunk(t *testing.T) {
	if _, err := parser.ParseListing([]byte(`<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>`), base); !errors.Is(err, fetch.ErrParse) {
		t.Fatalf("want ErrParse, got %v", err)
	}
}

func TestParseDealerFixture(t *testing.T) {
	html, err := os.ReadFile("testdata/dealer_page.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parser.ParseDealer(html, base)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Auto-Kreher GmbH" {
		t.Fatalf("bad name: %q", d.Name)
	}
	if d.ID != "16115115" {
		t.Fatalf("bad id: %q", d.ID)
	}
	if d.Address != "Zollstr. 21, 09526 Olbernhau, DE" {
		t.Fatalf("bad address: %q", d.Address)
	}
	if d.Rating != 5 {
		t.Fatalf("bad rating: %v", d.Rating)
	}
	if d.TotalCount != 562 || len(d.Listings) == 0 {
		t.Fatalf("bad listings: total=%d n=%d", d.TotalCount, len(d.Listings))
	}
	l := d.Listings[0]
	if l.ID != "f839b944-28e4-448d-af81-edc0d2b201c8" || l.PriceEUR != 14000 || l.MileageKM != 31304 || l.PowerKW != 220 {
		t.Fatalf("bad first listing: %+v", l)
	}
}

func TestParseDealerJunk(t *testing.T) {
	if _, err := parser.ParseDealer([]byte(`<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>`), base); !errors.Is(err, fetch.ErrParse) {
		t.Fatalf("want ErrParse, got %v", err)
	}
}
