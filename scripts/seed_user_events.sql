WITH me AS (
    SELECT 'REPLACE_WITH_YOUR_USER_ID'::text AS user_id
)
INSERT INTO user_events
    (user_id, event_type, source, item_id, media_type, imdb_id, title,
     season, episode, runtime_minutes, genres, release_year, metadata, occurred_at)
SELECT
    me.user_id, v.event_type, 'seed', v.item_id, v.media_type, v.imdb_id, v.title,
    v.season, v.episode, v.runtime_minutes, v.genres, v.release_year,
    v.metadata::jsonb, v.occurred_at
FROM me CROSS JOIN (VALUES
    -- movies watched
    ('movie_watch','m1375666','movie','tt1375666','Inception',     0,0,148, ARRAY['Action','Sci-Fi']::text[], 2010, '{}', '2026-01-04 22:30:00+00'::timestamptz),
    ('movie_watch','m133093', 'movie','tt0133093','The Matrix',     0,0,136, ARRAY['Action','Sci-Fi'],         1999, '{}', '2026-02-14 01:15:00+00'),
    ('movie_watch','m496243', 'movie','tt6751668','Parasite',       0,0,132, ARRAY['Drama','Thriller'],         2019, '{}', '2026-03-09 20:00:00+00'),
    ('movie_watch','m949',    'movie','tt0113277','Heat',           0,0,170, ARRAY['Crime','Drama'],            1995, '{}', '2026-04-20 23:45:00+00'),
    ('movie_watch','m157336', 'movie','tt0816692','Interstellar',   0,0,169, ARRAY['Sci-Fi','Drama'],           2014, '{}', '2026-05-30 21:00:00+00'),
    ('movie_watch','m155',    'movie','tt0468569','The Dark Knight',0,0,152, ARRAY['Action','Crime'],           2008, '{}', '2026-07-15 00:30:00+00'),
    ('movie_watch','m680',    'movie','tt0110912','Pulp Fiction',   0,0,154, ARRAY['Crime','Drama'],            1994, '{}', '2026-09-10 02:00:00+00'),
    ('movie_watch','m244786', 'movie','tt2582802','Whiplash',       0,0,106, ARRAY['Drama','Music'],            2014, '{}', '2026-11-22 19:30:00+00'),

    -- Breaking Bad with a 6 day streak
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,1,47, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-04 23:10:00+00'),
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,2,48, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-05 23:30:00+00'),
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,3,48, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-06 22:50:00+00'),
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,4,48, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-07 00:20:00+00'),
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,5,47, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-08 23:00:00+00'),
    ('episode_watch','t1396','tv','tt0903747','Breaking Bad',1,6,47, ARRAY['Crime','Drama'],2008, '{"delta":1}', '2026-01-09 21:40:00+00'),

    -- The Office
    ('episode_watch','t2316','tv','tt0386676','The Office',2,1,22, ARRAY['Comedy'],2005, '{"delta":1}', '2026-02-01 12:00:00+00'),
    ('episode_watch','t2316','tv','tt0386676','The Office',2,2,22, ARRAY['Comedy'],2005, '{"delta":1}', '2026-02-02 12:30:00+00'),
    ('episode_watch','t2316','tv','tt0386676','The Office',2,3,22, ARRAY['Comedy'],2005, '{"delta":1}', '2026-05-05 18:00:00+00'),
    ('episode_watch','t2316','tv','tt0386676','The Office',2,4,22, ARRAY['Comedy'],2005, '{"delta":1}', '2026-08-08 13:00:00+00'),

    -- Severance
    ('episode_watch','t95396','tv','tt11280740','Severance',1,1,50, ARRAY['Drama','Sci-Fi','Thriller'],2022, '{"delta":1}', '2026-10-10 22:00:00+00'),
    ('episode_watch','t95396','tv','tt11280740','Severance',1,2,50, ARRAY['Drama','Sci-Fi','Thriller'],2022, '{"delta":1}', '2026-10-11 23:30:00+00'),

    -- a completed show
    ('status_change','t1396','tv','tt0903747','Breaking Bad',0,0,NULL, NULL,NULL, '{"from":"watching","to":"watched"}', '2026-01-09 21:50:00+00'),

    -- watchlist additions
    ('add','m157336','movie','tt0816692','Interstellar',0,0,NULL, NULL,NULL, '{"status":"plan_to_watch"}', '2026-05-29 10:00:00+00'),
    ('add','t95396', 'tv',   'tt11280740','Severance', 0,0,NULL, NULL,NULL, '{"status":"plan_to_watch"}', '2026-10-09 09:00:00+00'),
    ('add','m244786','movie','tt2582802','Whiplash',   0,0,NULL, NULL,NULL, '{"status":"plan_to_watch"}', '2026-11-20 08:00:00+00'),

    -- searches
    ('search',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"query":"interstellar","results":4}', '2026-05-29 09:58:00+00'),
    ('search',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"query":"severance","results":6}',    '2026-10-09 08:55:00+00'),
    ('search',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"query":"whiplash","results":3}',     '2026-11-20 07:55:00+00'),

    -- logins / app opens
    ('login',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"ip":"86.21.4.10","user_agent":"Mozilla/5.0"}', '2026-01-04 18:00:00+00'),
    ('login',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"ip":"86.21.4.10","user_agent":"Mozilla/5.0"}', '2026-05-29 09:50:00+00'),
    ('login',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"ip":"86.21.4.10","user_agent":"Mozilla/5.0"}', '2026-10-09 08:50:00+00'),
    ('login',NULL,NULL,NULL,NULL,0,0,NULL, NULL,NULL, '{"ip":"86.21.4.10","user_agent":"Mozilla/5.0"}', '2026-11-20 07:50:00+00')
) AS v(event_type, item_id, media_type, imdb_id, title,
       season, episode, runtime_minutes, genres, release_year, metadata, occurred_at);

-- Wipe the seeded data (does NOT touch real events):
--   DELETE FROM user_events WHERE source = 'seed';
