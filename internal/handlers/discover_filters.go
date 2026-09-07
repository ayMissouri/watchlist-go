package handlers

import "strconv"

var movieGenres = map[string]int{
	"action": 28, "adventure": 12, "animation": 16, "comedy": 35, "crime": 80,
	"documentary": 99, "drama": 18, "family": 10751, "fantasy": 14, "history": 36,
	"horror": 27, "mystery": 9648, "romance": 10749, "sci-fi": 878, "thriller": 53,
	"war": 10752, "western": 37,
}

var seriesGenres = map[string]int{
	"action": 10759, "adventure": 10759, "animation": 16, "comedy": 35, "crime": 80,
	"documentary": 99, "drama": 18, "family": 10751, "fantasy": 10765, "mystery": 9648,
	"sci-fi": 10765, "war": 10768, "western": 37, "reality-tv": 10764, "talk-show": 10767,
}

func genreID(mediaType, genre string) (int, bool) {
	if mediaType == "series" {
		id, ok := seriesGenres[genre]
		return id, ok
	}
	id, ok := movieGenres[genre]
	return id, ok
}

var streamingProviders = map[string]int{
	"netflix": 8, "nfx": 8,
	"prime": 9, "amp": 9,
	"disney": 337, "dnp": 337,
	"appletv": 350, "atp": 350,
	"hbomax": 1899, "hbm": 1899,
}

func providerID(provider string) (int, bool) {
	id, ok := streamingProviders[provider]
	return id, ok
}

// validYear reports whether year is a plausible 4-digit release year.
func validYear(year string) bool {
	if len(year) != 4 {
		return false
	}
	n, err := strconv.Atoi(year)
	if err != nil {
		return false
	}
	return n >= 1900 && n <= 2100
}
