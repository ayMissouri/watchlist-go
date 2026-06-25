package handlers

import "strconv"

// movieGenres are the genre slugs for the movie catalogs.
var movieGenres = map[string]bool{
	"action": true, "adventure": true, "animation": true, "biography": true,
	"comedy": true, "crime": true, "documentary": true, "drama": true,
	"family": true, "fantasy": true, "history": true, "horror": true,
	"mystery": true, "romance": true, "sci-fi": true, "sport": true,
	"thriller": true, "war": true, "western": true,
}

// seriesExtraGenres are the TV genres for series on top of the movie genres.
var seriesExtraGenres = map[string]bool{
	"reality-tv": true, "talk-show": true, "game-show": true,
}

func validGenre(mediaType, genre string) bool {
	if movieGenres[genre] {
		return true
	}
	return mediaType == "series" && seriesExtraGenres[genre]
}

// streamingProviders maps a friendly provider name to the catalog short code.
var streamingProviders = map[string]string{
	"netflix": "nfx",
	"hbomax":  "hbm",
	"disney":  "dnp",
	"prime":   "amp",
	"appletv": "atp",
}

func providerCode(provider string) (string, bool) {
	if code, ok := streamingProviders[provider]; ok {
		return code, true
	}
	for _, code := range streamingProviders {
		if provider == code {
			return code, true
		}
	}
	return "", false
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
