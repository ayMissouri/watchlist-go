package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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