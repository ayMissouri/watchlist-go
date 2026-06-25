ALTER TABLE watchlist_items
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'plan_to_watch'
        CHECK (status IN ('watching', 'watched', 'plan_to_watch', 'paused', 'dropped'));
