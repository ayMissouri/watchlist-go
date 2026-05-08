CREATE TABLE IF NOT EXISTS users (
    id         TEXT        PRIMARY KEY,
    username   TEXT        NOT NULL,
    avatar     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);