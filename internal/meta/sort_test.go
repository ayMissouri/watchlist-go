package meta

import (
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestParseYear(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1994", 1994},
		{"2015–2018", 2018},
		{"2015-2018", 2018},
		{"", 0},
		{"TBA", 0},
		{"2024–", 2024},
	}

	for _, tt := range tests {
		got := parseYear(tt.input)
		if got != tt.expected {
			t.Errorf("parseYear(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestMergeAndWeight_NewerFirst(t *testing.T) {
	movies := []models.DiscoverItem{
		{ID: "tt1", Title: "Old Movie", Year: "1990", Type: "movie"},
		{ID: "tt2", Title: "New Movie", Year: "2023", Type: "movie"},
	}
	series := []models.DiscoverItem{
		{ID: "tt3", Title: "Recent Show", Year: "2021–2023", Type: "series"},
	}

	result := MergeAndWeight(movies, series)

	if result[0].ID != "tt2" {
		t.Errorf("expected tt2 (2023) first, got %s", result[0].ID)
	}
	if result[1].ID != "tt3" {
		t.Errorf("expected tt3 (2023) second, got %s", result[1].ID)
	}
	if result[2].ID != "tt1" {
		t.Errorf("expected tt1 (1990) last, got %s", result[2].ID)
	}
}

func TestMergeAndWeight_NoYear_SinksToBottom(t *testing.T) {
	items := []models.DiscoverItem{
		{ID: "tt1", Title: "No Year",   Year: ""},
		{ID: "tt2", Title: "Has Year",  Year: "2020"},
	}

	result := MergeAndWeight(items, nil)

	if result[0].ID != "tt2" {
		t.Errorf("expected tt2 (has year) first, got %s", result[0].ID)
	}
}