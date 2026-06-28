package models

// Event types for user activity tracking.
const (
	EventAdd          = "add"
	EventRemove       = "remove"
	EventBulkRemove   = "bulk_remove"
	EventStatusChange = "status_change"
	EventMovieWatch   = "movie_watch"
	EventEpisodeWatch = "episode_watch"
	EventLogin        = "login"
	EventSearch       = "search"
	EventView         = "view"
)

// UserEvent is a single tracked user action.
type UserEvent struct {
	ID             int64          `json:"id"`
	UserID         string         `json:"user_id,omitempty"`
	EventType      string         `json:"event_type"`
	Source         string         `json:"source,omitempty"`
	ItemID         string         `json:"item_id,omitempty"`
	MediaType      string         `json:"media_type,omitempty"`
	ImdbID         string         `json:"imdb_id,omitempty"`
	Title          string         `json:"title,omitempty"`
	Season         int            `json:"season,omitempty"`
	Episode        int            `json:"episode,omitempty"`
	RuntimeMinutes int            `json:"runtime_minutes,omitempty"`
	Genres         []string       `json:"genres,omitempty"`
	ReleaseYear    int            `json:"release_year,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	OccurredAt     int64          `json:"occurred_at"`
}

// ClientEvent is a single event in an ingestion batch. It is the same as a UserEvent but
// without server-owned fields (id, user_id, source).
type ClientEvent struct {
	EventType      string         `json:"event_type"`
	ItemID         string         `json:"item_id,omitempty"`
	MediaType      string         `json:"media_type,omitempty"`
	ImdbID         string         `json:"imdb_id,omitempty"`
	Title          string         `json:"title,omitempty"`
	Season         int            `json:"season,omitempty"`
	Episode        int            `json:"episode,omitempty"`
	RuntimeMinutes int            `json:"runtime_minutes,omitempty"`
	Genres         []string       `json:"genres,omitempty"`
	ReleaseYear    int            `json:"release_year,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	OccurredAt int64 `json:"occurred_at,omitempty"`
}

// RecordEventsRequest is the POST /events body.
type RecordEventsRequest struct {
	Events []ClientEvent `json:"events"`
}

// EventsResponse is the recent-activity feed for GET /events.
type EventsResponse struct {
	Items []UserEvent `json:"items"`
}

// LabelCount is a (label, count) pair used across the stats.
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// WatchRef points at a single watch event.
type WatchRef struct {
	Title      string `json:"title"`
	ItemID     string `json:"item_id,omitempty"`
	OccurredAt int64  `json:"occurred_at"`
}

// ProfileStats is the all-time summary shown on a user's profile page.
type ProfileStats struct {
	TotalEvents      int          `json:"total_events"`
	MoviesWatched    int          `json:"movies_watched"`
	EpisodesWatched  int          `json:"episodes_watched"`
	ShowsCompleted   int          `json:"shows_completed"`
	ItemsAdded       int          `json:"items_added"`
	WatchTimeMinutes int          `json:"watch_time_minutes"`
	CurrentStreakDays int         `json:"current_streak_days"`
	LongestStreakDays int         `json:"longest_streak_days"`
	TopGenres        []LabelCount `json:"top_genres"`
	TopTitles        []LabelCount `json:"top_titles"`
	FirstEventAt     int64        `json:"first_event_at,omitempty"`
	LastEventAt      int64        `json:"last_event_at,omitempty"`
}

// WrappedStats is the year-in-review payload for GET /stats/wrapped.
type WrappedStats struct {
	Year             int          `json:"year"`
	WatchTimeMinutes int          `json:"watch_time_minutes"`
	MoviesWatched    int          `json:"movies_watched"`
	EpisodesWatched  int          `json:"episodes_watched"`
	ShowsStarted     int          `json:"shows_started"`
	ShowsCompleted   int          `json:"shows_completed"`
	ItemsAdded       int          `json:"items_added"`
	UniqueTitles     int          `json:"unique_titles"`
	Searches         int          `json:"searches"`
	AppOpens         int          `json:"app_opens"`
	TopGenres        []LabelCount `json:"top_genres"`
	TopTitles        []LabelCount `json:"top_titles"`
	Decades          []LabelCount `json:"decades"`
	WatchesByMonth    []int     `json:"watches_by_month"`
	WatchesByHour     []int     `json:"watches_by_hour"`
	WatchesByWeekday  []int     `json:"watches_by_weekday"`
	BusiestMonth      string    `json:"busiest_month,omitempty"`
	PeakHour          int       `json:"peak_hour"`
	NightOwl          bool      `json:"night_owl"`
	LongestStreakDays int       `json:"longest_streak_days"`
	FirstWatch        *WatchRef `json:"first_watch,omitempty"`
	LastWatch         *WatchRef `json:"last_watch,omitempty"`
}
