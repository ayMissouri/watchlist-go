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

type Trailer struct {
	Source string `json:"source"`
	Type   string `json:"type"`
}

type TrailerStream struct {
	Title string `json:"title"`
	YtID  string `json:"ytId"`
}

type Link struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

type BehaviorHints struct {
	DefaultVideoID     *string `json:"defaultVideoId"`
	HasScheduledVideos bool    `json:"hasScheduledVideos"`
}

// popularity scores across multiple platforms
type Popularities map[string]any

type Episode struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Season     int    `json:"season"`
	Episode    int    `json:"episode"`
	Released   string `json:"released,omitempty"`
	Thumbnail  string `json:"thumbnail,omitempty"`
	Overview   string `json:"overview,omitempty"`
	ImdbRating string `json:"imdbRating,omitempty"`
}

type MovieDetail struct {
	ID             string          `json:"id"`
	ImdbID         string          `json:"imdb_id"`
	MoviedbID      *int            `json:"moviedb_id,omitempty"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug,omitempty"`
	Year           string          `json:"year,omitempty"`
	ReleaseInfo    string          `json:"releaseInfo,omitempty"`
	Released       string          `json:"released,omitempty"`
	DvdRelease     *string         `json:"dvdRelease,omitempty"`
	Runtime        string          `json:"runtime,omitempty"`
	Country        string          `json:"country,omitempty"`
	Description    string          `json:"description,omitempty"`
	Genre          []string        `json:"genre,omitempty"`
	Genres         []string        `json:"genres,omitempty"`
	Cast           []string        `json:"cast,omitempty"`
	Director       []string        `json:"director,omitempty"`
	Writer         []string        `json:"writer,omitempty"`
	Awards         string          `json:"awards,omitempty"`
	ImdbRating     string          `json:"imdbRating,omitempty"`
	Popularity     float64         `json:"popularity,omitempty"`
	Popularities   Popularities    `json:"popularities,omitempty"`
	Poster         string          `json:"poster,omitempty"`
	Background     string          `json:"background,omitempty"`
	Logo           string          `json:"logo,omitempty"`
	Trailers       []Trailer       `json:"trailers,omitempty"`
	TrailerStreams []TrailerStream `json:"trailerStreams,omitempty"`
	Videos         []Episode       `json:"videos,omitempty"`
	Links          []Link          `json:"links,omitempty"`
	BehaviorHints  BehaviorHints   `json:"behaviorHints,omitempty"`
}

type SeriesDetail struct {
	// embeds MovieDetail, so it has all the same fields, plus the ones defined below.
	MovieDetail
	Status string `json:"status,omitempty"`
	TvdbID *int   `json:"tvdb_id,omitempty"`
}

type MetaDetailResponse struct {
	Meta any `json:"meta"`
}

type SearchResponse struct {
	Items []DiscoverItem `json:"items"`
	Query string         `json:"query"`
	Type  string         `json:"type,omitempty"` // leave empty for mixed results
}

type BulkDeleteRequest struct {
	IDs []string `json:"ids"`
}
