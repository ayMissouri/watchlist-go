package meta

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const jikanPage1 = `{"pagination":{"has_next_page":true},"data":[
	{"mal_id":1,"title":"Fullmetal Alchemist","aired":"2009-04-05T00:00:00+00:00","filler":true,"recap":false}]}`

const jikanPage2 = `{"pagination":{"has_next_page":false},"data":[
	{"mal_id":2,"title":"The First Day","aired":"2009-04-12T00:00:00+00:00","filler":false,"recap":true}]}`

func TestAnimeEpisodes(t *testing.T) {
	c, _ := newAnimeTestClient(t)

	var fail, empty bool
	jikan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if empty {
			_, _ = io.WriteString(w, `{"pagination":{"has_next_page":false},"data":[]}`)
			return
		}
		if r.URL.Path != "/anime/5114/episodes" {
			t.Errorf("unexpected jikan path %q", r.URL.Path)
		}
		body := jikanPage1
		if r.URL.Query().Get("page") == "2" {
			body = jikanPage2
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(jikan.Close)
	c.jikanBaseURL = jikan.URL

	eps, err := c.AnimeEpisodes(t.Context(), "5114")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("got %d episodes, want 2 (both pages)", len(eps))
	}
	if eps[0] != (models.Episode{ID: "5114:1:1", Title: "Fullmetal Alchemist", Season: 1, Episode: 1, Released: "2009-04-05", Filler: true}) {
		t.Errorf("episode 1 = %+v", eps[0])
	}
	if eps[1].ID != "5114:1:2" || eps[1].Episode != 2 || !eps[1].Recap || eps[1].Filler {
		t.Errorf("episode 2 = %+v", eps[1])
	}

	fail = true
	c.mu.Lock()
	delete(c.cache, "anime/5114/episodes")
	c.mu.Unlock()

	eps, err = c.AnimeEpisodes(t.Context(), "5114")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 64 || eps[63].Title != "Episode 64" || eps[63].ID != "5114:1:64" || eps[63].Released != "" {
		t.Errorf("fallback: got %d episodes, last %+v", len(eps), eps[len(eps)-1])
	}

	fail, empty = false, true
	c.mu.Lock()
	delete(c.cache, "anime/5114/episodes")
	c.mu.Unlock()
	if eps, err := c.AnimeEpisodes(t.Context(), "5114"); err != nil || len(eps) != 64 {
		t.Errorf("empty jikan list: got %d episodes, err %v", len(eps), err)
	}

	c.jikanBaseURL = ""
	if eps, err := c.AnimeEpisodes(t.Context(), "5114"); err != nil || len(eps) != 64 {
		t.Errorf("unconfigured: got %d episodes, err %v", len(eps), err)
	}
	if _, err := c.AnimeEpisodes(t.Context(), "bogus"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bogus id: err = %v, want ErrNotFound", err)
	}
}
