package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

const (
	defaultEventsLimit = 50
	maxEventsLimit     = 200
)

type EventsHandler struct {
	DB      *db.DB
	Tracker *tracking.Service
}

// Record godoc
// @Summary     Ingest activity events
// @Description Accepts a batch of client-reported events (play/pause, heartbeats, page views, etc.) and appends them to the user's activity log. Events with an empty or over-long type are skipped.
// @Tags        events
// @Accept      json
// @Param       body body models.RecordEventsRequest true "Batch of events"
// @Success     204
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /events [post]
func (h *EventsHandler) Record(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	var req models.RecordEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	events, ok := tracking.ValidClientEvents(req.Events)
	if !ok {
		jsonError(w, "events must be a non-empty batch of at most 100 items", http.StatusBadRequest)
		return
	}

	h.Tracker.RecordClientEvents(r.Context(), claims.UserID, events)
	w.WriteHeader(http.StatusNoContent)
}

// List godoc
// @Summary     Recent activity
// @Description Returns the user's most recent tracked events, newest first. Useful for a transparency/activity feed.
// @Tags        events
// @Produce     json
// @Param       limit query int false "Max events to return" default(50)
// @Success     200 {object} models.EventsResponse
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /events [get]
func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	limit := defaultEventsLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > maxEventsLimit {
				v = maxEventsLimit
			}
			limit = v
		}
	}

	items, err := h.DB.GetEvents(r.Context(), claims.UserID, limit)
	if err != nil {
		jsonError(w, "could not fetch events", http.StatusInternalServerError)
		return
	}

	jsonOK(w, models.EventsResponse{Items: items})
}
