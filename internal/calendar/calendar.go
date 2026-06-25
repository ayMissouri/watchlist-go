package calendar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

// defaultInterval is how often the daily job re-syncs every users calendar.
const defaultInterval = 24 * time.Hour

// startupDelay is the delay before the first run of the daily job.
const startupDelay = 30 * time.Second

type Service struct {
	DB   *db.DB
	Meta *meta.Client
}

func NewService(database *db.DB, metaClient *meta.Client) *Service {
	return &Service{DB: database, Meta: metaClient}
}

func (s *Service) SyncUser(ctx context.Context, userID string) error {
	items, err := s.DB.GetWatchlistForSync(ctx, userID)
	if err != nil {
		return fmt.Errorf("load watchlist: %w", err)
	}

	now := time.Now()
	for _, item := range items {
		if item.ImdbID == "" {
			continue
		}
		switch item.Type {
		case "tv":
			s.syncSeries(ctx, userID, item, now)
		case "movie":
			s.syncMovie(ctx, userID, item, now)
		}
	}

	if err := s.DB.DeleteStaleCalendarEntries(ctx, userID); err != nil {
		return fmt.Errorf("prune stale entries: %w", err)
	}
	return nil
}

func (s *Service) syncSeries(ctx context.Context, userID string, item models.WatchlistItem, now time.Time) {
	detail, err := s.Meta.SeriesDetail(ctx, item.ImdbID)
	if err != nil {
		if !errors.Is(err, meta.ErrNotFound) {
			log.Printf("calendar: series %s: %v", item.ImdbID, err)
		}
		return
	}

	for _, ep := range detail.Videos {
		release, ok := parseReleaseDate(ep.Released)

		if !ok || !release.After(now) {
			continue
		}

		entry := &models.CalendarEntry{
			UserID:       userID,
			ItemID:       item.ID,
			MediaType:    "tv",
			ImdbID:       item.ImdbID,
			Title:        item.Title,
			PosterPath:   item.PosterPath,
			Season:       ep.Season,
			Episode:      ep.Episode,
			EpisodeTitle: ep.Title,
		}
		if err := s.DB.UpsertCalendarEntry(ctx, entry, release); err != nil {
			log.Printf("calendar: upsert %s s%de%d: %v", item.ID, ep.Season, ep.Episode, err)
		}
	}
}

func (s *Service) syncMovie(ctx context.Context, userID string, item models.WatchlistItem, now time.Time) {
	detail, err := s.Meta.MovieDetail(ctx, item.ImdbID)
	if err != nil {
		if !errors.Is(err, meta.ErrNotFound) {
			log.Printf("calendar: movie %s: %v", item.ImdbID, err)
		}
		return
	}

	release, ok := parseMovieRelease(detail)
	// Only unreleased movies belong on the calendar.
	if !ok || !release.After(now) {
		return
	}

	entry := &models.CalendarEntry{
		UserID:     userID,
		ItemID:     item.ID,
		MediaType:  "movie",
		ImdbID:     item.ImdbID,
		Title:      item.Title,
		PosterPath: item.PosterPath,
	}
	if err := s.DB.UpsertCalendarEntry(ctx, entry, release); err != nil {
		log.Printf("calendar: upsert movie %s: %v", item.ID, err)
	}
}

func (s *Service) SyncAll(ctx context.Context) error {
	userIDs, err := s.DB.ListUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	for _, userID := range userIDs {
		if err := s.SyncUser(ctx, userID); err != nil {
			log.Printf("calendar: sync user %s: %v", userID, err)
		}
	}
	return nil
}

func (s *Service) ProcessReleased(ctx context.Context) error {
	due, err := s.DB.GetDueCalendarEntries(ctx)
	if err != nil {
		return fmt.Errorf("load due entries: %w", err)
	}

	for _, e := range due {
		n := &models.Notification{
			Type:       "release",
			Title:      e.Title,
			Body:       releaseBody(e),
			PosterPath: e.PosterPath,
			Link:       detailLink(e.MediaType, e.ImdbID),
		}
		if err := s.DB.CreateNotification(ctx, e.UserID, n); err != nil {
			log.Printf("calendar: notify user %s for entry %d: %v", e.UserID, e.ID, err)
			continue
		}
		if err := s.DB.DeleteCalendarEntry(ctx, e.ID); err != nil {
			log.Printf("calendar: delete released entry %d: %v", e.ID, err)
		}
	}
	return nil
}

func (s *Service) RunDaily(ctx context.Context) {
	interval := syncInterval()
	timer := time.NewTimer(startupDelay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.runOnce(ctx)
			timer.Reset(interval)
		}
	}
}

func (s *Service) runOnce(ctx context.Context) {
	log.Println("calendar: daily sync starting")
	if err := s.SyncAll(ctx); err != nil {
		log.Printf("calendar: sync all: %v", err)
	}
	if err := s.ProcessReleased(ctx); err != nil {
		log.Printf("calendar: process released: %v", err)
	}
	log.Println("calendar: daily sync complete")
}

func syncInterval() time.Duration {
	if v := os.Getenv("CALENDAR_SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultInterval
}

func releaseBody(e models.CalendarEntry) string {
	if e.MediaType == "tv" {
		if e.EpisodeTitle != "" {
			return fmt.Sprintf("S%dE%d · %s is now available", e.Season, e.Episode, e.EpisodeTitle)
		}
		return fmt.Sprintf("S%dE%d is now available", e.Season, e.Episode)
	}
	return "Now available"
}

func detailLink(mediaType, imdbID string) string {
	if imdbID == "" {
		return ""
	}
	if mediaType == "movie" {
		return "/movie/" + imdbID
	}
	return "/series/" + imdbID
}

var releaseLayouts = []string{
	time.RFC3339Nano, // 2025-07-15T00:00:00.000Z
	time.RFC3339,     // 2025-07-15T00:00:00Z
	"2006-01-02",
	"2006-01",
	"2006",
}

func parseReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range releaseLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseMovieRelease(d *models.MovieDetail) (time.Time, bool) {
	for _, s := range []string{d.Released, d.ReleaseInfo, d.Year} {
		if t, ok := parseReleaseDate(s); ok {
			return t, true
		}
	}
	return time.Time{}, false
}
