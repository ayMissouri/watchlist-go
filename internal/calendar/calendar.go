package calendar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

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

func (s *Service) SyncItem(ctx context.Context, userID string, item models.WatchlistItem) {
	if item.ImdbID == "" || item.Status == models.StatusDropped {
		return
	}
	now := time.Now()
	switch item.Type {
	case "tv":
		s.syncSeries(ctx, userID, item, now)
	case "movie":
		s.syncMovie(ctx, userID, item, now)
	}
}

func (s *Service) ProcessReleasedForUser(ctx context.Context, userID string) error {
	due, err := s.DB.GetDueCalendarEntriesForUser(ctx, userID)
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
