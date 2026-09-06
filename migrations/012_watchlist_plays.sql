ALTER TABLE watchlist_items
    ADD COLUMN IF NOT EXISTS plays INTEGER NOT NULL DEFAULT 0;

UPDATE watchlist_items SET plays = 1 WHERE status = 'watched' AND plays = 0;

CREATE INDEX IF NOT EXISTS user_events_user_item_type_time_idx
    ON user_events (user_id, item_id, event_type, occurred_at DESC);
