//go:build integration

package main_test

import (
	"context"
	"testing"
	"time"

	"github.com/adam/autoscout24-mcp/internal/as24"
	"github.com/adam/autoscout24-mcp/internal/fetch"
)

func TestIntegrationLiveSite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	svc := as24.New(fetch.NewHTTPFetcher(0), "de")

	res, err := svc.Search(ctx, as24.SearchParams{Make: "BMW", Model: "320", PriceFrom: 10000, PriceTo: 30000})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount == 0 || len(res.Listings) == 0 {
		t.Fatalf("no results: %+v", res)
	}

	d, err := svc.Listing(ctx, res.Listings[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || d.PriceEUR <= 0 {
		t.Fatalf("bad listing details: %+v", d.Listing)
	}

	stats, err := svc.PriceAnalysis(ctx, as24.SearchParams{Make: "BMW", Model: "320", PriceFrom: 10000, PriceTo: 30000})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count == 0 || stats.Min > stats.Median || stats.Median > stats.Max {
		t.Fatalf("implausible stats: %+v", stats)
	}

	mm, err := svc.MakesModels(ctx, "BMW")
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) == 0 || len(mm["BMW"]) == 0 {
		t.Fatalf("bad taxonomy: %d makes, %d BMW models", len(mm), len(mm["BMW"]))
	}
}
