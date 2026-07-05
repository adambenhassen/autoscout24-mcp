package as24_test

import (
	"context"
	"os"
	"testing"

	"github.com/adam/autoscout24-mcp/internal/as24"
	"github.com/adam/autoscout24-mcp/internal/fetch"
)

type stubFetcher struct {
	pages map[string]string // url -> fixture path
	urls  []string          // record of requested urls
}

func (s *stubFetcher) Get(_ context.Context, url string) (*fetch.Page, error) {
	s.urls = append(s.urls, url)
	path, ok := s.pages[url]
	if !ok {
		return nil, fetch.ErrNotFound
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &fetch.Page{URL: url, Status: 200, Body: body}, nil
}

func TestBuildSearchURL(t *testing.T) {
	cases := []struct {
		name   string
		market string
		p      as24.SearchParams
		want   string
	}{
		{
			name:   "make model price",
			market: "de",
			p:      as24.SearchParams{Make: "BMW", Model: "320", PriceFrom: 10000, PriceTo: 30000},
			want:   "https://www.autoscout24.de/lst/bmw/320?pricefrom=10000&priceto=30000",
		},
		{
			name:   "no make",
			market: "de",
			p:      as24.SearchParams{PriceTo: 5000},
			want:   "https://www.autoscout24.de/lst?priceto=5000",
		},
		{
			name:   "make only italian market",
			market: "it",
			p:      as24.SearchParams{Make: "Alfa Romeo"},
			want:   "https://www.autoscout24.it/lst/alfa-romeo",
		},
		{
			name:   "com market",
			market: "com",
			p:      as24.SearchParams{Make: "BMW"},
			want:   "https://www.autoscout24.com/lst/bmw",
		},
		{
			name:   "zip radius sort desc page",
			market: "de",
			p:      as24.SearchParams{Make: "VW", Zip: "10115", ZipRadiusKM: 50, Sort: "price", Desc: true, Page: 3},
			want:   "https://www.autoscout24.de/lst/vw?desc=1&page=3&sort=price&zip=10115&zipr=50",
		},
		{
			name:   "full filters",
			market: "de",
			p: as24.SearchParams{
				Make: "BMW", Model: "320",
				MileageFrom: 1000, MileageTo: 90000,
				YearFrom: 2018, YearTo: 2024,
				PowerFromKW: 100, PowerToKW: 200,
				Fuel: "D", Gearbox: "A", Body: "1",
			},
			want: "https://www.autoscout24.de/lst/bmw/320?body=1&fregfrom=2018&fregto=2024&fuel=D&gear=A&kmfrom=1000&kmto=90000&powerfrom=100&powerto=200&powertype=kw",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := as24.BuildSearchURL(tc.market, tc.p)
			if got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestSearchUsesFixture(t *testing.T) {
	url := "https://www.autoscout24.de/lst/bmw/320?pricefrom=10000&priceto=30000"
	f := &stubFetcher{pages: map[string]string{url: "../parser/testdata/search_page.html"}}
	svc := as24.New(f, "de")
	res, err := svc.Search(context.Background(), as24.SearchParams{Make: "BMW", Model: "320", PriceFrom: 10000, PriceTo: 30000})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 2093 || len(res.Listings) != 20 {
		t.Fatalf("bad result: %d/%d", res.TotalCount, len(res.Listings))
	}
}

func TestPriceStats(t *testing.T) {
	prices := []int{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	s := as24.ComputePriceStats(prices)
	if s.Count != 10 || s.Min != 100 || s.Max != 1000 {
		t.Fatalf("bad min/max: %+v", s)
	}
	if s.Median != 500 || s.P25 != 300 || s.P75 != 800 {
		t.Fatalf("bad percentiles: %+v", s)
	}
	if s.Avg != 550 {
		t.Fatalf("bad avg: %d", s.Avg)
	}
}

func TestPriceAnalysisEmptyResult(t *testing.T) {
	// stub returns ErrNotFound for unknown URLs; use an empty-results fixture instead:
	// simplest: a search that parses but has zero listings is hard to fixture, so
	// verify the zero-listing stats path directly.
	s := as24.ComputePriceStats(nil)
	if s.Count != 0 {
		t.Fatalf("want zero count, got %+v", s)
	}
}

func TestListingURLValidation(t *testing.T) {
	svc := as24.New(&stubFetcher{}, "de")
	if _, err := svc.Listing(context.Background(), "https://evil.example/angebote/x"); err == nil {
		t.Fatal("want error for non-autoscout24 URL")
	}
}
