package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterHealthRoute(t *testing.T) {
	router := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}