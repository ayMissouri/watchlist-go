package meta

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const movieJSON = `{"id":1,"imdb_id":"tt1","title":"The Shawshank Redemption","overview":"Two imprisoned men.",
	"poster_path":"/p.jpg","backdrop_path":"/b.jpg","release_date":"1994-09-23","runtime":142,
	"vote_average":8.7,"vote_count":26000,"genres":[{"id":18,"name":"Drama"}],
	"production_countries":[{"name":"United States of America"}],
	"credits":{"cast":[{"id":2524,"name":"Tim Robbins","character":"Andy Dufresne","profile_path":"/tr.jpg"},
			{"id":192,"name":"Morgan Freeman","character":"Red"}],
		"crew":[{"name":"Frank Darabont","job":"Director","department":"Directing"},
			{"name":"Frank Darabont","job":"Screenplay","department":"Writing"},
			{"name":"Stephen King","job":"Novel","department":"Writing"}]},
	"videos":{"results":[{"name":"Official Trailer","key":"abc","site":"YouTube","type":"Trailer"},
		{"name":"Clip","key":"zzz","site":"YouTube","type":"Clip"}]},
	"images":{"logos":[{"file_path":"/l.svg"},{"file_path":"/l.png"}]}}`

const seriesJSON = `{"id":5,"name":"Breaking Bad","first_air_date":"2008-01-20","last_air_date":"2013-09-29",
	"status":"Ended","episode_run_time":[],"last_episode_to_air":{"runtime":49},
	"seasons":[{"season_number":0},{"season_number":1}],"created_by":[{"name":"Vince Gilligan"}],
	"external_ids":{"imdb_id":"tt5","tvdb_id":81189},
	"credits":{"cast":[{"name":"Bryan Cranston"}],"crew":[{"name":"Vince Gilligan","job":"Writer","department":"Writing"}]},
	"videos":{"results":[]},"images":{"logos":[]}}`

const seasonsJSON = `{"id":5,
	"season/0":{"episodes":[{"name":"Special","season_number":0,"episode_number":1,"air_date":"2009-01-01"}]},
	"season/1":{"episodes":[
		{"name":"Cat's in the Bag...","season_number":1,"episode_number":2,"air_date":"2008-01-27","still_path":"/s2.jpg"},
		{"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2008-01-20","still_path":"/s1.jpg","vote_average":8.2,"vote_count":50}]}}`

const personJSON = `{"id":2524,"name":"Tim Robbins","biography":"Actor.","birthday":"1958-10-16",
	"place_of_birth":"West Covina, California, USA","known_for_department":"Acting","profile_path":"/tr.jpg",
	"external_ids":{"imdb_id":"nm0000209"},
	"combined_credits":{"cast":[
		{"id":2,"media_type":"movie","title":"No IMDb id","character":"X","popularity":50},
		{"id":5,"media_type":"tv","name":"Breaking Bad","character":"Cameo","popularity":10,"first_air_date":"2008-01-20"},
		{"id":1,"media_type":"movie","title":"The Shawshank Redemption","character":"Andy Dufresne","popularity":80,"release_date":"1994-09-23"},
		{"id":1,"media_type":"movie","title":"The Shawshank Redemption","character":"Archive footage","popularity":80}]}}`

var tvQueries sync.Map

func newTestClient(t *testing.T) (*Client, *atomic.Int32) {
	t.Helper()
	reply := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }
	}

	var searches atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /discover/{type}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.PathValue("type") == "tv" {
			tvQueries.Store(q.Get("with_genres"), q.Get("without_genres"))
			reply(`{"results":[]}`)(w, r)
			return
		}
		if q.Get("sort_by") != "vote_average.desc" || q.Get("with_genres") != "28" || q.Get("api_key") != "k" {
			t.Errorf("unexpected discover query %q", r.URL.RawQuery)
		}
		if q.Get("page") != "1" {
			reply(`{"results":[{"id":1,"title":"The Shawshank Redemption"}]}`)(w, r)
			return
		}
		reply(`{"results":[
			{"id":1,"title":"The Shawshank Redemption","poster_path":"/p.jpg","vote_average":9.3,"vote_count":100,"release_date":"1994-09-23"},
			{"id":2,"title":"No IMDb id"}]}`)(w, r)
	})
	mux.HandleFunc("GET /search/{type}", func(w http.ResponseWriter, r *http.Request) {
		if searches.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		reply(`{"results":[{"id":5,"name":"Breaking Bad","first_air_date":"2008-01-20","vote_average":8.9,"vote_count":10}]}`)(w, r)
	})
	mux.HandleFunc("GET /movie/{id}/external_ids", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			reply(`{"imdb_id":"tt1"}`)(w, r)
			return
		}
		reply(`{"imdb_id":null}`)(w, r)
	})
	mux.HandleFunc("GET /tv/{id}/external_ids", reply(`{"imdb_id":"tt5"}`))
	mux.HandleFunc("GET /find/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "tt1" {
			reply(`{"movie_results":[{"id":1}],"tv_results":[]}`)(w, r)
			return
		}
		reply(`{"movie_results":[],"tv_results":[{"id":5}]}`)(w, r)
	})
	mux.HandleFunc("GET /movie/{id}/recommendations", reply(`{"results":[
		{"id":2,"title":"No IMDb id"},{"id":1,"title":"The Shawshank Redemption","release_date":"1994-09-23"}]}`))
	mux.HandleFunc("GET /movie/{id}", reply(movieJSON))
	mux.HandleFunc("GET /person/{id}", reply(personJSON))
	mux.HandleFunc("GET /tv/{id}", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("append_to_response"), "season/") {
			reply(seasonsJSON)(w, r)
			return
		}
		reply(seriesJSON)(w, r)
	})

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	c := newClient(srv.URL, "k", "US")
	c.httpClient = srv.Client()
	return c, &requests
}

func TestCatalog(t *testing.T) {
	c, requests := newTestClient(t)

	items, err := c.Catalog(t.Context(), "movie", "top_rated", 28)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (no-IMDb row dropped, page-2 repeat deduped)", len(items))
	}
	got := items[0]
	if got.ID != "tt1" || got.Type != "movie" || got.Year != "1994" || got.ImdbRating != "9.3" {
		t.Errorf("unexpected item %+v", got)
	}
	if got.Poster != imageBase+"w500/p.jpg" {
		t.Errorf("poster = %q", got.Poster)
	}

	before := requests.Load()
	if _, err := c.Catalog(t.Context(), "movie", "top_rated", 28); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != before {
		t.Error("second call was not served from cache")
	}
}

func TestSeriesCatalogsSkipTalkShows(t *testing.T) {
	c, _ := newTestClient(t)

	for _, genre := range []int{0, 18, 10767} {
		if _, err := c.Catalog(t.Context(), "series", "popular", genre); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.CatalogByYear(t.Context(), "series", "2025"); err != nil {
		t.Fatal(err)
	}

	noise := "10763,10764,10766,10767"
	for with, want := range map[string]string{"": noise, "18": noise, "10767": ""} {
		got, _ := tvQueries.Load(with)
		if got != want {
			t.Errorf("with_genres=%q: without_genres=%q, want %q", with, got, want)
		}
	}
}

func TestSearchRetriesRateLimit(t *testing.T) {
	retryDelay = 0
	c, _ := newTestClient(t)

	items, err := c.SearchSeries(t.Context(), "breaking")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "tt5" || items[0].Type != "series" || items[0].Year != "2008" {
		t.Errorf("unexpected items %+v", items)
	}
}

func TestSeriesDetail(t *testing.T) {
	c, _ := newTestClient(t)

	s, err := c.SeriesDetail(t.Context(), "tt5")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "tt5" || *s.MoviedbID != 5 || *s.TvdbID != 81189 || s.Type != "series" {
		t.Errorf("ids: %+v", s.MovieDetail)
	}
	if s.Year != "2008–2013" || s.Status != "Ended" || s.Runtime != "49 min" {
		t.Errorf("year=%q status=%q runtime=%q", s.Year, s.Status, s.Runtime)
	}
	if len(s.Writer) != 1 || s.Writer[0] != "Vince Gilligan" {
		t.Errorf("writer = %v, want creator listed once", s.Writer)
	}

	var order []string
	for _, e := range s.Videos {
		order = append(order, e.ID)
	}
	if strings.Join(order, " ") != "tt5:1:1 tt5:1:2 tt5:0:1" {
		t.Errorf("episode order = %v", order)
	}
	pilot := s.Videos[0]
	if pilot.Title != "Pilot" || pilot.Released != "2008-01-20" || pilot.Thumbnail != imageBase+"w300/s1.jpg" || pilot.ImdbRating != "8.2" {
		t.Errorf("pilot = %+v", pilot)
	}
	if s.BehaviorHints.HasScheduledVideos {
		t.Error("no future episodes, HasScheduledVideos should be false")
	}
}

func TestMovieDetail(t *testing.T) {
	c, _ := newTestClient(t)

	m, err := c.MovieDetail(t.Context(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "tt1" || m.ImdbID != "tt1" || *m.BehaviorHints.DefaultVideoID != "tt1" {
		t.Errorf("ids: %+v", m)
	}
	if m.Runtime != "142 min" || m.Year != "1994" || m.Country != "United States of America" {
		t.Errorf("runtime=%q year=%q country=%q", m.Runtime, m.Year, m.Country)
	}
	if strings.Join(m.Director, ",") != "Frank Darabont" || strings.Join(m.Writer, ",") != "Frank Darabont,Stephen King" {
		t.Errorf("director=%v writer=%v", m.Director, m.Writer)
	}
	if strings.Join(m.Cast, ",") != "Tim Robbins,Morgan Freeman" || len(m.Credits) != 2 {
		t.Errorf("cast=%v credits=%v", m.Cast, m.Credits)
	}
	if c0 := m.Credits[0]; c0.ID != 2524 || c0.Character != "Andy Dufresne" || c0.Photo != imageBase+"w185/tr.jpg" {
		t.Errorf("credit = %+v", c0)
	}
	if len(m.Trailers) != 1 || m.Trailers[0].Source != "abc" || m.TrailerStreams[0].YtID != "abc" {
		t.Errorf("trailers=%v streams=%v", m.Trailers, m.TrailerStreams)
	}
	if m.Logo != imageBase+"w500/l.png" {
		t.Errorf("logo = %q, want the PNG", m.Logo)
	}
	if len(m.Links) != 1 || m.Links[0].URL != "https://imdb.com/title/tt1" {
		t.Errorf("links = %v", m.Links)
	}

	if _, err := c.MovieDetail(t.Context(), "bogus"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bogus id: err = %v, want ErrNotFound", err)
	}
}

func TestRecommendations(t *testing.T) {
	c, _ := newTestClient(t)

	items, err := c.Recommendations(t.Context(), "movie", "tt1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "tt1" || items[0].Type != "movie" {
		t.Errorf("recommendations = %+v, want just tt1", items)
	}
	if _, err := c.Recommendations(t.Context(), "movie", "bogus"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bogus id: err = %v, want ErrNotFound", err)
	}
}

func TestPersonDetail(t *testing.T) {
	c, _ := newTestClient(t)

	p, err := c.PersonDetail(t.Context(), "2524")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 2524 || p.Name != "Tim Robbins" || p.ImdbID != "nm0000209" || p.KnownFor != "Acting" || p.Photo != imageBase+"w500/tr.jpg" {
		t.Errorf("person = %+v", p)
	}

	var got []string
	for _, cr := range p.Credits {
		got = append(got, cr.ID+"/"+cr.Type+"/"+cr.Character)
	}
	if want := "tt1/movie/Andy Dufresne tt5/series/Cameo"; strings.Join(got, " ") != want {
		t.Errorf("credits = %v, want %q", got, want)
	}

	if _, err := c.PersonDetail(t.Context(), "nm0000209"); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-numeric id: err = %v, want ErrNotFound", err)
	}
}
