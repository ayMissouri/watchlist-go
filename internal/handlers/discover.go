package handlers

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

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

// Discover godoc
// @Summary     Discover movies or shows
// @Description Returns one catalog. Use `sort` (optionally with `genre`) for popular/top-rated, `year` for a single release year, or `provider` for a streaming service. If you send more than one, `provider` wins over `year`, which wins over `sort`. Cached for an hour.
// @Description Anime comes from MyAnimeList and only takes `sort`, which also allows `airing` and `upcoming` there.
// @Tags        discover
// @Produce     json
// @Param       type     query string true  "Media type" Enums(movie, series, anime)
// @Param       sort     query string false "Sort order (default popular)" Enums(popular, top_rated, airing, upcoming)
// @Param       genre    query string false "Genre filter. Movies: action, adventure, animation, comedy, crime, documentary, drama, family, fantasy, history, horror, mystery, romance, sci-fi, thriller, war, western. Series: action, adventure, animation, comedy, crime, documentary, drama, family, fantasy, mystery, sci-fi, war, western, reality-tv, talk-show"
// @Param       year     query string false "Release year, e.g. 2025 (overrides sort)"
// @Param       provider query string false "Streaming provider (overrides sort and year)" Enums(netflix, hbomax, disney, prime, appletv)
// @Param       title    query string false "Anime title language (default romaji, used when MAL has no title in that language)" Enums(romaji, english, native)
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

	if mediaType != "movie" && mediaType != "series" && mediaType != "anime" {
		jsonError(w, `type must be "movie", "series" or "anime"`, http.StatusBadRequest)
		return
	}

	var (
		items []models.DiscoverItem
		err   error
	)

	switch {
	case mediaType == "anime":
		ranking, ok := animeRankings[cmp.Or(sort, "popular")]
		if !ok || genre != "" || year != "" || provider != "" {
			jsonError(w, `anime only supports sort: "popular", "top_rated", "airing" or "upcoming"`, http.StatusBadRequest)
			return
		}
		items, err = h.Meta.AnimeCatalog(r.Context(), ranking, q.Get("title"))

	case provider != "":
		id, ok := providerID(provider)
		if !ok {
			jsonError(w, "unknown provider", http.StatusBadRequest)
			return
		}
		items, err = h.Meta.ProviderCatalog(r.Context(), mediaType, id)

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
		if sort != "popular" && sort != "top_rated" {
			jsonError(w, `sort must be "popular" or "top_rated"`, http.StatusBadRequest)
			return
		}
		var gid int
		if genre != "" {
			var ok bool
			if gid, ok = genreID(mediaType, genre); !ok {
				if _, movieOnly := movieGenres[genre]; movieOnly {
					jsonOK(w, models.DiscoverResponse{Items: []models.DiscoverItem{}})
					return
				}
				jsonError(w, "unknown genre", http.StatusBadRequest)
				return
			}
		}
		items, err = h.Meta.Catalog(r.Context(), mediaType, sort, gid)
	}

	if err != nil {
		jsonError(w, "could not fetch catalog", http.StatusBadGateway)
		return
	}

	jsonOK(w, models.DiscoverResponse{Items: items})
}

// DiscoverAll godoc
// @Summary     Discover all catalogs
// @Description Popular movies, popular shows, top-rated movies and top-rated shows, all in one call. Cached for an hour.
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
	go fetch(h.Meta.PopularMovies, &popularMovies)
	go fetch(h.Meta.PopularShows, &popularShows)
	go fetch(h.Meta.TopRatedMovies, &topRatedMovies)
	go fetch(h.Meta.TopRatedShows, &topRatedShows)
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
// @Description Everything we know about a movie. Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "TMDB id (e.g. 278). IMDb ids like tt0111161 still work."
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

	h.trackView(r, id, detail.ImdbID, "movie", detail.Name)
	jsonOK(w, detail)
}

// SeriesDetail godoc
// @Summary     Get series details
// @Description Everything we know about a series, episodes included (they're in `videos`). Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "TMDB id (e.g. 1396). IMDb ids like tt0903747 still work."
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

	h.trackView(r, id, detail.ImdbID, "tv", detail.Name)
	jsonOK(w, detail)
}

// AnimeDetail godoc
// @Summary     Get anime details
// @Description Everything MyAnimeList has on an anime. There's no episode list, only the `episodes` count. Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       id    path  string true  "MyAnimeList id (e.g. 5114)"
// @Param       title query string false "Anime title language (default romaji, used when MAL has no title in that language)" Enums(romaji, english, native)
// @Success     200 {object} models.AnimeDetail
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/anime/{id} [get]
func (h *DiscoverHandler) AnimeDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	detail, err := h.Meta.AnimeDetail(r.Context(), id, r.URL.Query().Get("title"))
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "anime not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch anime details", http.StatusBadGateway)
		return
	}

	h.trackView(r, id, "", "anime", detail.Name)
	jsonOK(w, detail)
}

// AnimeEpisodes godoc
// @Summary     Get anime episodes
// @Description The episode list for an anime, all seasons flattened into season 1, the way MyAnimeList counts them.
// @Description Titles and air dates come from a self-hosted Jikan instance (JIKAN_URL). When that isn't configured
// @Description or can't be reached, episodes come back numbered ("Episode 1") with no air date, so the count is
// @Description still right. Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "MyAnimeList id (e.g. 5114)"
// @Success     200 {object} models.EpisodesResponse
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/anime/{id}/episodes [get]
func (h *DiscoverHandler) AnimeEpisodes(w http.ResponseWriter, r *http.Request) {
	items, err := h.Meta.AnimeEpisodes(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "anime not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch episodes", http.StatusBadGateway)
		return
	}
	if items == nil {
		items = []models.Episode{}
	}
	jsonOK(w, models.EpisodesResponse{Items: items})
}

// Recommendations godoc
// @Summary     Get recommendations for a title
// @Description What TMDB (or MyAnimeList, for anime) recommends alongside this title, in the same shape as a discover catalog. Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       type path     string true "Media type" Enums(movie, series, anime)
// @Param       id   path     string true "TMDB id (e.g. 278), MAL id for anime. IMDb ids like tt0111161 still work for movies and series."
// @Success     200 {object} models.DiscoverResponse
// @Failure     400 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/{type}/{id}/recommendations [get]
func (h *DiscoverHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	mediaType := chi.URLParam(r, "type")
	id := chi.URLParam(r, "id")

	var (
		items []models.DiscoverItem
		err   error
	)
	switch mediaType {
	case "movie", "series":
		items, err = h.Meta.Recommendations(r.Context(), mediaType, id)
	case "anime":
		items, err = h.Meta.AnimeRecommendations(r.Context(), id)
	default:
		jsonError(w, `type must be "movie", "series" or "anime"`, http.StatusBadRequest)
		return
	}
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "title not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch recommendations", http.StatusBadGateway)
		return
	}
	jsonOK(w, models.DiscoverResponse{Items: items})
}

// PersonDetail godoc
// @Summary     Get person details
// @Description Bio and the most popular acting credits of a cast member. Ids come from `credits` on a movie or series detail; each credit is keyed like a discover item so it links to a detail page. Cached for a day.
// @Tags        meta
// @Produce     json
// @Param       id  path     string true "TMDB person id (e.g. 2524)"
// @Success     200 {object} models.Person
// @Failure     404 {object} map[string]string
// @Failure     502 {object} map[string]string
// @Router      /meta/person/{id} [get]
func (h *DiscoverHandler) PersonDetail(w http.ResponseWriter, r *http.Request) {
	person, err := h.Meta.PersonDetail(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			jsonError(w, "person not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not fetch person details", http.StatusBadGateway)
		return
	}
	jsonOK(w, person)
}

// Search godoc
// @Summary     Search movies and shows
// @Description Search by title. Leave `type` out and you get movies and shows mixed together, newest first.
// @Description Anime is only searched with `type=anime` (MyAnimeList needs at least 3 characters).
// @Tags        search
// @Produce     json
// @Param       q    query string true  "Search query"
// @Param       type query string false "Filter by type" Enums(movie, series, anime)
// @Param       title query string false "Anime title language (default romaji, used when MAL has no title in that language)" Enums(romaji, english, native)
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
	if mediaType != "" && mediaType != "movie" && mediaType != "series" && mediaType != "anime" {
		jsonError(w, `type must be "movie", "series" or "anime"`, http.StatusBadRequest)
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

	case "anime":
		items, err := h.Meta.SearchAnime(r.Context(), query, r.URL.Query().Get("title"))
		if err != nil {
			jsonError(w, "search failed", http.StatusBadGateway)
			return
		}
		h.trackSearch(r, query, "anime", len(items))
		jsonOK(w, models.SearchResponse{Items: items, Query: query, Type: "anime"})

	default:
		var (
			wg       sync.WaitGroup
			movies   []models.DiscoverItem
			series   []models.DiscoverItem
			fetchErr error
			errMu    sync.Mutex
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
		go fetch(h.Meta.SearchMovies, &movies)
		go fetch(h.Meta.SearchSeries, &series)
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
func (h *DiscoverHandler) trackView(r *http.Request, id, imdbID, mediaType, title string) {
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
		ImdbID:    cmp.Or(imdbID, id),
		Title:     title,
	})
}
