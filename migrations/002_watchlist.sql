CREATE TABLE IF NOT EXISTS watchlist_items (
    id                    TEXT    NOT NULL,
    user_id               TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_type            TEXT    NOT NULL CHECK (media_type IN ('tv', 'movie')),
    tmdb_id               INTEGER NOT NULL,
    title                 TEXT    NOT NULL,
    poster_path           TEXT,
    backdrop_path         TEXT,
    progress_watched      FLOAT8  NOT NULL DEFAULT 0,
    progress_duration     FLOAT8  NOT NULL DEFAULT 0,
    last_season_watched   INTEGER,
    last_episode_watched  INTEGER,
    show_progress         JSONB,
    last_updated          BIGINT  NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT * 1000,

    PRIMARY KEY (id, user_id)
);

CREATE INDEX IF NOT EXISTS watchlist_items_user_id_idx
    ON watchlist_items (user_id);
