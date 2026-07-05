package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adambenhassen/autoscout24-mcp/internal/as24"
	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

func TestToolErrorMapping(t *testing.T) {
	cases := []struct {
		in   error
		want string
	}{
		{fetch.ErrBlocked, "fallback stage"},
		{fetch.ErrNotFound, "no longer available"},
		{fetch.ErrParse, "parser needs updating"},
		{errors.New("boom"), "boom"},
	}
	for _, tc := range cases {
		got := toolError(tc.in)
		if got == nil || !strings.Contains(got.Error(), tc.want) {
			t.Errorf("toolError(%v) = %v, want containing %q", tc.in, got, tc.want)
		}
	}
}

func TestServerRegistersTools(t *testing.T) {
	ctx := context.Background()
	server := New(as24.New(fetch.NewHTTPFetcher(0), "de"))

	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := ss.Close(); cerr != nil {
			t.Error(cerr)
		}
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := cs.Close(); cerr != nil {
			t.Error(cerr)
		}
	}()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"search_listings": false, "get_listing": false, "price_analysis": false,
		"get_dealer": false, "list_makes_models": false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %s not registered", name)
		}
	}
}
