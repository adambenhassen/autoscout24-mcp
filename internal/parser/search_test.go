package parser_test

import (
	"errors"
	"os"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/fetch"
	"github.com/adam/autoscout24-mcp/internal/parser"
)

// base is the market host the fixtures were captured from.
const base = "https://www.autoscout24.de"

func TestExtractNextDataMissing(t *testing.T) {
	if _, err := parser.ExtractNextData([]byte("<html>no payload</html>")); !errors.Is(err, fetch.ErrParse) {
		t.Fatal("want ErrParse")
	}
}

func TestParseSearchFixture(t *testing.T) {
	html, err := os.ReadFile("testdata/search_page.html")
	if err != nil {
		t.Fatal(err)
	}
	res, err := parser.ParseSearch(html, base)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 2093 || res.PageCount != 108 || len(res.Listings) != 20 {
		t.Fatalf("bad counts: total=%d pages=%d listings=%d", res.TotalCount, res.PageCount, len(res.Listings))
	}
	l := res.Listings[0]
	if l.ID != "37d4ccc4-6999-48ef-a92a-2c37232f177d" {
		t.Fatalf("bad id: %q", l.ID)
	}
	if l.URL != "https://www.autoscout24.de/angebote/bmw-320-d-xdrive-mild-hybrid-aut-diesel-cat_ma13mo1641-37d4ccc4-6999-48ef-a92a-2c37232f177d" {
		t.Fatalf("bad url: %q", l.URL)
	}
	if l.Title != "BMW 320 d xDrive Mild-Hybrid Aut." {
		t.Fatalf("bad title: %q", l.Title)
	}
	if l.PriceEUR != 20000 || l.MileageKM != 74983 || l.FirstReg != "05-2024" {
		t.Fatalf("bad numbers: %+v", l)
	}
	if l.Fuel != "Diesel" || l.Gearbox != "Automatik" || l.PowerKW != 140 {
		t.Fatalf("bad specs: %+v", l)
	}
	if l.SellerType != "dealer" || l.Location != "09526 Olbernhau, DE" {
		t.Fatalf("bad seller/location: %+v", l)
	}
	if l.PriceRating != "toolow-price" {
		t.Fatalf("bad price rating: %q", l.PriceRating)
	}
}

func TestParseSearchJunk(t *testing.T) {
	if _, err := parser.ParseSearch([]byte(`<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>`), base); !errors.Is(err, fetch.ErrParse) {
		t.Fatalf("want ErrParse, got %v", err)
	}
}
