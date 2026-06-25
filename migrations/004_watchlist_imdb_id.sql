ALTER TABLE watchlist_items
    ADD COLUMN IF NOT EXISTS imdb_id TEXT;

UPDATE watchlist_items SET imdb_id = 'tt4052886' WHERE id = 't63174'  AND imdb_id IS NULL;
UPDATE watchlist_items SET imdb_id = 'tt6263850' WHERE id = 'm533535' AND imdb_id IS NULL;
