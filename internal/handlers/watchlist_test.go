package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
