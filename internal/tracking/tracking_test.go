package tracking

import (
	"testing"
	"time"
)

func TestParseRuntimeMinutes(t *testing.T) {
	cases := map[string]int{
		"120 min": 120,
		"47":      47,
		"1h 30m":  1,
		"":        0,
		"min":     0,
		"90 mins": 90,
	}
	for in, want := range cases {
		if got := parseRuntimeMinutes(in); got != want {
			t.Errorf("parseRuntimeMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseYear(t *testing.T) {
	cases := map[string]int{
		"2014":       2014,
		"2014–2019":  2014,
		"":           0,
		"abcd":       0,
		"99":         0,
		"The 2010s":  2010,
		"3001 movie": 0,
	}
	for in, want := range cases {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestMinutesFromSeconds(t *testing.T) {
	cases := map[float64]int{
		0:      0,
		-5:     0,
		30:     1,
		29:     0,
		7200:   120,
		2730.0: 46,
	}
	for in, want := range cases {
		if got := MinutesFromSeconds(in); got != want {
			t.Errorf("MinutesFromSeconds(%v) = %d, want %d", in, got, want)
		}
	}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestLongestStreakDays(t *testing.T) {
	cases := []struct {
		name string
		days []time.Time
		want int
	}{
		{"empty", nil, 0},
		{"single", []time.Time{day(2026, 1, 1)}, 1},
		{
			"three in a row",
			[]time.Time{day(2026, 1, 1), day(2026, 1, 2), day(2026, 1, 3)},
			3,
		},
		{
			"gap resets",
			[]time.Time{day(2026, 1, 1), day(2026, 1, 2), day(2026, 1, 5), day(2026, 1, 6), day(2026, 1, 7)},
			3,
		},
		{
			"duplicates ignored",
			[]time.Time{day(2026, 1, 1), day(2026, 1, 1), day(2026, 1, 2)},
			2,
		},
		{
			"crosses month boundary",
			[]time.Time{day(2026, 1, 31), day(2026, 2, 1), day(2026, 2, 2)},
			3,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := longestStreakDays(c.days); got != c.want {
				t.Fatalf("longestStreakDays = %d, want %d", got, c.want)
			}
		})
	}
}

func TestMaxIndex(t *testing.T) {
	cases := []struct {
		in   []int
		want int
	}{
		{nil, 0},
		{[]int{0, 0, 0}, 0},
		{[]int{1, 5, 3}, 1},
		{[]int{5, 5, 3}, 0},
		{[]int{0, 0, 9}, 2},
	}
	for _, c := range cases {
		if got := maxIndex(c.in); got != c.want {
			t.Errorf("maxIndex(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestIsNightOwl(t *testing.T) {
	byHour := make([]int, 24)
	if isNightOwl(byHour) {
		t.Error("all-zero should not be a night owl")
	}

	byHour[23] = 2
	byHour[1] = 2
	byHour[14] = 6
	if !isNightOwl(byHour) {
		t.Error("expected night owl at 40% night watches")
	}

	byHour = make([]int, 24)
	byHour[2] = 1
	byHour[20] = 9
	if isNightOwl(byHour) {
		t.Error("did not expect night owl at 10% night watches")
	}
}

func TestPickGenres(t *testing.T) {
	if got := pickGenres([]string{"Drama"}, []string{"Comedy"}); got[0] != "Drama" {
		t.Errorf("expected primary, got %v", got)
	}
	if got := pickGenres(nil, []string{"Comedy"}); got[0] != "Comedy" {
		t.Errorf("expected fallback, got %v", got)
	}
	if got := pickGenres(nil, nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
