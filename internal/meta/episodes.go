package meta

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const maxJikanPages = 20

type jikanEpisodes struct {
	Pagination struct {
		HasNextPage bool `json:"has_next_page"`
	} `json:"pagination"`
	Data []struct {
		MalID  int    `json:"mal_id"` // the episode number
		Title  string `json:"title"`
		Aired  string `json:"aired"`
		Filler bool   `json:"filler"`
		Recap  bool   `json:"recap"`
	} `json:"data"`
}

func (c *Client) AnimeEpisodes(ctx context.Context, id string) ([]models.Episode, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, ErrNotFound
	}
	malID := strconv.Itoa(n)

	if c.jikanBaseURL != "" {
		eps, err := cached(c, "anime/"+malID+"/episodes", detailCacheTTL, func() ([]models.Episode, error) {
			return c.jikanEpisodes(ctx, malID)
		})
		switch {
		case err != nil:
			log.Printf("meta: jikan episodes for anime %s: %v", malID, err)
		case len(eps) > 0:
			return eps, nil
		}
	}
	return c.numberedEpisodes(ctx, malID)
}

func (c *Client) jikanEpisodes(ctx context.Context, malID string) ([]models.Episode, error) {
	var eps []models.Episode
	for page := 1; page <= maxJikanPages; page++ {
		var res jikanEpisodes
		q := url.Values{"page": {strconv.Itoa(page)}}
		if err := c.fetch(ctx, c.jikanBaseURL+"/anime/"+malID+"/episodes?"+q.Encode(), "jikan episodes", nil, &res); err != nil {
			return nil, err
		}
		for _, e := range res.Data {
			eps = append(eps, models.Episode{
				ID:       episodeID(malID, e.MalID),
				Title:    e.Title,
				Season:   1,
				Episode:  e.MalID,
				Released: yearMonthDay(e.Aired),
				Filler:   e.Filler,
				Recap:    e.Recap,
			})
		}
		if !res.Pagination.HasNextPage {
			break
		}
	}
	return eps, nil
}

func (c *Client) numberedEpisodes(ctx context.Context, malID string) ([]models.Episode, error) {
	a, err := c.anime(ctx, malID)
	if err != nil {
		return nil, err
	}
	eps := make([]models.Episode, a.NumEpisodes)
	for i := range eps {
		eps[i] = models.Episode{
			ID:      episodeID(malID, i+1),
			Title:   fmt.Sprintf("Episode %d", i+1),
			Season:  1,
			Episode: i + 1,
		}
	}
	return eps, nil
}

func episodeID(malID string, episode int) string {
	return fmt.Sprintf("%s:1:%d", malID, episode)
}

func yearMonthDay(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	return ts[:10]
}
