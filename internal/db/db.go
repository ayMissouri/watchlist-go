package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

// Every function that does I/O accepts a ctx as its first argument.
// It lets us propagate deadlines and cancellations through a chain of calls,
// for example, if a request times out, the context gets cancelled and your DB query stops automatically.

// the * means New returns a pointer to a DB struct.
// pointers in Go mean youre sharing the same piece of memory rather than copying it.
func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		// %w wraps the original error.
		// It means we can add context to an error while still preserving the original error underneath.
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	if d.Pool == nil {
		return fmt.Errorf("no database pool")
	}
	return d.Pool.Ping(ctx)
}

func (d *DB) UpsertUser(ctx context.Context, u *models.User) error {
	// $1, $2, $3 are placeholders for query params.
	// ON CONFLICT makes it so if a user logs in again, their username and avatar get updated, rather than throwing a duplicate key error.
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO users (id, username, avatar)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		  SET username   = EXCLUDED.username,
		      avatar     = EXCLUDED.avatar,
		      updated_at = NOW()
	`, u.ID, u.Username, u.Avatar)
	return err
}

func (d *DB) GetUser(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	// QueryRow returns a single row and scan reads the column values into Go variables in order.
	err := d.Pool.QueryRow(ctx,
		`SELECT id, username, avatar FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Avatar)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetWatchlist returns all items for a user by item ID,
func (d *DB) GetWatchlist(ctx context.Context, userID string, q models.WatchlistQuery) ([]models.WatchlistItem, int, error) {
	args := []any{userID}
	where := "WHERE user_id = $1"

	if q.Type != "" {
		args = append(args, q.Type)
		where += fmt.Sprintf(" AND media_type = $%d", len(args))
	}

	if q.Status != "" {
		args = append(args, q.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	// gets total count of items.
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM watchlist_items %s`, where)
	if err := d.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count query: %w", err)
	}

	offset := (q.Page - 1) * q.PerPage
	args = append(args, q.PerPage, offset)

	dataQuery := fmt.Sprintf(`
		SELECT id, media_type, tmdb_id, imdb_id, title, poster_path, backdrop_path, status,
		       progress_watched, progress_duration,
		       last_season_watched, last_episode_watched,
		       episodes_watched, episodes_total,
		       show_progress, last_updated
		FROM watchlist_items
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, where, q.Sort, q.Order, len(args)-1, len(args))

	rows, err := d.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("data query: %w", err)
	}
	defer rows.Close()

	var items []models.WatchlistItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// return empty array rather than nil for no results.
	if items == nil {
		items = []models.WatchlistItem{}
	}

	return items, total, nil
}

func (d *DB) GetItem(ctx context.Context, userID, itemID string) (*models.WatchlistItem, error) {
	row := d.Pool.QueryRow(ctx, `
		SELECT id, media_type, tmdb_id, imdb_id, title, poster_path, backdrop_path, status,
		       progress_watched, progress_duration,
		       last_season_watched, last_episode_watched,
		       episodes_watched, episodes_total,
		       show_progress, last_updated
		FROM watchlist_items
		WHERE user_id = $1 AND id = $2
	`, userID, itemID)

	return scanItem(row)
}

// UpsertItem inserts or updates a watchlist item.
func (d *DB) UpsertItem(ctx context.Context, userID string, item *models.WatchlistItem) (bool, error) {
	showProgressJSON, err := json.Marshal(item.ShowProgress)
	if err != nil {
		return false, err
	}

	var inserted bool
	err = d.Pool.QueryRow(ctx, `
		INSERT INTO watchlist_items (
			id, user_id, media_type, tmdb_id, imdb_id, title,
			poster_path, backdrop_path, status,
			progress_watched, progress_duration,
			last_season_watched, last_episode_watched,
			episodes_watched, episodes_total,
			show_progress, last_updated
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id, user_id) DO UPDATE SET
			media_type           = EXCLUDED.media_type,
			tmdb_id              = EXCLUDED.tmdb_id,
			imdb_id              = EXCLUDED.imdb_id,
			title                = EXCLUDED.title,
			poster_path          = EXCLUDED.poster_path,
			backdrop_path        = EXCLUDED.backdrop_path,
			status               = EXCLUDED.status,
			progress_watched     = EXCLUDED.progress_watched,
			progress_duration    = EXCLUDED.progress_duration,
			last_season_watched  = EXCLUDED.last_season_watched,
			last_episode_watched = EXCLUDED.last_episode_watched,
			episodes_watched     = EXCLUDED.episodes_watched,
			episodes_total       = EXCLUDED.episodes_total,
			show_progress        = EXCLUDED.show_progress,
			last_updated         = EXCLUDED.last_updated
		RETURNING (xmax = 0)
	`,
		item.ID, userID, item.Type, item.TmdbID, item.ImdbID, item.Title,
		item.PosterPath, item.BackdropPath, item.Status,
		item.Progress.Watched, item.Progress.Duration,
		item.LastSeasonWatched, item.LastEpisodeWatched,
		item.EpisodesWatched, item.EpisodesTotal,
		showProgressJSON, item.LastUpdated,
	).Scan(&inserted)
	return inserted, err
}

func (d *DB) DeleteItem(ctx context.Context, userID, itemID string) error {
	tag, err := d.Pool.Exec(ctx,
		`DELETE FROM watchlist_items WHERE user_id = $1 AND id = $2`,
		userID, itemID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// this is an interface with a single method.
// both pgx.Row and pgx.Rows implement this interface, so scanItem can be used for both single-row and multi-row queries.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (*models.WatchlistItem, error) {
	item := &models.WatchlistItem{}
	var showProgressJSON []byte

	var imdbID *string
	err := row.Scan(
		&item.ID, &item.Type, &item.TmdbID, &imdbID, &item.Title,
		&item.PosterPath, &item.BackdropPath, &item.Status,
		&item.Progress.Watched, &item.Progress.Duration,
		&item.LastSeasonWatched, &item.LastEpisodeWatched,
		&item.EpisodesWatched, &item.EpisodesTotal,
		&showProgressJSON, &item.LastUpdated,
	)
	if err != nil {
		return nil, err
	}

	if imdbID != nil {
		item.ImdbID = *imdbID
	}

	if len(showProgressJSON) > 0 && string(showProgressJSON) != "null" {
		if err := json.Unmarshal(showProgressJSON, &item.ShowProgress); err != nil {
			return nil, err
		}
	}

	return item, nil
}

func (d *DB) BulkDeleteItems(ctx context.Context, userID string, ids []string) (int64, error) {
	tag, err := d.Pool.Exec(ctx,
		`DELETE FROM watchlist_items WHERE user_id = $1 and id = ANY($2)`,
		userID, ids,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

const notificationsLimit = 50

func (d *DB) CreateNotification(ctx context.Context, userID string, n *models.Notification) error {
	if n.Type == "" {
		n.Type = "general"
	}
	if n.CreatedAt == 0 {
		n.CreatedAt = time.Now().UnixMilli()
	}
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, body, poster_path, link, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userID, n.Type, n.Title, n.Body, n.PosterPath, n.Link, n.CreatedAt)
	return err
}

func (d *DB) GetNotifications(ctx context.Context, userID string) ([]models.Notification, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, type, title, body, poster_path, link, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, notificationsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		var poster, link *string
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &poster, &link, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		if poster != nil {
			n.PosterPath = *poster
		}
		if link != nil {
			n.Link = *link
		}
		items = append(items, n)
	}
	return items, rows.Err()
}

func (d *DB) CountUnreadNotifications(ctx context.Context, userID string) (int, error) {
	var count int
	err := d.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`,
		userID,
	).Scan(&count)
	return count, err
}

func (d *DB) MarkNotificationRead(ctx context.Context, userID string, id int64) error {
	tag, err := d.Pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND id = $2`,
		userID, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (d *DB) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`,
		userID,
	)
	return err
}

func (d *DB) ListUserIDs(ctx context.Context) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *DB) GetWatchlistForSync(ctx context.Context, userID string) ([]models.WatchlistItem, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, media_type, imdb_id, title, poster_path, status
		FROM watchlist_items
		WHERE user_id = $1 AND status <> 'dropped'
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.WatchlistItem{}
	for rows.Next() {
		var item models.WatchlistItem
		var imdbID, poster *string
		if err := rows.Scan(&item.ID, &item.Type, &imdbID, &item.Title, &poster, &item.Status); err != nil {
			return nil, err
		}
		if imdbID != nil {
			item.ImdbID = *imdbID
		}
		if poster != nil {
			item.PosterPath = *poster
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DB) UpsertCalendarEntry(ctx context.Context, e *models.CalendarEntry, release time.Time) error {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO calendar_entries (
			user_id, item_id, media_type, imdb_id, title, poster_path,
			season, episode, episode_title, release_date
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (user_id, item_id, season, episode) DO UPDATE SET
			media_type    = EXCLUDED.media_type,
			imdb_id       = EXCLUDED.imdb_id,
			title         = EXCLUDED.title,
			poster_path   = EXCLUDED.poster_path,
			episode_title = EXCLUDED.episode_title,
			release_date  = EXCLUDED.release_date
	`,
		e.UserID, e.ItemID, e.MediaType, nullString(e.ImdbID), e.Title, nullString(e.PosterPath),
		e.Season, e.Episode, nullString(e.EpisodeTitle), release,
	)
	return err
}

func (d *DB) DeleteStaleCalendarEntries(ctx context.Context, userID string) error {
	_, err := d.Pool.Exec(ctx, `
		DELETE FROM calendar_entries c
		WHERE c.user_id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM watchlist_items w
			WHERE w.user_id = c.user_id
			  AND w.id = c.item_id
			  AND w.status <> 'dropped'
		  )
	`, userID)
	return err
}

func (d *DB) GetCalendar(ctx context.Context, userID string) ([]models.CalendarEntry, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, item_id, media_type, imdb_id, title, poster_path,
		       season, episode, episode_title,
		       EXTRACT(EPOCH FROM release_date)::BIGINT * 1000 AS release_ms,
		       (release_date <= NOW()) AS released
		FROM calendar_entries
		WHERE user_id = $1
		ORDER BY release_date ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCalendarRows(rows)
}

func (d *DB) GetDueCalendarEntries(ctx context.Context) ([]models.CalendarEntry, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, user_id, item_id, media_type, imdb_id, title, poster_path,
		       season, episode, episode_title,
		       EXTRACT(EPOCH FROM release_date)::BIGINT * 1000 AS release_ms
		FROM calendar_entries
		WHERE release_date <= NOW()
		ORDER BY release_date ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []models.CalendarEntry{}
	for rows.Next() {
		var e models.CalendarEntry
		var imdbID, poster, episodeTitle *string
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.ItemID, &e.MediaType, &imdbID, &e.Title, &poster,
			&e.Season, &e.Episode, &episodeTitle, &e.ReleaseDate,
		); err != nil {
			return nil, err
		}
		e.ImdbID = derefString(imdbID)
		e.PosterPath = derefString(poster)
		e.EpisodeTitle = derefString(episodeTitle)
		e.Released = true
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) DeleteCalendarEntry(ctx context.Context, id int64) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM calendar_entries WHERE id = $1`, id)
	return err
}

func scanCalendarRows(rows pgx.Rows) ([]models.CalendarEntry, error) {
	entries := []models.CalendarEntry{}
	for rows.Next() {
		var e models.CalendarEntry
		var imdbID, poster, episodeTitle *string
		if err := rows.Scan(
			&e.ID, &e.ItemID, &e.MediaType, &imdbID, &e.Title, &poster,
			&e.Season, &e.Episode, &episodeTitle, &e.ReleaseDate, &e.Released,
		); err != nil {
			return nil, err
		}
		e.ImdbID = derefString(imdbID)
		e.PosterPath = derefString(poster)
		e.EpisodeTitle = derefString(episodeTitle)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
