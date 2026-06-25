ALTER TABLE watchlist_items
    ADD COLUMN IF NOT EXISTS episodes_watched INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS episodes_total   INTEGER NOT NULL DEFAULT 0;

UPDATE watchlist_items
   SET episodes_watched  = progress_watched::INT,
       episodes_total    = progress_duration::INT,
       progress_watched  = 0,
       progress_duration = 0
 WHERE media_type = 'tv'
   AND progress_duration > 0;
