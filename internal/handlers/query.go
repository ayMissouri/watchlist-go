package handlers

import (
	"net/http"
	"strconv"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

func parseWatchlistQuery(r *http.Request) models.WatchlistQuery {
	q := models.WatchlistQuery{
		Page:    defaultPage,
		PerPage: defaultPerPage,
		Sort:    "last_updated",
		Order:   "desc",
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			q.Page = v
		}
	}

	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 {
			if v > maxPerPage {
				v = maxPerPage
			}
			q.PerPage = v
		}
	}

	// Whitelist of query parameters to prevent SQL injection.
	if t := r.URL.Query().Get("type"); t == "tv" || t == "movie" {
		q.Type = t
	}

	if s := models.WatchlistStatus(r.URL.Query().Get("status")); s.Valid() {
		q.Status = string(s)
	}

	if s := r.URL.Query().Get("sort"); s == "title" || s == "last_updated" {
		q.Sort = s
	}

	if o := r.URL.Query().Get("order"); o == "asc" || o == "desc" {
		q.Order = o
	}

	return q
}