package models

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	// omitempty means if field is empty, it will be omitted from the JSON response.
	Avatar    string `json:"avatar,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Progress struct {
	Watched  float64 `json:"watched"`
	Duration float64 `json:"duration"`
}

type EpisodeProgress struct {
	Season      int      `json:"season"`
	Episode     int      `json:"episode"`
	Progress    Progress `json:"progress"`
	LastUpdated int64    `json:"last_updated"`
}

// ShowProgress is keyed by "s1e1", "s1e2" etc.
type ShowProgress map[string]EpisodeProgress

type WatchlistItem struct {
	ID           string   `json:"id"`
	TmdbID       int      `json:"tmdb_id,omitempty"`
	Type         string   `json:"type"`
	Title        string   `json:"title"`
	PosterPath   string   `json:"poster_path,omitempty"`
	BackdropPath string   `json:"backdrop_path,omitempty"`
	Progress     Progress `json:"progress"`
	// A *int can be nil.
	LastSeasonWatched  *int         `json:"last_season_watched,omitempty"`
	LastEpisodeWatched *int         `json:"last_episode_watched,omitempty"`
	ShowProgress       ShowProgress `json:"show_progress,omitempty"`
	LastUpdated        int64        `json:"last_updated"`
}

type UpdateProgressRequest struct {
	Progress           Progress     `json:"progress"`
	ShowProgress       ShowProgress `json:"show_progress,omitempty"`
	LastSeasonWatched  *int         `json:"last_season_watched,omitempty"`
	LastEpisodeWatched *int         `json:"last_episode_watched,omitempty"`
	LastUpdated        int64        `json:"last_updated"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type WatchlistResponse struct {
	Items      []WatchlistItem `json:"items"`
	Pagination PaginationMeta  `json:"pagination"`
}

type WatchlistQuery struct {
	Page    int
	PerPage int
	Type    string // "tv", "movie", or "" for all
	Sort    string // "last_updated", "title"
	Order   string // "asc", "desc"
}

type DiscoverItem struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Poster     string `json:"poster,omitempty"`
	ImdbRating string `json:"imdb_rating,omitempty"`
	Year       string `json:"year,omitempty"`
}

type DiscoverResponse struct {
	Items []DiscoverItem `json:"items"`
}

type DiscoverAllResponse struct {
	PopularMovies  []DiscoverItem `json:"popular_movies"`
	PopularShows   []DiscoverItem `json:"popular_shows"`
	TopRatedMovies []DiscoverItem `json:"top_rated_movies"`
	TopRatedShows  []DiscoverItem `json:"top_rated_shows"`
}