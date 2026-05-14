package meta

import (
	"sort"
	"strconv"
	"strings"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

// this function merges the search data and sorts it by year.
func MergeAndWeight(movies, series []models.DiscoverItem) []models.DiscoverItem {
	merged := make([]models.DiscoverItem, 0, len(movies)+len(series))
	merged = append(merged, movies...)
	merged = append(merged, series...)

	sort.SliceStable(merged, func(i, j int) bool {
		return parseYear(merged[i].Year) > parseYear(merged[j].Year)
	})

	return merged
}

func parseYear(s string) int {
	if s == "" {
		return 0
	}

	// "2015–2018" -> "2018", "1994" -> "1994"
	s = strings.TrimSpace(s)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '–' || r == '—'
	})

	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if y, err := strconv.Atoi(part); err == nil && y > 1880 && y < 2200 {
			return y
		}
	}

	return 0
}