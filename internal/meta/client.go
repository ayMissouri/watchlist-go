package meta

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	cacheTTL       = time.Hour
	detailCacheTTL = 24 * time.Hour

	imageBase = "https://image.tmdb.org/t/p/"

	catalogPages      = 2
	seasonsPerRequest = 20
	maxInFlight       = 8
	maxCast           = 20
	maxCredits        = 40
)

var (
	ErrNotFound    = errors.New("not found")
	errRateLimited = errors.New("rate limited")
	retryDelay     = time.Second
)

type cacheEntry struct {
	val       any
	expiresAt time.Time
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	region     string
	sem        chan struct{}

	mu      sync.RWMutex
	cache   map[string]cacheEntry
	imdbIDs sync.Map
}

func NewClient() *Client {
	key := os.Getenv("TMDB_API_KEY")
	if key == "" {
		log.Print("meta: TMDB_API_KEY is not set, every TMDB call will fail")
	}
	return newClient("https://api.themoviedb.org/3", key, cmp.Or(os.Getenv("TMDB_REGION"), "US"))
}

func newClient(baseURL, apiKey, region string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
		region:     region,
		sem:        make(chan struct{}, maxInFlight),
		cache:      map[string]cacheEntry{},
	}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	params := url.Values{}
	maps.Copy(params, q)
	params.Set("api_key", c.apiKey)
	u := c.baseURL + path + "?" + params.Encode()

	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.sem }()

	err := c.getOnce(ctx, u, path, out)
	if errors.Is(err, errRateLimited) {
		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
		err = c.getOnce(ctx, u, path, out)
	}
	return err
}

func (c *Client) getOnce(ctx context.Context, u, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return errRateLimited
	default:
		return fmt.Errorf("tmdb returned %d for %s", resp.StatusCode, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func cached[T any](c *Client, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	c.mu.RLock()
	e, ok := c.cache[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		if v, ok := e.val.(T); ok {
			return v, nil
		}
	}

	v, err := fetch()
	if err != nil {
		var zero T
		return zero, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{val: v, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
	return v, nil
}

func (c *Client) imdbID(ctx context.Context, t string, id int) (string, error) {
	key := t + "/" + strconv.Itoa(id)
	if v, ok := c.imdbIDs.Load(key); ok {
		return v.(string), nil
	}
	var ext struct {
		ImdbID string `json:"imdb_id"`
	}
	if err := c.get(ctx, "/"+key+"/external_ids", nil, &ext); err != nil && !errors.Is(err, ErrNotFound) {
		return "", err
	}
	c.imdbIDs.Store(key, ext.ImdbID)
	return ext.ImdbID, nil
}

func (c *Client) resolveIDs(ctx context.Context, t string, rows []tmdbListItem) ([]string, error) {
	ids := make([]string, len(rows))
	errs := make([]error, len(rows))
	var wg sync.WaitGroup
	for i, r := range rows {
		wg.Go(func() { ids[i], errs[i] = c.imdbID(ctx, cmp.Or(r.MediaType, t), r.ID) })
	}
	wg.Wait()
	return ids, errors.Join(errs...)
}

func toItem(r tmdbListItem, t, imdbID string) models.DiscoverItem {
	return models.DiscoverItem{
		ID:         imdbID,
		Type:       publicType(cmp.Or(r.MediaType, t)),
		Title:      cmp.Or(r.Title, r.Name),
		Poster:     image("w500", r.PosterPath),
		Background: image("w1280", r.BackdropPath),
		ImdbRating: rating(r.VoteAverage, r.VoteCount),
		Year:       yearOf(cmp.Or(r.ReleaseDate, r.FirstAirDate)),
	}
}

func (c *Client) toItems(ctx context.Context, t string, rows []tmdbListItem) ([]models.DiscoverItem, error) {
	ids, err := c.resolveIDs(ctx, t, rows)
	if err != nil {
		return nil, err
	}
	items := make([]models.DiscoverItem, 0, len(rows))
	seen := map[string]bool{}
	for i, r := range rows {
		if ids[i] != "" && !seen[ids[i]] {
			seen[ids[i]] = true
			items = append(items, toItem(r, t, ids[i]))
		}
	}
	return items, nil
}

func (c *Client) list(ctx context.Context, t, path string, q url.Values, pages int) ([]models.DiscoverItem, error) {
	results := make([][]tmdbListItem, pages)
	errs := make([]error, pages)
	var wg sync.WaitGroup
	for i := range pages {
		wg.Go(func() {
			pq := url.Values{}
			maps.Copy(pq, q)
			pq.Set("page", strconv.Itoa(i+1))
			var page tmdbPage
			errs[i] = c.get(ctx, path, pq, &page)
			results[i] = page.Results
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return c.toItems(ctx, t, slices.Concat(results...))
}

var tvNoise = map[string]bool{"10763": true, "10764": true, "10766": true, "10767": true}

func (c *Client) discover(ctx context.Context, mediaType string, q url.Values) ([]models.DiscoverItem, error) {
	t := tmdbType(mediaType)
	if t == "tv" && !tvNoise[q.Get("with_genres")] {
		q.Set("without_genres", strings.Join(slices.Sorted(maps.Keys(tvNoise)), ","))
	}
	return cached(c, "discover/"+t+"?"+q.Encode(), cacheTTL, func() ([]models.DiscoverItem, error) {
		return c.list(ctx, t, "/discover/"+t, q, catalogPages)
	})
}

func (c *Client) PopularMovies(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Catalog(ctx, "movie", "popular", 0)
}

func (c *Client) PopularShows(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Catalog(ctx, "series", "popular", 0)
}

func (c *Client) TopRatedMovies(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Catalog(ctx, "movie", "top_rated", 0)
}

func (c *Client) TopRatedShows(ctx context.Context) ([]models.DiscoverItem, error) {
	return c.Catalog(ctx, "series", "top_rated", 0)
}

func (c *Client) Catalog(ctx context.Context, mediaType, sort string, genreID int) ([]models.DiscoverItem, error) {
	q := url.Values{"sort_by": {"popularity.desc"}}
	if sort == "top_rated" {
		q.Set("sort_by", "vote_average.desc")
		q.Set("vote_count.gte", "200")
	}
	if genreID != 0 {
		q.Set("with_genres", strconv.Itoa(genreID))
	}
	return c.discover(ctx, mediaType, q)
}

func (c *Client) CatalogByYear(ctx context.Context, mediaType, year string) ([]models.DiscoverItem, error) {
	q := url.Values{"sort_by": {"popularity.desc"}}
	if mediaType == "movie" {
		q.Set("primary_release_year", year)
	} else {
		q.Set("first_air_date_year", year)
	}
	return c.discover(ctx, mediaType, q)
}

func (c *Client) ProviderCatalog(ctx context.Context, mediaType string, providerID int) ([]models.DiscoverItem, error) {
	q := url.Values{
		"sort_by":              {"popularity.desc"},
		"with_watch_providers": {strconv.Itoa(providerID)},
		"watch_region":         {c.region},
	}
	return c.discover(ctx, mediaType, q)
}

func (c *Client) SearchMovies(ctx context.Context, query string) ([]models.DiscoverItem, error) {
	return c.search(ctx, "movie", query)
}

func (c *Client) SearchSeries(ctx context.Context, query string) ([]models.DiscoverItem, error) {
	return c.search(ctx, "tv", query)
}

func (c *Client) search(ctx context.Context, t, query string) ([]models.DiscoverItem, error) {
	var page tmdbPage
	if err := c.get(ctx, "/search/"+t, url.Values{"query": {query}}, &page); err != nil {
		return nil, err
	}
	return c.toItems(ctx, t, page.Results)
}

func (c *Client) MovieDetail(ctx context.Context, id string) (*models.MovieDetail, error) {
	return cached(c, "movie/"+id, detailCacheTTL, func() (*models.MovieDetail, error) {
		d, err := c.fetchDetail(ctx, "movie", id, "credits,videos,images")
		if err != nil {
			return nil, err
		}
		m := toDetail(d, "movie", cmp.Or(d.ImdbID, id))
		m.BehaviorHints.DefaultVideoID = &m.ID
		return &m, nil
	})
}

func (c *Client) SeriesDetail(ctx context.Context, id string) (*models.SeriesDetail, error) {
	return cached(c, "series/"+id, detailCacheTTL, func() (*models.SeriesDetail, error) {
		d, err := c.fetchDetail(ctx, "tv", id, "external_ids,credits,videos,images")
		if err != nil {
			return nil, err
		}
		s := models.SeriesDetail{
			MovieDetail: toDetail(d, "series", cmp.Or(d.ExternalIDs.ImdbID, id)),
			Status:      d.Status,
			TvdbID:      d.ExternalIDs.TvdbID,
		}
		if d.Status == "Returning Series" {
			s.Status = "Continuing"
		}

		s.Year = yearOf(d.FirstAirDate) + "–"
		if d.Status == "Ended" || d.Status == "Canceled" {
			s.Year += yearOf(d.LastAirDate)
		}
		s.ReleaseInfo = s.Year

		runtime := first(d.EpisodeRunTime)
		if runtime == 0 && d.LastEpisodeToAir != nil {
			runtime = d.LastEpisodeToAir.Runtime
		}
		s.Runtime = minutes(runtime)

		seasons := make([]int, len(d.Seasons))
		for i, se := range d.Seasons {
			seasons[i] = se.SeasonNumber
		}
		if s.Videos, err = c.episodes(ctx, d.ID, s.ID, seasons); err != nil {
			return nil, err
		}
		today := time.Now().Format("2006-01-02")
		s.BehaviorHints.HasScheduledVideos = slices.ContainsFunc(s.Videos, func(e models.Episode) bool {
			return e.Released > today
		})
		return &s, nil
	})
}

func (c *Client) Recommendations(ctx context.Context, mediaType, id string) ([]models.DiscoverItem, error) {
	t := tmdbType(mediaType)
	return cached(c, "recommendations/"+t+"/"+id, detailCacheTTL, func() ([]models.DiscoverItem, error) {
		tmdbID, err := c.resolve(ctx, t, id)
		if err != nil {
			return nil, err
		}
		var page tmdbPage
		if err := c.get(ctx, "/"+t+"/"+strconv.Itoa(tmdbID)+"/recommendations", nil, &page); err != nil {
			return nil, err
		}
		return c.toItems(ctx, t, page.Results)
	})
}

func (c *Client) PersonDetail(ctx context.Context, id string) (*models.Person, error) {
	return cached(c, "person/"+id, detailCacheTTL, func() (*models.Person, error) {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, ErrNotFound
		}
		var p tmdbPerson
		q := url.Values{"append_to_response": {"combined_credits,external_ids"}}
		if err := c.get(ctx, "/person/"+strconv.Itoa(n), q, &p); err != nil {
			return nil, err
		}

		rows := p.CombinedCredits.Cast
		slices.SortFunc(rows, func(a, b tmdbListItem) int { return cmp.Compare(b.Popularity, a.Popularity) })
		rows = rows[:min(maxCredits, len(rows))]
		ids, err := c.resolveIDs(ctx, "", rows)
		if err != nil {
			return nil, err
		}
		credits := make([]models.PersonCredit, 0, len(rows))
		seen := map[string]bool{}
		for i, r := range rows {
			if ids[i] != "" && !seen[ids[i]] {
				seen[ids[i]] = true
				credits = append(credits, models.PersonCredit{DiscoverItem: toItem(r, "", ids[i]), Character: r.Character})
			}
		}

		return &models.Person{
			ID:           p.ID,
			ImdbID:       p.ExternalIDs.ImdbID,
			Name:         p.Name,
			Biography:    p.Biography,
			Birthday:     p.Birthday,
			Deathday:     p.Deathday,
			PlaceOfBirth: p.PlaceOfBirth,
			KnownFor:     p.KnownForDepartment,
			Photo:        image("w500", p.ProfilePath),
			Credits:      credits,
		}, nil
	})
}

func (c *Client) fetchDetail(ctx context.Context, t, id, appends string) (*tmdbDetail, error) {
	tmdbID, err := c.resolve(ctx, t, id)
	if err != nil {
		return nil, err
	}
	var d tmdbDetail
	q := url.Values{"append_to_response": {appends}, "include_image_language": {"en,null"}}
	if err := c.get(ctx, "/"+t+"/"+strconv.Itoa(tmdbID), q, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *Client) resolve(ctx context.Context, t, id string) (int, error) {
	if n, err := strconv.Atoi(id); err == nil {
		return n, nil
	}
	if !strings.HasPrefix(id, "tt") {
		return 0, ErrNotFound
	}
	var found struct {
		Movies []tmdbListItem `json:"movie_results"`
		TV     []tmdbListItem `json:"tv_results"`
	}
	if err := c.get(ctx, "/find/"+url.PathEscape(id), url.Values{"external_source": {"imdb_id"}}, &found); err != nil {
		return 0, err
	}
	hits := found.Movies
	if t == "tv" {
		hits = found.TV
	}
	if len(hits) == 0 {
		return 0, ErrNotFound
	}
	return hits[0].ID, nil
}

func (c *Client) episodes(ctx context.Context, tmdbID int, publicID string, seasons []int) ([]models.Episode, error) {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		eps  []models.Episode
		errs []error
	)
	for chunk := range slices.Chunk(seasons, seasonsPerRequest) {
		wg.Go(func() {
			keys := make([]string, len(chunk))
			for i, n := range chunk {
				keys[i] = "season/" + strconv.Itoa(n)
			}
			var raw map[string]json.RawMessage
			q := url.Values{"append_to_response": {strings.Join(keys, ",")}}
			err := c.get(ctx, "/tv/"+strconv.Itoa(tmdbID), q, &raw)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			for _, k := range keys {
				if raw[k] == nil {
					continue
				}
				var season struct {
					Episodes []tmdbEpisode `json:"episodes"`
				}
				if err := json.Unmarshal(raw[k], &season); err != nil {
					errs = append(errs, fmt.Errorf("decode %s: %w", k, err))
					return
				}
				for _, e := range season.Episodes {
					eps = append(eps, models.Episode{
						ID:         fmt.Sprintf("%s:%d:%d", publicID, e.SeasonNumber, e.EpisodeNumber),
						Title:      e.Name,
						Season:     e.SeasonNumber,
						Episode:    e.EpisodeNumber,
						Released:   e.AirDate,
						Thumbnail:  image("w300", e.StillPath),
						Overview:   e.Overview,
						ImdbRating: rating(e.VoteAverage, e.VoteCount),
					})
				}
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	slices.SortFunc(eps, func(a, b models.Episode) int {
		if (a.Season == 0) != (b.Season == 0) {
			if a.Season == 0 {
				return 1
			}
			return -1
		}
		return cmp.Or(cmp.Compare(a.Season, b.Season), cmp.Compare(a.Episode, b.Episode))
	})
	return eps, nil
}

func toDetail(d *tmdbDetail, mediaType, publicID string) models.MovieDetail {
	imdb := cmp.Or(d.ImdbID, d.ExternalIDs.ImdbID)
	release := cmp.Or(d.ReleaseDate, d.FirstAirDate)
	genres := names(d.Genres)

	var director, writer []string
	for _, cr := range d.Credits.Crew {
		switch {
		case cr.Job == "Director":
			director = append(director, cr.Name)
		case cr.Department == "Writing":
			writer = append(writer, cr.Name)
		}
	}

	cast := d.Credits.Cast[:min(maxCast, len(d.Credits.Cast))]
	castNames := make([]string, len(cast))
	credits := make([]models.Credit, len(cast))
	for i, p := range cast {
		castNames[i] = p.Name
		credits[i] = models.Credit{ID: p.ID, Name: p.Name, Character: p.Character, Photo: image("w185", p.ProfilePath)}
	}

	var trailers []models.Trailer
	var streams []models.TrailerStream
	for _, v := range d.Videos.Results {
		if v.Site != "YouTube" || v.Type != "Trailer" {
			continue
		}
		trailers = append(trailers, models.Trailer{Source: v.Key, Type: v.Type})
		streams = append(streams, models.TrailerStream{Title: v.Name, YtID: v.Key})
	}

	m := models.MovieDetail{
		ID:             publicID,
		ImdbID:         imdb,
		MoviedbID:      &d.ID,
		Type:           mediaType,
		Name:           cmp.Or(d.Title, d.Name),
		Year:           yearOf(release),
		ReleaseInfo:    yearOf(release),
		Released:       release,
		Runtime:        minutes(d.Runtime),
		Country:        first(names(d.ProductionCountries)),
		Description:    d.Overview,
		Genre:          genres,
		Genres:         genres,
		Cast:           castNames,
		Credits:        credits,
		Director:       uniq(director),
		Writer:         uniq(append(names(d.CreatedBy), writer...)),
		ImdbRating:     rating(d.VoteAverage, d.VoteCount),
		Popularity:     d.Popularity,
		Poster:         image("w500", d.PosterPath),
		Background:     image("w1280", d.BackdropPath),
		Logo:           logo(d.Images.Logos),
		Trailers:       trailers,
		TrailerStreams: streams,
	}
	if imdb != "" {
		m.Links = []models.Link{{Name: m.ImdbRating, Category: "imdb", URL: "https://imdb.com/title/" + imdb}}
	}
	return m
}

func tmdbType(mediaType string) string {
	if mediaType == "series" {
		return "tv"
	}
	return mediaType
}

func publicType(t string) string {
	if t == "tv" {
		return "series"
	}
	return t
}

func image(size, path string) string {
	if path == "" {
		return ""
	}
	return imageBase + size + path
}

func logo(logos []tmdbImage) string {
	if len(logos) == 0 {
		return ""
	}
	i := max(slices.IndexFunc(logos, func(l tmdbImage) bool { return strings.HasSuffix(l.FilePath, ".png") }), 0)
	return image("w500", logos[i].FilePath)
}

func yearOf(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

func rating(avg float64, votes int) string {
	if votes == 0 {
		return ""
	}
	return strconv.FormatFloat(avg, 'f', 1, 64)
}

func minutes(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n) + " min"
}

func names(ns []named) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Name
	}
	return out
}

func uniq(s []string) []string {
	seen := map[string]bool{}
	return slices.DeleteFunc(s, func(v string) bool {
		if seen[v] {
			return true
		}
		seen[v] = true
		return false
	})
}

func first[T any](s []T) T {
	if len(s) > 0 {
		return s[0]
	}
	var zero T
	return zero
}
