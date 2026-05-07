package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

type DB struct {
	Pool *pgxpool.Pool
}

// context.Context is very important. Every function that does I/O accepts a ctx as its first argument.
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