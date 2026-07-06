// Package mcpserver registers the AutoScout24 MCP tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adambenhassen/autoscout24-mcp/internal/as24"
	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
	"github.com/adambenhassen/autoscout24-mcp/internal/parser"
)

// New builds the MCP server with all AutoScout24 tools registered.
func New(svc *as24.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "autoscout24", Version: "0.1.1"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_listings",
		Description: "Search AutoScout24 car listings. All filters optional. Fuel/gearbox/body take AutoScout24 codes (fuel: B=petrol, D=diesel, E=electric, 2=hybrid; gear: A=automatic, M=manual). Sort: price|mileage|year|standard. Returns one page of listings (20) plus total count and page count.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in as24.SearchParams) (*mcp.CallToolResult, *parser.SearchResult, error) {
		res, err := svc.Search(ctx, in)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})

	type listingIn struct {
		Listing string `json:"listing" jsonschema:"listing ID (GUID) or full autoscout24 listing URL"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_listing",
		Description: "Get full details of an AutoScout24 listing: specs, equipment, price vs. market median, images, seller contact.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listingIn) (*mcp.CallToolResult, *parser.ListingDetails, error) {
		res, err := svc.Listing(ctx, in.Listing)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "price_analysis",
		Description: "Price distribution (min/p25/median/avg/p75/max) over listings matching the given search filters, sampled from up to 100 live listings, plus AutoScout24's own price-rating breakdown.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in as24.SearchParams) (*mcp.CallToolResult, *as24.PriceStats, error) {
		res, err := svc.PriceAnalysis(ctx, in)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})

	type dealerIn struct {
		Dealer string `json:"dealer" jsonschema:"dealer page URL or dealer slug (e.g. auto-kreher-gmbh)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_dealer",
		Description: "Get an AutoScout24 dealer profile: name, address, rating, and current inventory.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in dealerIn) (*mcp.CallToolResult, *parser.Dealer, error) {
		res, err := svc.Dealer(ctx, in.Dealer)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})

	type makesIn struct {
		Make string `json:"make,omitempty" jsonschema:"optional make name to get models for (e.g. BMW)"`
	}
	type makesOut struct {
		MakesModels map[string][]string `json:"makes_models"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_makes_models",
		Description: "List valid car makes (and models for a given make) to use in search_listings.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in makesIn) (*mcp.CallToolResult, *makesOut, error) {
		res, err := svc.MakesModels(ctx, in.Make)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &makesOut{MakesModels: res}, nil
	})

	return server
}

// toolError converts internal errors into actionable tool errors.
func toolError(err error) error {
	switch {
	case errors.Is(err, fetch.ErrBlocked), errors.Is(err, fetch.ErrUnavailable):
		return fmt.Errorf("%w — enable the camoufox fallback stage: install camoufox (pip install \"camoufox[geoip]\") and include it in AS24_FETCHERS", err)
	case errors.Is(err, fetch.ErrNotFound):
		return errors.New("listing or page no longer available on AutoScout24")
	case errors.Is(err, fetch.ErrParse):
		return fmt.Errorf("%w — AutoScout24 likely changed its page structure; the parser needs updating", err)
	default:
		return err
	}
}
