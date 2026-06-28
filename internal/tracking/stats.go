package tracking

import (
	"context"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	profileTopLimit = 8
	wrappedTopLimit = 5
)

var epochStart = time.Unix(0, 0)

// all-time summary for a user profile page.
func (s *Service) Profile(ctx context.Context, userID string) (models.ProfileStats, error) {
	end := time.Now().AddDate(1, 0, 0)

	totals, err := s.DB.GetEventTotals(ctx, userID, epochStart, end)
	if err != nil {
		return models.ProfileStats{}, err
	}

	genres, err := s.DB.GetTopGenres(ctx, userID, epochStart, end, profileTopLimit)
	if err != nil {
		return models.ProfileStats{}, err
	}
	titles, err := s.DB.GetTopTitles(ctx, userID, epochStart, end, profileTopLimit)
	if err != nil {
		return models.ProfileStats{}, err
	}
	days, err := s.DB.GetDistinctWatchDays(ctx, userID, epochStart, end)
	if err != nil {
		return models.ProfileStats{}, err
	}

	return models.ProfileStats{
		TotalEvents:       totals.Total,
		MoviesWatched:     totals.Movies,
		EpisodesWatched:   totals.Episodes,
		ShowsCompleted:    totals.ShowsCompleted,
		ItemsAdded:        totals.ItemsAdded,
		WatchTimeMinutes:  totals.WatchMinutes,
		CurrentStreakDays: currentStreakDays(days),
		LongestStreakDays: longestStreakDays(days),
		TopGenres:         genres,
		TopTitles:         titles,
		FirstEventAt:      totals.FirstAt,
		LastEventAt:       totals.LastAt,
	}, nil
}

func currentStreakDays(days []time.Time) int {
	if len(days) == 0 {
		return 0
	}
	const day = 24 * time.Hour
	today := time.Now().UTC().Truncate(day)
	last := days[len(days)-1].UTC().Truncate(day)
	if gap := today.Sub(last); gap != 0 && gap != day {
		return 0
	}
	streak := 1
	for i := len(days) - 1; i > 0; i-- {
		switch days[i].UTC().Truncate(day).Sub(days[i-1].UTC().Truncate(day)) {
		case day:
			streak++
		case 0:
			// Same calendar day twice — ignore the duplicate.
		default:
			return streak
		}
	}
	return streak
}

// Wrapped builds the year-in-review for a given calendar year.
func (s *Service) Wrapped(ctx context.Context, userID string, year int) (models.WrappedStats, error) {
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)

	totals, err := s.DB.GetEventTotals(ctx, userID, start, end)
	if err != nil {
		return models.WrappedStats{}, err
	}
	genres, err := s.DB.GetTopGenres(ctx, userID, start, end, wrappedTopLimit)
	if err != nil {
		return models.WrappedStats{}, err
	}
	titles, err := s.DB.GetTopTitles(ctx, userID, start, end, wrappedTopLimit)
	if err != nil {
		return models.WrappedStats{}, err
	}
	decades, err := s.DB.GetDecadeBreakdown(ctx, userID, start, end, wrappedTopLimit)
	if err != nil {
		return models.WrappedStats{}, err
	}
	byMonth, err := s.DB.GetWatchHistogram(ctx, userID, start, end, "month", 12)
	if err != nil {
		return models.WrappedStats{}, err
	}
	byHour, err := s.DB.GetWatchHistogram(ctx, userID, start, end, "hour", 24)
	if err != nil {
		return models.WrappedStats{}, err
	}
	byWeekday, err := s.DB.GetWatchHistogram(ctx, userID, start, end, "dow", 7)
	if err != nil {
		return models.WrappedStats{}, err
	}
	days, err := s.DB.GetDistinctWatchDays(ctx, userID, start, end)
	if err != nil {
		return models.WrappedStats{}, err
	}
	firstWatch, err := s.DB.GetEdgeWatch(ctx, userID, start, end, false)
	if err != nil {
		return models.WrappedStats{}, err
	}
	lastWatch, err := s.DB.GetEdgeWatch(ctx, userID, start, end, true)
	if err != nil {
		return models.WrappedStats{}, err
	}

	totalWatches := totals.Movies + totals.Episodes
	out := models.WrappedStats{
		Year:              year,
		WatchTimeMinutes:  totals.WatchMinutes,
		MoviesWatched:     totals.Movies,
		EpisodesWatched:   totals.Episodes,
		ShowsStarted:      totals.ShowsStarted,
		ShowsCompleted:    totals.ShowsCompleted,
		ItemsAdded:        totals.ItemsAdded,
		UniqueTitles:      totals.UniqueTitles,
		Searches:          totals.Searches,
		AppOpens:          totals.AppOpens,
		TopGenres:         genres,
		TopTitles:         titles,
		Decades:           decades,
		WatchesByMonth:    byMonth,
		WatchesByHour:     byHour,
		WatchesByWeekday:  byWeekday,
		PeakHour:          maxIndex(byHour),
		NightOwl:          isNightOwl(byHour),
		LongestStreakDays: longestStreakDays(days),
		FirstWatch:        firstWatch,
		LastWatch:         lastWatch,
	}
	if totalWatches > 0 {
		out.BusiestMonth = monthName(maxIndex(byMonth) + 1)
	}
	return out, nil
}

func longestStreakDays(days []time.Time) int {
	if len(days) == 0 {
		return 0
	}
	longest, current := 1, 1
	for i := 1; i < len(days); i++ {
		gap := days[i].Sub(days[i-1])
		switch gap {
		case 24 * time.Hour:
			current++
			if current > longest {
				longest = current
			}
		case 0:
		default:
			current = 1
		}
	}
	return longest
}

func maxIndex(counts []int) int {
	best, idx := -1, 0
	for i, c := range counts {
		if c > best {
			best, idx = c, i
		}
	}
	return idx
}

var nightHours = map[int]bool{22: true, 23: true, 0: true, 1: true, 2: true, 3: true, 4: true}

// isNightOwl reports whether at least 30% of watches happened in the night-hour
func isNightOwl(byHour []int) bool {
	total, night := 0, 0
	for h, c := range byHour {
		total += c
		if nightHours[h] {
			night += c
		}
	}
	return total > 0 && night*100 >= total*30
}

func monthName(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return time.Month(m).String()
}
