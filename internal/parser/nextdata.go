// Package parser extracts structured data from AutoScout24 pages.
package parser

import (
	"fmt"
	"regexp"

	"github.com/adambenhassen/autoscout24-mcp/internal/fetch"
)

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

// ExtractNextData returns the raw __NEXT_DATA__ JSON embedded in an AutoScout24 page.
func ExtractNextData(html []byte) ([]byte, error) {
	m := nextDataRe.FindSubmatch(html)
	if m == nil {
		return nil, fmt.Errorf("__NEXT_DATA__ script not found: %w", fetch.ErrParse)
	}
	return m[1], nil
}
