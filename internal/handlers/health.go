package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
)

// HealthResponse is what /health hands back.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// Health godoc
// @Summary     Health check
// @Description Quick liveness check. Also says whether the database is reachable.
// @Tags        system
// @Produce     json
// @Success     200 {object} HealthResponse
// @Router      /health [get]
func Health(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := database.Ping(r.Context()); err != nil {
			dbStatus = "unreachable"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := HealthResponse{
			Status:   "ok",
			Database: dbStatus,
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}