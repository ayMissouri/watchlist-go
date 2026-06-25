package meta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestCacheEntry_IsExpired(t *testing.T) {
	fresh := cacheEntry{
		items:     []models.DiscoverItem{},
		expiresAt: time.Now().Add(time.Hour),
	}
	if fresh.isExpired() {
		t.Error("expected fresh entry to not be expired")
	}

	stale := cacheEntry{
		items:     []models.DiscoverItem{},
		expiresAt: time.Now().Add(-time.Minute),
	}
	if !stale.isExpired() {
		t.Error("expected stale entry to be expired")
	}
}

func TestTransform(t *testing.T) {
	metas := []Meta{
		{ID: "tt0111161", Type: "movie", Name: "The Shawshank Redemption", ImdbRating: "9.3"},
	}

	items := transform(metas)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "The Shawshank Redemption" {
		t.Errorf("expected title 'The Shawshank Redemption', got %s", items[0].Title)
	}
	if items[0].ImdbRating != "9.3" {
		t.Errorf("expected imdb_rating '9.3', got %s", items[0].ImdbRating)
	}
}

func newCatalogTestClient(t *testing.T) (*Client, *string) {
	t.Helper()

	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"metas":[{"id":"tt1","type":"movie","name":"Test"}]}`))
	}))
	t.Cleanup(srv.Close)

	return &Client{
		httpClient:   srv.Client(),
		baseURL:      srv.URL,
		streamingURL: srv.URL,
		cache:        make(map[string]cacheEntry),
	}, &lastPath
}

func TestCatalogURLs(t *testing.T) {
	tests := []struct {
		name string
		call func(c *Client) error
		want string
	}{
		{
			name: "popular by genre",
			call: func(c *Client) error {
				_, err := c.Catalog(context.Background(), "movie", "top", "action")
				return err
			},
			want: "/catalog/movie/top/genre=action.json",
		},
		{
			name: "top-rated by hyphenated genre",
			call: func(c *Client) error {
				_, err := c.Catalog(context.Background(), "series", "imdbRating", "sci-fi")
				return err
			},
			want: "/catalog/series/imdbRating/genre=sci-fi.json",
		},
		{
			name: "no genre",
			call: func(c *Client) error {
				_, err := c.Catalog(context.Background(), "movie", "top", "")
				return err
			},
			want: "/catalog/movie/top.json",
		},
		{
			name: "by year",
			call: func(c *Client) error {
				_, err := c.CatalogByYear(context.Background(), "movie", "2025")
				return err
			},
			want: "/catalog/movie/year/genre=2025.json",
		},
		{
			name: "provider",
			call: func(c *Client) error {
				_, err := c.ProviderCatalog(context.Background(), "series", "nfx")
				return err
			},
			want: "/catalog/series/nfx.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, lastPath := newCatalogTestClient(t)
			if err := tt.call(c); err != nil {
				t.Fatalf("call returned error: %v", err)
			}
			if *lastPath != tt.want {
				t.Errorf("requested %q, want %q", *lastPath, tt.want)
			}
		})
	}
}