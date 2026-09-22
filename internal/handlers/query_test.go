package handlers

import (
	"net/http/httptest"
	"net/http"
	"slices"
	"testing"
)

var mainTypes = []string{"tv", "movie"}

func TestParseWatchlistQuery_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist", nil)
	q := parseWatchlistQuery(req, mainTypes)

	if q.Page != 1 {
		t.Errorf("expected page 1, got %d", q.Page)
	}
	if q.PerPage != 20 {
		t.Errorf("expected per_page 20, got %d", q.PerPage)
	}
	if q.Sort != "last_updated" {
		t.Errorf("expected sort last_updated, got %s", q.Sort)
	}
	if q.Order != "desc" {
		t.Errorf("expected order desc, got %s", q.Order)
	}
	if !slices.Equal(q.Types, mainTypes) {
		t.Errorf("expected types %v, got %v", mainTypes, q.Types)
	}
}

func TestParseWatchlistQuery_InvalidTypeIgnored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist?type=book", nil)
	q := parseWatchlistQuery(req, mainTypes)

	if !slices.Equal(q.Types, mainTypes) {
		t.Errorf("expected types %v, got %v", mainTypes, q.Types)
	}
}

func TestParseWatchlistQuery_TypeStaysInItsWatchlist(t *testing.T) {
	cases := []struct {
		url   string
		types []string
		want  []string
	}{
		{"/watchlist?type=movie", mainTypes, []string{"movie"}},
		{"/watchlist?type=anime", mainTypes, mainTypes},
		{"/anime/watchlist?type=tv", []string{"anime"}, []string{"anime"}},
	}
	for _, c := range cases {
		q := parseWatchlistQuery(httptest.NewRequest(http.MethodGet, c.url, nil), c.types)
		if !slices.Equal(q.Types, c.want) {
			t.Errorf("%s: expected types %v, got %v", c.url, c.want, q.Types)
		}
	}
}

func TestParseWatchlistQuery_PerPageCappedAt100(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist?per_page=999", nil)
	q := parseWatchlistQuery(req, mainTypes)

	if q.PerPage != 100 {
		t.Errorf("expected per_page capped at 100, got %d", q.PerPage)
	}
}

func TestParseMediaTypes(t *testing.T) {
	cases := map[string][]string{
		"":               nil,
		"anime":          {"anime"},
		"tv,movie":       {"tv", "movie"},
		"tv,book,,anime": {"tv", "anime"},
		"book":           nil,
	}
	for in, want := range cases {
		if got := parseMediaTypes(in); !slices.Equal(got, want) {
			t.Errorf("parseMediaTypes(%q) = %v, want %v", in, got, want)
		}
	}
}
