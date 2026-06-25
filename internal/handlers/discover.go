package handlers

import (
	"net/http"
	"sync"
	"context"
	"errors"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

type DiscoverHandler struct {
	Meta    *meta.Client
	Tracker *tracking.Service
}

// catalogSort maps the public sort values to the catalog names.
var catalogSort = map[string]string{
	"popular":   "top",
	"top_rated": "imdbRating",
}

// Discover godoc
// @Summary     Discover movies or shows
// @Description Returns a single catalog of movies/shows. Use `sort` (with optional `genre`) for the popular/top-rated catalogs, `year` for one release year, or `provider` for a streaming service. `provider` takes precedence over `year`, which takes precedence over `sort`. Results are cached for 1 hour.
// @Tags        discover
// @Produce     json
// @Param       type     query string true  "Media type" Enums(movie, series)
// @Param       sort     query string false "Sort order (default popular)" Enums(popular, top_rated)
// @Param       genre    query string false "Genre filter, e.g. action, sci-fi (series also: reality-tv, talk-show, game-show)"
// @Param       year     query string false "Release year, e.g. 2025 (overrides sort)"
// @Param       provider query string false "Streaming provider (overrides sort and year)" Enums(netflix, hbomax, disney, prime, appletv)
// @Success     200 {object} models.DiscoverResponse
// @Failure     400 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /discover [get]
func (h *DiscoverHandler) Discover(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mediaType := q.Get("type")
	sort := q.Get("sort")
	genre := strings.ToLower(strings.TrimSpace(q.Get("genre")))
	year := strings.TrimSpace(q.Get("year"))
	provider := strings.ToLower(strings.TrimSpace(q.Get("provider")))

	if mediaType != "movie" && mediaType != "series" {
		jsonError(w, `type must be "movie" or "series"`, http.StatusBadRequest)
		return
	}

	var (
		items []models.DiscoverItem
		err   error
	)

	switch {
	case provider != "":
		code, ok := providerCode(provider)
		if !ok {
			jsonError(w, "unknown provider", http.StatusBadRequest)
			return
		}
		items, err = h.Meta.ProviderCatalog(r.Context(), mediaType, code)

	case year != "":
		if !validYear(year) {
			jsonError(w, "year must be a 4-digit year", http.StatusBadRequest)
			return
		}
		items, err = h.Meta.CatalogByYear(r.Context(), mediaType, year)

	default:
		if sort == "" {
			sort = "popular"
		}
		catalog, ok := catalogSort[sort]
		if !ok {
			jsonError(w, `sort must be "popular" or "top_rated"`, http.StatusBadRequest)
			return
		}
		if genre != "" && !validGenre(mediaType, genre) {
			jsonError(w, "unknown genre", http.StatusBadRequest)
			return
		}
		items, err = h.Meta.Catalog(r.Context(), mediaType, catalog, genre)
	}

	if err != nil {
		jsonError(w, "could not fetch catalog", http.StatusBadGateway)
		return
	}

	jsonOK(w, models.DiscoverResponse{Items: items})
}

// DiscoverAll godoc
// @Summary     Discover all catalogs
// @Description Returns all four catalogs (popular movies, popular shows, top-rated movies, top-rated shows) in a single request. All results are cached for 1 hour.
// @Tags        discover
// @Produce     json
// @Success     200 {object} models.DiscoverAllResponse
// @Failure     502 {object} map[string]string
// @Router      /discover/all [get]
func (h *DiscoverHandler) DiscoverAll(w http.ResponseWriter, r *http.Request) {
	var (
		wg             sync.WaitGroup
		popularMovies  []models.DiscoverItem
		popularShows   []models.DiscoverItem
		topRatedMovies []models.DiscoverItem
		topRatedShows  []models.DiscoverItem
		fetchErr       error
		errMu          sync.Mutex
	)

	fetch := func(fn func(context.Context) ([]models.DiscoverItem, error), dest *[]models.DiscoverItem) {
		defer wg.Done()
		result, err := fn(r.Context())
		if err != nil {
			errMu.Lock()
			fetchErr = err
			errMu.Unlock()
			return
		}
		*dest = result
	}

	wg.Add(4)
	go fetch(h.Meta.PopularMovies,  &popularMovies)
	go fetch(h.Meta.PopularShows,   &popularShows)
	go fetch(h.Meta.TopRatedMovies, &topRatedMovies)
	go fetch(h.Meta.TopRatedShows,  &topRatedShows)
	wg.Wait()

	if fetchErr != nil {
		jsonError(w, "could not fetch catalogs", http.StatusBadGateway)
		return
	}

	jsonOK(w, models.DiscoverAllResponse{
		PopularMovies:  popularMovies,
		PopularShows:   popularShows,
		TopRatedMovies: topRatedMovies,
		TopRatedShows:  topRatedShows,
	})
}

// MovieDetail godoc
// @Summary     Get movie details
// @Description Returns full metadata for a movie by ID. Results are cached for 24 hours.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "ID (e.g. tt0111161)"
// @Success     200 {object} models.MovieDetail
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/movie/{id} [get]
func (h *DiscoverHandler) MovieDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	detail, err := h.Meta.MovieDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "movie not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch movie details", http.StatusBadGateway)
		return
	}

	h.trackView(r, id, "movie", detail.Name)
	jsonOK(w, detail)
}

// SeriesDetail godoc
// @Summary     Get series details
// @Description Returns full metadata for a series by ID, including all episodes in the videos array. Results are cached for 24 hours.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "ID (e.g. tt3322312)"
// @Success     200 {object} models.SeriesDetail
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/series/{id} [get]
func (h *DiscoverHandler) SeriesDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	detail, err := h.Meta.SeriesDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "series not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch series details", http.StatusBadGateway)
		return
	}

	h.trackView(r, id, "tv", detail.Name)
	jsonOK(w, detail)
}

// Search godoc
// @Summary     Search movies and shows
// @Description Searches for movies and/or shows by query string. With no type filter, returns mixed results weighted by recency.
// @Tags        search
// @Produce     json
// @Param       q    query string true  "Search query"
// @Param       type query string false "Filter by type" Enums(movie, series)
// @Success     200 {object} models.SearchResponse
// @Failure     400 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /search [get]
func (h *DiscoverHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		jsonError(w, "q is required", http.StatusBadRequest)
		return
	}

	mediaType := r.URL.Query().Get("type")
	if mediaType != "" && mediaType != "movie" && mediaType != "series" {
		jsonError(w, `type must be "movie" or "series"`, http.StatusBadRequest)
		return
	}

	switch mediaType {
	case "movie":
		items, err := h.Meta.SearchMovies(r.Context(), query)
		if err != nil {
			jsonError(w, "search failed", http.StatusBadGateway)
			return
		}
		h.trackSearch(r, query, "movie", len(items))
		jsonOK(w, models.SearchResponse{Items: items, Query: query, Type: "movie"})

	case "series":
		items, err := h.Meta.SearchSeries(r.Context(), query)
		if err != nil {
			jsonError(w, "search failed", http.StatusBadGateway)
			return
		}
		h.trackSearch(r, query, "series", len(items))
		jsonOK(w, models.SearchResponse{Items: items, Query: query, Type: "series"})

	default:
		var (
			wg      sync.WaitGroup
			movies  []models.DiscoverItem
			series  []models.DiscoverItem
			fetchErr error
			errMu   sync.Mutex
		)

		fetch := func(fn func(context.Context, string) ([]models.DiscoverItem, error), dest *[]models.DiscoverItem) {
			defer wg.Done()
			result, err := fn(r.Context(), query)
			if err != nil {
				errMu.Lock()
				fetchErr = err
				errMu.Unlock()
				return
			}
			*dest = result
		}

		wg.Add(2)
		go fetch(h.Meta.SearchMovies,  &movies)
		go fetch(h.Meta.SearchSeries,  &series)
		wg.Wait()

		if fetchErr != nil {
			jsonError(w, "search failed", http.StatusBadGateway)
			return
		}

		merged := meta.MergeAndWeight(movies, series)
		h.trackSearch(r, query, "", len(merged))
		jsonOK(w, models.SearchResponse{Items: merged, Query: query})
	}
}

// trackSearch records a search event when the request is from a logged-in user.
func (h *DiscoverHandler) trackSearch(r *http.Request, query, mediaType string, results int) {
	if h.Tracker == nil {
		return
	}
	claims := middleware.ClaimsFromCtx(r)
	if claims == nil {
		return
	}
	md := map[string]any{"query": query, "results": results}
	if mediaType != "" {
		md["type"] = mediaType
	}
	h.Tracker.Record(r.Context(), models.UserEvent{
		UserID:    claims.UserID,
		EventType: models.EventSearch,
		Metadata:  md,
	})
}

// trackView records a detail-page view for a logged-in user.
func (h *DiscoverHandler) trackView(r *http.Request, id, mediaType, title string) {
	if h.Tracker == nil {
		return
	}
	claims := middleware.ClaimsFromCtx(r)
	if claims == nil {
		return
	}
	h.Tracker.Record(r.Context(), models.UserEvent{
		UserID:    claims.UserID,
		EventType: models.EventView,
		ItemID:    id,
		MediaType: mediaType,
		ImdbID:    id,
		Title:     title,
	})
}