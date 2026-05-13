package handlers

import (
	"net/http"
	"sync"
	"context"
	"errors"
	
	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

type DiscoverHandler struct {
	Meta *meta.Client
}

// Discover godoc
// @Summary     Discover movies or shows
// @Description Returns a list of popular or top-rated movies/shows. Results are cached for 1 hour.
// @Tags        discover
// @Produce     json
// @Param       type query string true  "Media type"  Enums(movie, series)
// @Param       sort query string true  "Sort order"  Enums(popular, top_rated)
// @Success     200 {object} models.DiscoverResponse
// @Failure     400 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /discover [get]
func (h *DiscoverHandler) Discover(w http.ResponseWriter, r *http.Request) {
	mediaType := r.URL.Query().Get("type")
	sort      := r.URL.Query().Get("sort")

	if mediaType != "movie" && mediaType != "series" {
		jsonError(w, `type must be "movie" or "series"`, http.StatusBadRequest)
		return
	}
	if sort != "popular" && sort != "top_rated" {
		jsonError(w, `sort must be "popular" or "top_rated"`, http.StatusBadRequest)
		return
	}

	var (
		items []models.DiscoverItem
		err   error
	)

	switch {
	case mediaType == "movie" && sort == "popular":
		items, err = h.Meta.PopularMovies(r.Context())
	case mediaType == "movie" && sort == "top_rated":
		items, err = h.Meta.TopRatedMovies(r.Context())
	case mediaType == "series" && sort == "popular":
		items, err = h.Meta.PopularShows(r.Context())
	case mediaType == "series" && sort == "top_rated":
		items, err = h.Meta.TopRatedShows(r.Context())
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

	jsonOK(w, detail)
}