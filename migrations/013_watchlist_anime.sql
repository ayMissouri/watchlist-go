ALTER TABLE watchlist_items
    DROP CONSTRAINT IF EXISTS watchlist_items_media_type_check,
    ADD CONSTRAINT watchlist_items_media_type_check CHECK (media_type IN ('tv', 'movie', 'anime')),
    ADD COLUMN IF NOT EXISTS mal_id INTEGER NOT NULL DEFAULT 0;
