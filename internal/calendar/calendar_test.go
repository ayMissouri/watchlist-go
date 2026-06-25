package calendar

import (
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestParseReleaseDate(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		wantOK bool
		wantYr int
	}{
		{"rfc3339 with ms", "2025-07-15T00:00:00.000Z", true, 2025},
		{"rfc3339", "2025-07-15T00:00:00Z", true, 2025},
		{"date only", "2025-07-15", true, 2025},
		{"year and month", "2025-07", true, 2025},
		{"year only", "2027", true, 2027},
		{"empty", "", false, 0},
		{"whitespace", "   ", false, 0},
		{"tba", "TBA", false, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseReleaseDate(c.in)
			if ok != c.wantOK {
				t.Fatalf("parseReleaseDate(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			}
			if ok && got.Year() != c.wantYr {
				t.Fatalf("parseReleaseDate(%q) year = %d, want %d", c.in, got.Year(), c.wantYr)
			}
		})
	}
}

func TestParseMovieRelease_FallsBackThroughFields(t *testing.T) {
	d := &models.MovieDetail{Year: "2030"}
	got, ok := parseMovieRelease(d)
	if !ok {
		t.Fatal("expected a parsed release from Year fallback")
	}
	if got.Year() != 2030 {
		t.Fatalf("year = %d, want 2030", got.Year())
	}

	d = &models.MovieDetail{Released: "2031-05-01T00:00:00Z", ReleaseInfo: "2032", Year: "2033"}
	got, ok = parseMovieRelease(d)
	if !ok || got.Year() != 2031 {
		t.Fatalf("expected Released to win, got year %d ok %v", got.Year(), ok)
	}
}

func TestReleaseBody(t *testing.T) {
	tv := models.CalendarEntry{MediaType: "tv", Season: 2, Episode: 5, EpisodeTitle: "The Return"}
	if got := releaseBody(tv); got != "S2E5 · The Return is now available" {
		t.Fatalf("tv body = %q", got)
	}

	tvNoTitle := models.CalendarEntry{MediaType: "tv", Season: 1, Episode: 1}
	if got := releaseBody(tvNoTitle); got != "S1E1 is now available" {
		t.Fatalf("tv body (no title) = %q", got)
	}

	movie := models.CalendarEntry{MediaType: "movie"}
	if got := releaseBody(movie); got != "Now available" {
		t.Fatalf("movie body = %q", got)
	}
}
