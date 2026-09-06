package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestJsonError(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonError(rr, "something went wrong", http.StatusBadRequest)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected Content-Type: application/json")
	}
}

func TestPlayDelta(t *testing.T) {
	const (
		watched = models.StatusWatched
		plan    = models.StatusPlanToWatch
		going   = models.StatusWatching
	)
	cases := []struct {
		prev, next models.WatchlistStatus
		want       int
	}{
		{plan, watched, 1},
		{going, watched, 1},
		{watched, plan, -1},
		{watched, models.StatusDropped, -1},
		{watched, watched, 0},
		{plan, going, 0},
	}
	for _, c := range cases {
		if got := playDelta(c.prev, c.next); got != c.want {
			t.Errorf("playDelta(%s, %s) = %d, want %d", c.prev, c.next, got, c.want)
		}
	}

	total := 0
	status := plan
	for range 5 {
		total += playDelta(status, watched)
		status = watched
		total += playDelta(status, plan)
		status = plan
	}
	if total != 0 {
		t.Errorf("watched/unwatched spam netted %d plays, want 0", total)
	}
}
