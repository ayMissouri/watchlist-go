package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/meta"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestHealthRoute_NoDB(t *testing.T) {
	router := NewRouter(&db.DB{}, meta.NewClient())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestLobbyRoutes(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-that-is-long-enough-for-hs256")
	token, err := auth.IssueJWT("u1", "alice")
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(&db.DB{}, meta.NewClient())

	req := httptest.NewRequest(http.MethodPost, "/lobbies", strings.NewReader(`{"item_id":"tt1"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var created models.Lobby
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.Code == "" {
		t.Fatalf("create: bad body %s", rr.Body)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	req = httptest.NewRequest(http.MethodGet, "/lobbies/"+created.Code+"/stream", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	if body := rr.Body.String(); !strings.HasPrefix(body, "event: state\ndata: ") || !strings.Contains(body, `"username":"alice"`) {
		t.Fatalf("stream: unexpected body %q", body)
	}
}
