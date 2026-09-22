package meta

import (
	"cmp"
	"context"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	animeLimit       = 40
	malListFields    = "alternative_titles,start_date,mean,num_scoring_users"
	malDetailFields  = malListFields + ",end_date,synopsis,media_type,status,genres,studios,num_episodes,average_episode_duration,recommendations{alternative_titles}"
	malMinQueryChars = 3
)

type malNode struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	AlternativeTitles struct {
		En string `json:"en"`
		Ja string `json:"ja"`
	} `json:"alternative_titles"`
	MainPicture struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"main_picture"`
	StartDate       string  `json:"start_date"`
	Mean            float64 `json:"mean"`
	NumScoringUsers int     `json:"num_scoring_users"`
}

type malList struct {
	Data []struct {
		Node malNode `json:"node"`
	} `json:"data"`
}

type malAnime struct {
	malNode
	Synopsis               string  `json:"synopsis"`
	MediaType              string  `json:"media_type"`
	Status                 string  `json:"status"`
	Genres                 []named `json:"genres"`
	Studios                []named `json:"studios"`
	NumEpisodes            int     `json:"num_episodes"`
	AverageEpisodeDuration int     `json:"average_episode_duration"`
	Recommendations        []struct {
		Node malNode `json:"node"`
	} `json:"recommendations"`
}

func (c *Client) AnimeCatalog(ctx context.Context, ranking, lang string) ([]models.DiscoverItem, error) {
	q := url.Values{"ranking_type": {ranking}, "limit": {strconv.Itoa(animeLimit)}, "fields": {malListFields}}
	nodes, err := cached(c, "anime/ranking?"+q.Encode(), cacheTTL, func() ([]malNode, error) {
		return c.animeList(ctx, "/anime/ranking", q)
	})
	if err != nil {
		return nil, err
	}
	return toAnimeItems(nodes, lang), nil
}

func (c *Client) SearchAnime(ctx context.Context, query, lang string) ([]models.DiscoverItem, error) {
	if utf8.RuneCountInString(query) < malMinQueryChars {
		return []models.DiscoverItem{}, nil
	}
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(animeLimit)}, "fields": {malListFields}}
	nodes, err := c.animeList(ctx, "/anime", q)
	if err != nil {
		return nil, err
	}
	return toAnimeItems(nodes, lang), nil
}

func (c *Client) AnimeDetail(ctx context.Context, id, lang string) (*models.AnimeDetail, error) {
	a, err := c.anime(ctx, id)
	if err != nil {
		return nil, err
	}
	malID := strconv.Itoa(a.ID)
	return &models.AnimeDetail{
		ID:          malID,
		Type:        "anime",
		Name:        a.title(lang),
		EnglishName: a.AlternativeTitles.En,
		Format:      a.MediaType,
		Status:      a.Status,
		Year:        yearOf(a.StartDate),
		Released:    a.StartDate,
		Episodes:    a.NumEpisodes,
		Runtime:     minutes(a.AverageEpisodeDuration / 60),
		Description: a.Synopsis,
		Genres:      names(a.Genres),
		Studios:     names(a.Studios),
		Score:       rating(a.Mean, a.NumScoringUsers),
		Poster:      poster(a.malNode),
		Links:       []models.Link{{Name: "MyAnimeList", Category: "mal", URL: "https://myanimelist.net/anime/" + malID}},
	}, nil
}

func (c *Client) AnimeRecommendations(ctx context.Context, id string) ([]models.DiscoverItem, error) {
	a, err := c.anime(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]models.DiscoverItem, len(a.Recommendations))
	for i, r := range a.Recommendations {
		items[i] = toAnimeItem(r.Node, "")
	}
	return items, nil
}

func (c *Client) anime(ctx context.Context, id string) (*malAnime, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, ErrNotFound
	}
	path := "/anime/" + strconv.Itoa(n)
	return cached(c, path, detailCacheTTL, func() (*malAnime, error) {
		var a malAnime
		if err := c.malGet(ctx, path, url.Values{"fields": {malDetailFields}}, &a); err != nil {
			return nil, err
		}
		return &a, nil
	})
}

func (c *Client) animeList(ctx context.Context, path string, q url.Values) ([]malNode, error) {
	var list malList
	if err := c.malGet(ctx, path, q, &list); err != nil {
		return nil, err
	}
	nodes := make([]malNode, len(list.Data))
	for i, d := range list.Data {
		nodes[i] = d.Node
	}
	return nodes, nil
}

func toAnimeItems(nodes []malNode, lang string) []models.DiscoverItem {
	items := make([]models.DiscoverItem, len(nodes))
	for i, n := range nodes {
		items[i] = toAnimeItem(n, lang)
	}
	return items
}

func toAnimeItem(n malNode, lang string) models.DiscoverItem {
	return models.DiscoverItem{
		ID:           strconv.Itoa(n.ID),
		Type:         "anime",
		Title:        n.title(lang),
		TitleEnglish: n.AlternativeTitles.En,
		Poster:       poster(n),
		ImdbRating:   rating(n.Mean, n.NumScoringUsers),
		Year:         yearOf(n.StartDate),
	}
}

func (n malNode) title(lang string) string {
	switch lang {
	case "english":
		return cmp.Or(n.AlternativeTitles.En, n.Title)
	case "native":
		return cmp.Or(n.AlternativeTitles.Ja, n.Title)
	}
	return n.Title
}

func poster(n malNode) string {
	return cmp.Or(n.MainPicture.Large, n.MainPicture.Medium)
}
