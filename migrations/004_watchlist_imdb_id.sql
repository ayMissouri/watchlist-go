ALTER TABLE watchlist_items
    ADD COLUMN IF NOT EXISTS imdb_id TEXT;