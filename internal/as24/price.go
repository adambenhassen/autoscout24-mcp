package as24

import (
	"context"
	"slices"
)

// PriceStats is a price distribution over matching listings.
type PriceStats struct {
	Count           int            `json:"count"`
	Min             int            `json:"min,omitempty"`
	P25             int            `json:"p25,omitempty"`
	Median          int            `json:"median,omitempty"`
	Avg             int            `json:"avg,omitempty"`
	P75             int            `json:"p75,omitempty"`
	Max             int            `json:"max,omitempty"`
	RatingBreakdown map[string]int `json:"rating_breakdown,omitempty"`
	SampledListings int            `json:"sampled_listings"`
	TotalMatches    int            `json:"total_matches"`
}

const maxPriceAnalysisPages = 5

// PriceAnalysis searches with the given filters and computes price stats
// over up to maxPriceAnalysisPages result pages.
func (s *Service) PriceAnalysis(ctx context.Context, p SearchParams) (*PriceStats, error) {
	var prices []int
	ratings := map[string]int{}
	total := 0
	for page := 1; page <= maxPriceAnalysisPages; page++ {
		p.Page = page
		res, err := s.Search(ctx, p)
		if err != nil {
			return nil, err
		}
		total = res.TotalCount
		for _, l := range res.Listings {
			if l.PriceEUR > 0 {
				prices = append(prices, l.PriceEUR)
			}
			if l.PriceRating != "" {
				ratings[l.PriceRating]++
			}
		}
		if page >= res.PageCount || len(res.Listings) == 0 {
			break
		}
	}
	stats := ComputePriceStats(prices)
	stats.TotalMatches = total
	stats.SampledListings = len(prices)
	if len(ratings) > 0 {
		stats.RatingBreakdown = ratings
	}
	return stats, nil
}

// ComputePriceStats computes nearest-rank percentile stats over prices.
func ComputePriceStats(prices []int) *PriceStats {
	s := &PriceStats{Count: len(prices)}
	if len(prices) == 0 {
		return s
	}
	sorted := slices.Clone(prices)
	slices.Sort(sorted)
	n := len(sorted)
	rank := func(pct int) int {
		r := max((pct*n+99)/100, 1) // ceil(pct/100 * n), at least 1
		return sorted[r-1]
	}
	sum := 0
	for _, v := range sorted {
		sum += v
	}
	s.Min = sorted[0]
	s.Max = sorted[n-1]
	s.P25 = rank(25)
	s.Median = rank(50)
	s.P75 = rank(75)
	s.Avg = sum / n
	return s
}
