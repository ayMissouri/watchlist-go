package handlers

import (
	"net/http/httptest"
	"net/http"
	"testing"
)

func TestParseWatchlistQuery_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist", nil)
	q := parseWatchlistQuery(req)

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
}

func TestParseWatchlistQuery_InvalidTypeIgnored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist?type=anime", nil)
	q := parseWatchlistQuery(req)

	if q.Type != "" {
		t.Errorf("expected empty type, got %s", q.Type)
	}
}

func TestParseWatchlistQuery_PerPageCappedAt100(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/watchlist?per_page=999", nil)
	q := parseWatchlistQuery(req)

	if q.PerPage != 100 {
		t.Errorf("expected per_page capped at 100, got %d", q.PerPage)
	}
}