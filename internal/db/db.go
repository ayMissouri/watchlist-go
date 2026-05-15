package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
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

	// gets total count of items.
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM watchlist_items %s`, where)
	if err := d.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count query: %w", err)
	}

	offset := (q.Page - 1) * q.PerPage
	args = append(args, q.PerPage, offset)

	dataQuery := fmt.Sprintf(`
		SELECT id, media_type, tmdb_id, title, poster_path, backdrop_path,
		       progress_watched, progress_duration,
		       last_season_watched, last_episode_watched,
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
		SELECT id, media_type, tmdb_id, title, poster_path, backdrop_path,
		       progress_watched, progress_duration,
		       last_season_watched, last_episode_watched,
		       show_progress, last_updated
		FROM watchlist_items
		WHERE user_id = $1 AND id = $2
	`, userID, itemID)

	return scanItem(row)
}

func (d *DB) UpsertItem(ctx context.Context, userID string, item *models.WatchlistItem) error {
	showProgressJSON, err := json.Marshal(item.ShowProgress)
	if err != nil {
		return err
	}

	_, err = d.Pool.Exec(ctx, `
		INSERT INTO watchlist_items (
			id, user_id, media_type, tmdb_id, title,
			poster_path, backdrop_path,
			progress_watched, progress_duration,
			last_season_watched, last_episode_watched,
			show_progress, last_updated
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id, user_id) DO UPDATE SET
			media_type           = EXCLUDED.media_type,
			tmdb_id              = EXCLUDED.tmdb_id,
			title                = EXCLUDED.title,
			poster_path          = EXCLUDED.poster_path,
			backdrop_path        = EXCLUDED.backdrop_path,
			progress_watched     = EXCLUDED.progress_watched,
			progress_duration    = EXCLUDED.progress_duration,
			last_season_watched  = EXCLUDED.last_season_watched,
			last_episode_watched = EXCLUDED.last_episode_watched,
			show_progress        = EXCLUDED.show_progress,
			last_updated         = EXCLUDED.last_updated
	`,
		item.ID, userID, item.Type, item.TmdbID, item.Title,
		item.PosterPath, item.BackdropPath,
		item.Progress.Watched, item.Progress.Duration,
		item.LastSeasonWatched, item.LastEpisodeWatched,
		showProgressJSON, item.LastUpdated,
	)
	return err
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

	err := row.Scan(
		&item.ID, &item.Type, &item.TmdbID, &item.Title,
		&item.PosterPath, &item.BackdropPath,
		&item.Progress.Watched, &item.Progress.Duration,
		&item.LastSeasonWatched, &item.LastEpisodeWatched,
		&showProgressJSON, &item.LastUpdated,
	)
	if err != nil {
		return nil, err
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
