package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"os"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const cacheTTL = time.Hour

// this holds cached data and its expiration time.
type cacheEntry struct {
	items     []models.DiscoverItem
	expiresAt time.Time
}

func (e *cacheEntry) isExpired() bool {
	return time.Now().After(e.expiresAt)
}

// sync.RWMutex allows multiple concurrent readers but only one writer.
type Client struct {
	httpClient *http.Client
	baseURL    string
	mu         sync.RWMutex
	cache      map[string]cacheEntry
}

func NewClient() *Client {
	baseURL := os.Getenv("META_BASE_URL")

	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:   baseURL,
		cache:      make(map[string]cacheEntry),
	}
}

func (c *Client) Fetch(ctx context.Context, url string) ([]models.DiscoverItem, error) {
	// check cache first
	c.mu.RLock()
	entry, found := c.cache[url]
	c.mu.RUnlock()

	if found && !entry.isExpired() {
		return entry.items, nil
	}

	// if no cache or expired then fetch data.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("meta returned %d for %s", resp.StatusCode, url)
	}

	var catalog CatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	items := transform(catalog.Metas)

	// write the data to cache.
	c.mu.Lock()
	c.cache[url] = cacheEntry{
		items:     items,
		expiresAt: time.Now().Add(cacheTTL),
	}
	c.mu.Unlock()

	return items, nil
}

func (c *Client) PopularMovies(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Fetch(ctx, fmt.Sprintf("%s/catalog/movie/top.json", c.baseURL))
}

func (c *Client) PopularShows(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Fetch(ctx, fmt.Sprintf("%s/catalog/series/top.json", c.baseURL))
}

func (c *Client) TopRatedMovies(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Fetch(ctx, fmt.Sprintf("%s/catalog/movie/imdbRating.json", c.baseURL))
}

func (c *Client) TopRatedShows(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Fetch(ctx, fmt.Sprintf("%s/catalog/series/imdbRating.json", c.baseURL))
}

// transform to fit DiscoverItem shape.
func transform(metas []Meta) []models.DiscoverItem {
	items := make([]models.DiscoverItem, 0, len(metas))
	for _, m := range metas {
		items = append(items, models.DiscoverItem{
			ID:         m.ID,
			Type:       m.Type,
			Title:      m.Name,
			Poster:     m.Poster,
			ImdbRating: m.ImdbRating,
			Year:       m.Year,
		})
	}
	return items
}
