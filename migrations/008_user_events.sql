CREATE TABLE IF NOT EXISTS user_events (
    id              BIGSERIAL   PRIMARY KEY,
    user_id         TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    source          TEXT        NOT NULL DEFAULT 'server',
    item_id         TEXT,
    media_type      TEXT,
    imdb_id         TEXT,
    title           TEXT,
    season          INTEGER,
    episode         INTEGER,
    runtime_minutes INTEGER,
    genres          TEXT[],
    release_year    INTEGER,
    metadata        JSONB       NOT NULL DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS user_events_user_time_idx
    ON user_events (user_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS user_events_user_type_time_idx
    ON user_events (user_id, event_type, occurred_at DESC);

CREATE INDEX IF NOT EXISTS user_events_genres_idx
    ON user_events USING GIN (genres);
