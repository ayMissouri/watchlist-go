CREATE TABLE IF NOT EXISTS notifications (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT        NOT NULL DEFAULT 'general',
    title       TEXT        NOT NULL,
    body        TEXT        NOT NULL DEFAULT '',
    poster_path TEXT,
    link        TEXT,
    read        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  BIGINT      NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT * 1000
);

CREATE INDEX IF NOT EXISTS notifications_user_id_idx
    ON notifications (user_id, created_at DESC);
