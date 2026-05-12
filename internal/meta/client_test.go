package meta

import (
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