CREATE TABLE IF NOT EXISTS calendar_entries (
    id            BIGSERIAL   PRIMARY KEY,
    user_id       TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id       TEXT        NOT NULL,
    media_type    TEXT        NOT NULL CHECK (media_type IN ('tv', 'movie')),
    imdb_id       TEXT,
    title         TEXT        NOT NULL,
    poster_path   TEXT,
    season        INTEGER     NOT NULL DEFAULT 0,
    episode       INTEGER     NOT NULL DEFAULT 0,
    episode_title TEXT,
    release_date  TIMESTAMPTZ NOT NULL,
    created_at    BIGINT      NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT * 1000,

    UNIQUE (user_id, item_id, season, episode)
);

CREATE INDEX IF NOT EXISTS calendar_entries_user_release_idx
    ON calendar_entries (user_id, release_date);
