package tracking

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const enrichTimeout = 15 * time.Second

const maxClientBatch = 100

const maxEventTypeLen = 64

type Service struct {
	DB   *db.DB
	Meta *meta.Client
}

func NewService(database *db.DB, metaClient *meta.Client) *Service {
	return &Service{DB: database, Meta: metaClient}
}

func (s *Service) Record(ctx context.Context, ev models.UserEvent) {
	if s == nil || s.DB == nil {
		return
	}
	id, err := s.DB.RecordEvent(ctx, ev)
	if err != nil {
		log.Printf("tracking: record %s: %v", ev.EventType, err)
		return
	}
	if s.Meta != nil && ev.ImdbID != "" && len(ev.Genres) == 0 && enrichable(ev.EventType) {
		go s.enrich(id, ev.ImdbID, ev.MediaType)
	}
}

func (s *Service) RecordClientEvents(ctx context.Context, userID string, events []models.ClientEvent) {
	now := time.Now().UnixMilli()
	for _, ce := range events {
		ev := models.UserEvent{
			UserID:         userID,
			EventType:      ce.EventType,
			Source:         "client",
			ItemID:         ce.ItemID,
			MediaType:      ce.MediaType,
			ImdbID:         ce.ImdbID,
			Title:          ce.Title,
			Season:         ce.Season,
			Episode:        ce.Episode,
			RuntimeMinutes: ce.RuntimeMinutes,
			Genres:         ce.Genres,
			ReleaseYear:    ce.ReleaseYear,
			Metadata:       ce.Metadata,
			OccurredAt:     ce.OccurredAt,
		}
		if ev.OccurredAt <= 0 || ev.OccurredAt > now+60_000 {
			ev.OccurredAt = now
		}
		s.Record(ctx, ev)
	}
}

func (s *Service) enrich(eventID int64, imdbID, mediaType string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tracking: enrich panic for event %d: %v", eventID, r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()

	genres, year, runtime := s.lookupMeta(ctx, imdbID, mediaType)
	if len(genres) == 0 && year == 0 && runtime == 0 {
		return
	}
	if err := s.DB.EnrichEvent(ctx, eventID, genres, year, runtime); err != nil {
		log.Printf("tracking: enrich event %d: %v", eventID, err)
	}
}

func (s *Service) lookupMeta(ctx context.Context, imdbID, mediaType string) (genres []string, year, runtime int) {
	if mediaType == "movie" {
		d, err := s.Meta.MovieDetail(ctx, imdbID)
		if err != nil {
			return nil, 0, 0
		}
		return pickGenres(d.Genres, d.Genre), parseYear(d.Year), parseRuntimeMinutes(d.Runtime)
	}
	d, err := s.Meta.SeriesDetail(ctx, imdbID)
	if err != nil {
		return nil, 0, 0
	}
	return pickGenres(d.Genres, d.Genre), parseYear(d.Year), parseRuntimeMinutes(d.Runtime)
}

func ValidClientEvents(events []models.ClientEvent) ([]models.ClientEvent, bool) {
	if len(events) == 0 || len(events) > maxClientBatch {
		return nil, false
	}
	kept := make([]models.ClientEvent, 0, len(events))
	for _, e := range events {
		if e.EventType == "" || len(e.EventType) > maxEventTypeLen {
			continue
		}
		kept = append(kept, e)
	}
	return kept, true
}

func enrichable(t string) bool {
	switch t {
	case models.EventAdd, models.EventMovieWatch, models.EventEpisodeWatch, models.EventView:
		return true
	}
	return false
}

func pickGenres(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

var leadingInt = regexp.MustCompile(`\d+`)

func parseRuntimeMinutes(s string) int {
	m := leadingInt.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

func parseYear(s string) int {
	for i := 0; i+4 <= len(s); i++ {
		chunk := s[i : i+4]
		if isDigits(chunk) {
			n, _ := strconv.Atoi(chunk)
			if n >= 1870 && n <= 2999 {
				return n
			}
		}
	}
	return 0
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func MinutesFromSeconds(sec float64) int {
	if sec <= 0 {
		return 0
	}
	return int(sec/60 + 0.5)
}
