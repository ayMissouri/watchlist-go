package meta

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const animeJSON = `{"id":5114,"title":"Fullmetal Alchemist: Brotherhood",
	"main_picture":{"medium":"https://cdn/m.jpg","large":"https://cdn/l.jpg"},
	"alternative_titles":{"en":"FMA: Brotherhood"},
	"start_date":"2009-04-05","mean":9.1,"num_scoring_users":2000000,
	"synopsis":"Two brothers.","media_type":"tv","status":"finished_airing",
	"genres":[{"id":1,"name":"Action"}],"studios":[{"id":4,"name":"Bones"}],
	"num_episodes":64,"average_episode_duration":1440,
	"recommendations":[{"node":{"id":11061,"title":"Hunter x Hunter","main_picture":{"medium":"https://cdn/h.jpg"}},"num_recommendations":9}]}`

func newAnimeTestClient(t *testing.T) (*Client, *atomic.Int32) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /anime/ranking", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ranking_type") != "bypopularity" {
			t.Errorf("unexpected ranking query %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[{"node":{"id":16498,"title":"Shingeki no Kyojin","start_date":"2013-04-07",
			"alternative_titles":{"en":"Attack on Titan","ja":"進撃の巨人"}},"ranking":{"rank":1}}]}`)
	})
	mux.HandleFunc("GET /anime", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "fullmetal" {
			t.Errorf("unexpected search query %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[{"node":{"id":5114,"title":"Fullmetal Alchemist: Brotherhood","mean":9.1,"num_scoring_users":10}}]}`)
	})
	mux.HandleFunc("GET /anime/5114", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, animeJSON) })

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("X-MAL-CLIENT-ID") != "m" {
			t.Errorf("%s: missing MAL client id header", r.URL.Path)
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	c := newClient("", "k", "US")
	c.httpClient = srv.Client()
	c.malBaseURL = srv.URL
	c.malClientID = "m"
	return c, &requests
}

func TestAnime(t *testing.T) {
	c, requests := newAnimeTestClient(t)

	items, err := c.AnimeCatalog(t.Context(), "bypopularity", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "16498" || items[0].Type != "anime" || items[0].Year != "2013" ||
		items[0].ImdbRating != "" || items[0].Title != "Shingeki no Kyojin" || items[0].TitleEnglish != "Attack on Titan" {
		t.Errorf("catalog = %+v", items)
	}

	before := requests.Load()
	for lang, want := range map[string]string{"english": "Attack on Titan", "native": "進撃の巨人", "bogus": "Shingeki no Kyojin"} {
		items, err := c.AnimeCatalog(t.Context(), "bypopularity", lang)
		if err != nil || items[0].Title != want {
			t.Errorf("title=%s: got %+v err=%v, want %q", lang, items, err, want)
		}
	}
	if requests.Load() != before {
		t.Error("switching title language refetched the catalog instead of using the cache")
	}

	items, err = c.SearchAnime(t.Context(), "fullmetal", "native")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "5114" || items[0].ImdbRating != "9.1" || items[0].Title != "Fullmetal Alchemist: Brotherhood" {
		t.Errorf("search = %+v", items)
	}

	before = requests.Load()
	if items, err := c.SearchAnime(t.Context(), "fm", ""); err != nil || len(items) != 0 || requests.Load() != before {
		t.Errorf("short query: items=%v err=%v, want empty without calling MAL", items, err)
	}

	d, err := c.AnimeDetail(t.Context(), "5114", "english")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "5114" || d.Type != "anime" || d.Name != "FMA: Brotherhood" || d.Episodes != 64 || d.Runtime != "24 min" || d.Year != "2009" ||
		d.Score != "9.1" || d.Poster != "https://cdn/l.jpg" || d.Studios[0] != "Bones" || d.Genres[0] != "Action" {
		t.Errorf("detail = %+v", d)
	}

	before = requests.Load()
	recs, err := c.AnimeRecommendations(t.Context(), "5114")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "11061" || recs[0].Poster != "https://cdn/h.jpg" {
		t.Errorf("recommendations = %+v", recs)
	}
	if requests.Load() != before {
		t.Error("recommendations refetched the anime instead of using the cached detail")
	}

	if _, err := c.AnimeDetail(t.Context(), "bogus", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("bogus id: err = %v, want ErrNotFound", err)
	}
	if _, err := c.AnimeDetail(t.Context(), "1", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing id: err = %v, want ErrNotFound", err)
	}
}
