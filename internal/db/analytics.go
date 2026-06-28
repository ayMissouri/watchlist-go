package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const watchEventFilter = `event_type IN ('movie_watch','episode_watch')`

// RecordEvent appends a single event and returns its id.
func (d *DB) RecordEvent(ctx context.Context, ev models.UserEvent) (int64, error) {
	metaJSON := []byte("{}")
	if len(ev.Metadata) > 0 {
		b, err := json.Marshal(ev.Metadata)
		if err != nil {
			return 0, err
		}
		metaJSON = b
	}

	occurred := time.Now()
	if ev.OccurredAt > 0 {
		occurred = time.UnixMilli(ev.OccurredAt)
	}

	source := ev.Source
	if source == "" {
		source = "server"
	}

	var id int64
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO user_events (
			user_id, event_type, source, item_id, media_type, imdb_id, title,
			season, episode, runtime_minutes, genres, release_year, metadata, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id
	`,
		ev.UserID, ev.EventType, source,
		nullString(ev.ItemID), nullString(ev.MediaType), nullString(ev.ImdbID), nullString(ev.Title),
		ev.Season, ev.Episode, nullInt(ev.RuntimeMinutes),
		nullStrings(ev.Genres), nullInt(ev.ReleaseYear), metaJSON, occurred,
	).Scan(&id)
	return id, err
}

// EnrichEvent backfills genre/year/runtime on an existing event.
func (d *DB) EnrichEvent(ctx context.Context, id int64, genres []string, releaseYear, runtimeMinutes int) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE user_events SET
			genres          = COALESCE(genres, $2),
			release_year    = COALESCE(release_year, $3),
			runtime_minutes = COALESCE(runtime_minutes, $4)
		WHERE id = $1
	`, id, nullStrings(genres), nullInt(releaseYear), nullInt(runtimeMinutes))
	return err
}

func (d *DB) GetEvents(ctx context.Context, userID string, limit int, fromMs, toMs int64) ([]models.UserEvent, error) {
	if fromMs < 0 {
		fromMs = 0
	}
	if toMs <= 0 {
		toMs = math.MaxInt64
	}
	rows, err := d.Pool.Query(ctx, `
		SELECT id, event_type, source, item_id, media_type, imdb_id, title,
		       season, episode, runtime_minutes, genres, release_year, metadata,
		       (EXTRACT(EPOCH FROM occurred_at) * 1000)::BIGINT AS occurred_ms
		FROM user_events
		WHERE user_id = $1
		  AND (EXTRACT(EPOCH FROM occurred_at) * 1000)::BIGINT >= $2
		  AND (EXTRACT(EPOCH FROM occurred_at) * 1000)::BIGINT <  $3
		ORDER BY occurred_at DESC
		LIMIT $4
	`, userID, fromMs, toMs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.UserEvent{}
	for rows.Next() {
		var ev models.UserEvent
		var itemID, mediaType, imdbID, title *string
		var season, episode, runtime, releaseYear *int
		var metaJSON []byte
		if err := rows.Scan(
			&ev.ID, &ev.EventType, &ev.Source, &itemID, &mediaType, &imdbID, &title,
			&season, &episode, &runtime, &ev.Genres, &releaseYear, &metaJSON, &ev.OccurredAt,
		); err != nil {
			return nil, err
		}
		ev.ItemID = derefString(itemID)
		ev.MediaType = derefString(mediaType)
		ev.ImdbID = derefString(imdbID)
		ev.Title = derefString(title)
		ev.Season = derefInt(season)
		ev.Episode = derefInt(episode)
		ev.RuntimeMinutes = derefInt(runtime)
		ev.ReleaseYear = derefInt(releaseYear)
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &ev.Metadata)
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

type EventTotals struct {
	Total          int
	Movies         int
	Episodes       int
	ItemsAdded     int
	Searches       int
	AppOpens       int
	WatchMinutes   int
	UniqueTitles   int
	ShowsStarted   int
	ShowsCompleted int
	FirstAt        int64
	LastAt         int64
}

// GetEventTotals computes every stat for a time window in one query.
func (d *DB) GetEventTotals(ctx context.Context, userID string, start, end time.Time) (EventTotals, error) {
	var t EventTotals
	err := d.Pool.QueryRow(ctx, `
		SELECT
		  COUNT(*),
		  COUNT(*) FILTER (WHERE event_type = 'movie_watch'),
		  COUNT(*) FILTER (WHERE event_type = 'episode_watch'),
		  COUNT(*) FILTER (WHERE event_type = 'add'),
		  COUNT(*) FILTER (WHERE event_type = 'search'),
		  COUNT(*) FILTER (WHERE event_type = 'login'),
		  COALESCE(SUM(runtime_minutes) FILTER (WHERE `+watchEventFilter+`), 0),
		  COUNT(DISTINCT item_id) FILTER (WHERE `+watchEventFilter+`),
		  COUNT(DISTINCT item_id) FILTER (WHERE event_type = 'episode_watch' AND media_type = 'tv'),
		  COUNT(*) FILTER (WHERE event_type = 'status_change' AND media_type = 'tv' AND metadata->>'to' = 'watched'),
		  COALESCE((EXTRACT(EPOCH FROM MIN(occurred_at)) * 1000)::BIGINT, 0),
		  COALESCE((EXTRACT(EPOCH FROM MAX(occurred_at)) * 1000)::BIGINT, 0)
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3
	`, userID, start, end).Scan(
		&t.Total, &t.Movies, &t.Episodes, &t.ItemsAdded, &t.Searches, &t.AppOpens,
		&t.WatchMinutes, &t.UniqueTitles, &t.ShowsStarted, &t.ShowsCompleted,
		&t.FirstAt, &t.LastAt,
	)
	return t, err
}

// GetTopGenres returns the user's most-watched genres in the window.
func (d *DB) GetTopGenres(ctx context.Context, userID string, start, end time.Time, limit int) ([]models.LabelCount, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT g AS label, COUNT(*) AS cnt
		FROM user_events, UNNEST(genres) AS g
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND `+watchEventFilter+`
		GROUP BY g
		ORDER BY cnt DESC, g ASC
		LIMIT $4
	`, userID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLabelCounts(rows)
}

// GetTopTitles returns the user's most-watched titles in the window.
func (d *DB) GetTopTitles(ctx context.Context, userID string, start, end time.Time, limit int) ([]models.LabelCount, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT title AS label, COUNT(*) AS cnt
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		  AND `+watchEventFilter+` AND title IS NOT NULL
		GROUP BY title
		ORDER BY cnt DESC, label ASC
		LIMIT $4
	`, userID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLabelCounts(rows)
}

// GetDecadeBreakdown buckets watched titles by release decade.
func (d *DB) GetDecadeBreakdown(ctx context.Context, userID string, start, end time.Time, limit int) ([]models.LabelCount, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT (FLOOR(release_year / 10) * 10)::INT AS decade, COUNT(*) AS cnt
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		  AND `+watchEventFilter+` AND release_year IS NOT NULL AND release_year > 0
		GROUP BY decade
		ORDER BY cnt DESC, decade ASC
		LIMIT $4
	`, userID, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.LabelCount{}
	for rows.Next() {
		var decade, cnt int
		if err := rows.Scan(&decade, &cnt); err != nil {
			return nil, err
		}
		out = append(out, models.LabelCount{Label: fmt.Sprintf("%ds", decade), Count: cnt})
	}
	return out, rows.Err()
}

// GetWatchHistogram returns a fixed-length histogram of watch events in the window.
func (d *DB) GetWatchHistogram(ctx context.Context, userID string, start, end time.Time, part string, size int) ([]int, error) {
	q := fmt.Sprintf(`
		SELECT EXTRACT(%s FROM occurred_at)::INT AS bucket, COUNT(*) AS cnt
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND %s
		GROUP BY bucket
	`, part, watchEventFilter)

	rows, err := d.Pool.Query(ctx, q, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]int, size)
	for rows.Next() {
		var bucket, cnt int
		if err := rows.Scan(&bucket, &cnt); err != nil {
			return nil, err
		}
		if part == "month" {
			bucket-- // EXTRACT(MONTH) is 1-12; index 0-11.
		}
		if bucket >= 0 && bucket < size {
			out[bucket] = cnt
		}
	}
	return out, rows.Err()
}

// GetDistinctWatchDays returns the distinct UTC days the user watched something.
func (d *DB) GetDistinctWatchDays(ctx context.Context, userID string, start, end time.Time) ([]time.Time, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT DISTINCT date_trunc('day', occurred_at AT TIME ZONE 'UTC') AS d
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND `+watchEventFilter+`
		ORDER BY d ASC
	`, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := []time.Time{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

// GetEdgeWatch returns the first or last watch eventin the window.
func (d *DB) GetEdgeWatch(ctx context.Context, userID string, start, end time.Time, newest bool) (*models.WatchRef, error) {
	order := "ASC"
	if newest {
		order = "DESC"
	}
	var ref models.WatchRef
	var title, itemID *string
	err := d.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT title, item_id, (EXTRACT(EPOCH FROM occurred_at) * 1000)::BIGINT
		FROM user_events
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND %s
		ORDER BY occurred_at %s
		LIMIT 1
	`, watchEventFilter, order), userID, start, end).Scan(&title, &itemID, &ref.OccurredAt)
	if err != nil {
		// No rows is not an error here, the user didn't watch anything.
		return nil, nil //nolint:nilerr
	}
	ref.Title = derefString(title)
	ref.ItemID = derefString(itemID)
	return &ref, nil
}

func scanLabelCounts(rows pgx.Rows) ([]models.LabelCount, error) {
	out := []models.LabelCount{}
	for rows.Next() {
		var lc models.LabelCount
		if err := rows.Scan(&lc.Label, &lc.Count); err != nil {
			return nil, err
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

func nullInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}

func nullStrings(s []string) any {
	if len(s) == 0 {
		return nil
	}
	return s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
