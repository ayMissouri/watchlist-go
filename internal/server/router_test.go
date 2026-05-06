package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
)

func TestHealthRoute_NoDB(t *testing.T) {
	router := NewRouter(&db.DB{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}