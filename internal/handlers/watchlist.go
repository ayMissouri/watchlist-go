package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

type WatchlistHandler struct {
	DB *db.DB
}

// GET /watchlist
// Returns full watchlist for user.
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

// GET /watchlist/{id}
// Returns single item by ID.
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

// PUT /watchlist/{id}
// Creates or updates an item by ID.
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
	if item.LastUpdated == 0 {
		item.LastUpdated = time.Now().UnixMilli()
	}

	if err := h.DB.UpsertItem(r.Context(), claims.UserID, &item); err != nil {
		jsonError(w, "could not save item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PATCH /watchlist/{id}/progress
// UpdateProgress is for lightweight progress-only updates without needing to send the entire item.
func (h *WatchlistHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	var req models.UpdateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Load the existing item so it only overwrites the progress fields
	item, err := h.DB.GetItem(r.Context(), claims.UserID, itemID)
	if err != nil {
		jsonError(w, "item not found", http.StatusNotFound)
		return
	}

	item.Progress = req.Progress
	if req.ShowProgress != nil {
		item.ShowProgress = req.ShowProgress
	}
	if req.LastSeasonWatched != nil {
		item.LastSeasonWatched = req.LastSeasonWatched
	}
	if req.LastEpisodeWatched != nil {
		item.LastEpisodeWatched = req.LastEpisodeWatched
	}
	if req.LastUpdated != 0 {
		item.LastUpdated = req.LastUpdated
	} else {
		item.LastUpdated = time.Now().UnixMilli()
	}

	if err := h.DB.UpsertItem(r.Context(), claims.UserID, item); err != nil {
		jsonError(w, "could not update progress", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /watchlist/{id}
// Delete removes an item from the users watchlist by ID.
func (h *WatchlistHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)
	itemID := chi.URLParam(r, "id")

	if err := h.DB.DeleteItem(r.Context(), claims.UserID, itemID); err != nil {
		if err.Error() == "not found" {
			jsonError(w, "item not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not delete item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
