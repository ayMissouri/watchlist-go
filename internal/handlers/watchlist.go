package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/calendar"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

const calendarSyncTimeout = 15 * time.Second

type WatchlistHandler struct {
	DB       *db.DB
	Tracker  *tracking.Service
	Calendar *calendar.Service
}

// GetAll godoc
// @Summary     Get watchlist
// @Description The user's watchlist, one page at a time. Filter by type or status, sort however you like.
// @Tags        watchlist
// @Produce     json
// @Param       page     query int    false "Page number"        default(1)
// @Param       per_page query int    false "Items per page"     default(20)
// @Param       type     query string false "Filter by type"     Enums(tv, movie)
// @Param       status   query string false "Filter by status"   Enums(watching, watched, plan_to_watch, paused, dropped)
// @Param       sort     query string false "Sort field"         Enums(last_updated, title)
// @Param       order    query string false "Sort order"         Enums(asc, desc)
// @Success     200 {object} models.WatchlistResponse
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist [get]
func (h *WatchlistHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	q := parseWatchlistQuery(r)

	items, total, err := h.DB.GetWatchlist(r.Context(), claims.UserID, q)
	if err != nil {
		jsonError(w, "could not fetch watchlist", http.StatusInternalServerError)
		return
	}

	totalPages := total / q.PerPage
	if total%q.PerPage != 0 {
		totalPages++
	}

	resp := models.WatchlistResponse{
		Items: items,
		Pagination: models.PaginationMeta{
			Page:       q.Page,
			PerPage:    q.PerPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	jsonOK(w, resp)
}

// GetOne godoc
// @Summary     Get watchlist item
// @Description Returns a single watchlist item by ID
// @Tags        watchlist
// @Produce     json
// @Param       id  path string true "Item ID (e.g. t63174 or m533535)"
// @Success     200 {object} models.WatchlistItem
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id} [get]
func (h *WatchlistHandler) GetOne(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	item, err := h.DB.GetItem(r.Context(), claims.UserID, itemID)
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	jsonOK(w, item)
}

// Upsert godoc
// @Summary     Add or update watchlist item
// @Description Adds the item, or replaces the whole thing if it's already there. Use the progress/status endpoints for partial updates.
// @Tags        watchlist
// @Accept      json
// @Produce     json
// @Param       id   path     string               true "Item ID (e.g. t63174 or m533535)"
// @Param       body body     models.WatchlistItem true "Watchlist item"
// @Success     204
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id} [put]
func (h *WatchlistHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	var item models.WatchlistItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	item.ID = itemID

	if item.Type != "tv" && item.Type != "movie" {
		jsonError(w, `type must be "tv" or "movie"`, http.StatusBadRequest)
		return
	}
	if item.Title == "" {
		jsonError(w, "title is required", http.StatusBadRequest)
		return
	}
	if item.Status == "" {
		item.Status = models.StatusPlanToWatch
	} else if !item.Status.Valid() {
		jsonError(w, "invalid status", http.StatusBadRequest)
		return
	}
	if item.LastUpdated == 0 {
		item.LastUpdated = time.Now().UnixMilli()
	}

	inserted, err := h.DB.UpsertItem(r.Context(), claims.UserID, &item)
	if err != nil {
		jsonError(w, "could not save item", http.StatusInternalServerError)
		return
	}

	if inserted {
		h.syncCalendar(claims.UserID, item)

		h.track(r, models.UserEvent{
			EventType: models.EventAdd,
			ItemID:    item.ID,
			MediaType: item.Type,
			ImdbID:    item.ImdbID,
			Title:     item.Title,
			Metadata:  map[string]any{"status": string(item.Status)},
		})

		if item.Status == models.StatusWatched {
			h.addPlay(r, &item)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WatchlistHandler) syncCalendar(userID string, item models.WatchlistItem) {
	if h.Calendar == nil || item.ImdbID == "" {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("watchlist: calendar sync panic for %s: %v", item.ID, rec)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), calendarSyncTimeout)
		defer cancel()
		h.Calendar.SyncItem(ctx, userID, item)
	}()
}

func (h *WatchlistHandler) pruneCalendar(ctx context.Context, userID string) {
	if err := h.DB.DeleteStaleCalendarEntries(ctx, userID); err != nil {
		log.Printf("watchlist: prune calendar for %s: %v", userID, err)
	}
}

func (h *WatchlistHandler) track(r *http.Request, ev models.UserEvent) {
	if h.Tracker == nil {
		return
	}
	ev.UserID = middleware.ClaimsFromCtx(r).UserID
	h.Tracker.Record(r.Context(), ev)
}

func (h *WatchlistHandler) addPlay(r *http.Request, item *models.WatchlistItem) int {
	userID := middleware.ClaimsFromCtx(r).UserID
	plays, err := h.DB.AddPlays(r.Context(), userID, item.ID, 1)
	if err != nil {
		log.Printf("watchlist: add play for %s: %v", item.ID, err)
		return item.Plays
	}
	item.Plays = plays

	if item.Type == "movie" {
		h.track(r, models.UserEvent{
			EventType:      models.EventMovieWatch,
			ItemID:         item.ID,
			MediaType:      item.Type,
			ImdbID:         item.ImdbID,
			Title:          item.Title,
			RuntimeMinutes: tracking.MinutesFromSeconds(item.Progress.Duration),
		})
	}
	return plays
}

func (h *WatchlistHandler) removePlay(r *http.Request, item *models.WatchlistItem) int {
	if item.Plays <= 0 {
		return 0
	}
	userID := middleware.ClaimsFromCtx(r).UserID
	plays, err := h.DB.AddPlays(r.Context(), userID, item.ID, -1)
	if err != nil {
		log.Printf("watchlist: remove play for %s: %v", item.ID, err)
		return item.Plays
	}
	item.Plays = plays

	if item.Type == "movie" {
		if _, err := h.DB.DeleteRecentEvents(r.Context(), userID, item.ID, models.EventMovieWatch, 1); err != nil {
			log.Printf("watchlist: drop watch event for %s: %v", item.ID, err)
		}
	}
	return plays
}

func playDelta(prev, next models.WatchlistStatus) int {
	switch {
	case next == models.StatusWatched && prev != models.StatusWatched:
		return 1
	case prev == models.StatusWatched && next != models.StatusWatched:
		return -1
	}
	return 0
}

func (h *WatchlistHandler) applyStatusPlay(r *http.Request, item *models.WatchlistItem, prev models.WatchlistStatus) {
	switch playDelta(prev, item.Status) {
	case 1:
		h.addPlay(r, item)
	case -1:
		h.removePlay(r, item)
	}
}

func detailLink(mediaType, imdbID string) string {
	if imdbID == "" {
		return ""
	}
	if mediaType == "movie" {
		return "/movie/" + imdbID
	}
	return "/series/" + imdbID
}

// UpdateProgress godoc
// @Summary     Update item progress
// @Description Just bumps playback progress. Everything else on the item stays as it was.
// @Tags        watchlist
// @Accept      json
// @Param       id   path     string                         true "Item ID"
// @Param       body body     models.UpdateProgressRequest   true "Progress update"
// @Success     204
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id}/progress [patch]
func (h *WatchlistHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	var req models.UpdateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status != "" && !req.Status.Valid() {
		jsonError(w, "invalid status", http.StatusBadRequest)
		return
	}

	// Load the existing item so it only overwrites the progress fields
	item, err := h.DB.GetItem(r.Context(), claims.UserID, itemID)
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	prevStatus := item.Status
	prevEpisodes := item.EpisodesWatched

	// Only overwrite the fields the request actually carries
	if req.Progress != nil {
		item.Progress = *req.Progress
	}
	if req.ShowProgress != nil {
		item.ShowProgress = req.ShowProgress
	}
	if req.LastSeasonWatched != nil {
		item.LastSeasonWatched = req.LastSeasonWatched
	}
	if req.LastEpisodeWatched != nil {
		item.LastEpisodeWatched = req.LastEpisodeWatched
	}
	if req.EpisodesWatched != nil {
		item.EpisodesWatched = *req.EpisodesWatched
	}
	if req.EpisodesTotal != nil {
		item.EpisodesTotal = *req.EpisodesTotal
	}

	if req.Status != "" {
		item.Status = req.Status
	} else if item.Status == models.StatusPlanToWatch || item.Status == "" {
		item.Status = models.StatusWatching
	}

	if req.LastUpdated != 0 {
		item.LastUpdated = req.LastUpdated
	} else {
		item.LastUpdated = time.Now().UnixMilli()
	}

	if _, err := h.DB.UpsertItem(r.Context(), claims.UserID, item); err != nil {
		jsonError(w, "could not update progress", http.StatusInternalServerError)
		return
	}

	if item.Status == models.StatusDropped {
		h.pruneCalendar(r.Context(), claims.UserID)
	}

	h.trackProgress(r, item, prevStatus, prevEpisodes, req)

	w.WriteHeader(http.StatusNoContent)
}

func (h *WatchlistHandler) trackProgress(r *http.Request, item *models.WatchlistItem, prevStatus models.WatchlistStatus, prevEpisodes int, req models.UpdateProgressRequest) {
	if item.Status != prevStatus {
		h.track(r, models.UserEvent{
			EventType: models.EventStatusChange,
			ItemID:    item.ID,
			MediaType: item.Type,
			ImdbID:    item.ImdbID,
			Title:     item.Title,
			Metadata:  map[string]any{"from": string(prevStatus), "to": string(item.Status)},
		})
		h.applyStatusPlay(r, item, prevStatus)
	}

	if item.Type != "tv" {
		return
	}

	switch {
	case item.EpisodesWatched > prevEpisodes:
		delta := item.EpisodesWatched - prevEpisodes
		runtime := 0
		if delta == 1 && req.Progress != nil {
			runtime = tracking.MinutesFromSeconds(req.Progress.Duration)
		}
		h.track(r, models.UserEvent{
			EventType:      models.EventEpisodeWatch,
			ItemID:         item.ID,
			MediaType:      item.Type,
			ImdbID:         item.ImdbID,
			Title:          item.Title,
			Season:         derefInt(item.LastSeasonWatched),
			Episode:        derefInt(item.LastEpisodeWatched),
			RuntimeMinutes: runtime,
			Metadata:       map[string]any{"delta": delta},
		})

	case item.EpisodesWatched < prevEpisodes:
		userID := middleware.ClaimsFromCtx(r).UserID
		n := prevEpisodes - item.EpisodesWatched
		if err := h.DB.RemoveEpisodeWatches(r.Context(), userID, item.ID, n); err != nil {
			log.Printf("watchlist: drop episode events for %s: %v", item.ID, err)
		}
	}
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// UpdateStatus godoc
// @Summary     Update item status
// @Description Sets the watch status of an item (watching, watched, plan_to_watch, paused, dropped)
// @Tags        watchlist
// @Accept      json
// @Param       id   path     string                      true "Item ID"
// @Param       body body     models.UpdateStatusRequest  true "Status update"
// @Success     204
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id}/status [patch]
func (h *WatchlistHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	var req models.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !req.Status.Valid() {
		jsonError(w, "invalid status", http.StatusBadRequest)
		return
	}

	item, err := h.DB.GetItem(r.Context(), claims.UserID, itemID)
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	prevStatus := item.Status
	item.Status = req.Status
	item.LastUpdated = time.Now().UnixMilli()

	if _, err := h.DB.UpsertItem(r.Context(), claims.UserID, item); err != nil {
		jsonError(w, "could not update status", http.StatusInternalServerError)
		return
	}

	if item.Status == models.StatusDropped {
		h.pruneCalendar(r.Context(), claims.UserID)
	}

	if req.Status != prevStatus {
		h.track(r, models.UserEvent{
			EventType: models.EventStatusChange,
			ItemID:    item.ID,
			MediaType: item.Type,
			ImdbID:    item.ImdbID,
			Title:     item.Title,
			Metadata:  map[string]any{"from": string(prevStatus), "to": string(req.Status)},
		})
		h.applyStatusPlay(r, item, prevStatus)
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddPlay godoc
// @Summary     Log another play
// @Description Records one more completed watch of an item and marks it watched. This is the
// @Description "watch it again" half of the checkbox prompt.
// @Tags        watchlist
// @Produce     json
// @Param       id path string true "Item ID"
// @Success     200 {object} models.PlaysResponse
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id}/plays [post]
func (h *WatchlistHandler) AddPlay(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	item, err := h.DB.GetItem(r.Context(), claims.UserID, chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	if item.Status != models.StatusWatched {
		prev := item.Status
		item.Status = models.StatusWatched
		item.LastUpdated = time.Now().UnixMilli()
		if _, err := h.DB.UpsertItem(r.Context(), claims.UserID, item); err != nil {
			jsonError(w, "could not log play", http.StatusInternalServerError)
			return
		}
		h.track(r, models.UserEvent{
			EventType: models.EventStatusChange,
			ItemID:    item.ID,
			MediaType: item.Type,
			ImdbID:    item.ImdbID,
			Title:     item.Title,
			Metadata:  map[string]any{"from": string(prev), "to": string(item.Status)},
		})
	}

	jsonOK(w, models.PlaysResponse{Plays: h.addPlay(r, item), Status: item.Status})
}

// RemovePlay godoc
// @Summary     Undo the newest play
// @Description Drops the most recent play and the watch time that came with it. The item goes
// @Description back to plan_to_watch once no plays are left.
// @Tags        watchlist
// @Produce     json
// @Param       id path string true "Item ID"
// @Success     200 {object} models.PlaysResponse
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id}/plays [delete]
func (h *WatchlistHandler) RemovePlay(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	item, err := h.DB.GetItem(r.Context(), claims.UserID, chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	plays := h.removePlay(r, item)

	if plays == 0 && item.Status == models.StatusWatched {
		prev := item.Status
		item.Status = models.StatusPlanToWatch
		item.LastUpdated = time.Now().UnixMilli()
		if _, err := h.DB.UpsertItem(r.Context(), claims.UserID, item); err != nil {
			jsonError(w, "could not undo play", http.StatusInternalServerError)
			return
		}
		h.track(r, models.UserEvent{
			EventType: models.EventStatusChange,
			ItemID:    item.ID,
			MediaType: item.Type,
			ImdbID:    item.ImdbID,
			Title:     item.Title,
			Metadata:  map[string]any{"from": string(prev), "to": string(item.Status)},
		})
	}

	jsonOK(w, models.PlaysResponse{Plays: plays, Status: item.Status})
}

// Delete godoc
// @Summary     Remove watchlist item
// @Description Deletes a watchlist item by ID
// @Tags        watchlist
// @Param       id path string true "Item ID"
// @Success     204
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist/{id} [delete]
func (h *WatchlistHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	removed, _ := h.DB.GetItem(r.Context(), claims.UserID, itemID)

	if err := h.DB.DeleteItem(r.Context(), claims.UserID, itemID); err != nil {
		if err.Error() == "not found" {
			jsonError(w, "item not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not delete item", http.StatusInternalServerError)
		return
	}

	h.pruneCalendar(r.Context(), claims.UserID)

	ev := models.UserEvent{EventType: models.EventRemove, ItemID: itemID}
	if removed != nil {
		ev.MediaType = removed.Type
		ev.ImdbID = removed.ImdbID
		ev.Title = removed.Title
		ev.Metadata = map[string]any{"status": string(removed.Status)}
	}
	h.track(r, ev)

	w.WriteHeader(http.StatusNoContent)
}

// BulkDelete godoc
// @Summary     Bulk delete watchlist items
// @Description Deletes several items in one go and tells you how many actually went.
// @Tags        watchlist
// @Accept      json
// @Produce     json
// @Param       body body models.BulkDeleteRequest true "List of item IDs to delete"
// @Success     200 {object} map[string]int64
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /watchlist [delete]
func (h *WatchlistHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	var req models.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		jsonError(w, "ids must not be empty", http.StatusBadRequest)
		return
	}

	deleted, err := h.DB.BulkDeleteItems(r.Context(), claims.UserID, req.IDs)
	if err != nil {
		jsonError(w, "could not delete items", http.StatusInternalServerError)
		return
	}

	h.pruneCalendar(r.Context(), claims.UserID)

	h.track(r, models.UserEvent{
		EventType: models.EventBulkRemove,
		Metadata:  map[string]any{"count": deleted, "ids": req.IDs},
	})

	jsonOK(w, map[string]int64{"deleted": deleted})
}

// Helper functions.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := fmt.Fprintf(w, `{"error":%q}`, msg); err != nil {
		return
	}
}
