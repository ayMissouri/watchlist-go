package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

type NotificationsHandler struct {
	DB *db.DB
}

// GetAll godoc
// @Summary     List notifications
// @Description Returns the user's most recent notifications plus the unread count
// @Tags        notifications
// @Produce     json
// @Success     200 {object} models.NotificationsResponse
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /notifications [get]
func (h *NotificationsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	items, err := h.DB.GetNotifications(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "could not fetch notifications", http.StatusInternalServerError)
		return
	}

	unread, err := h.DB.CountUnreadNotifications(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "could not fetch notifications", http.StatusInternalServerError)
		return
	}

	jsonOK(w, models.NotificationsResponse{Items: items, Unread: unread})
}

// MarkRead godoc
// @Summary     Mark a notification read
// @Description Marks a single notification as read
// @Tags        notifications
// @Param       id path int true "Notification ID"
// @Success     204
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /notifications/{id}/read [patch]
func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid notification id", http.StatusBadRequest)
		return
	}

	if err := h.DB.MarkNotificationRead(r.Context(), claims.UserID, id); err != nil {
		if err.Error() == "not found" {
			jsonError(w, "notification not found", http.StatusNotFound)
			return
		}
		jsonError(w, "could not update notification", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead godoc
// @Summary     Mark all notifications read
// @Description Marks every unread notification for the user as read
// @Tags        notifications
// @Success     204
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /notifications/read-all [patch]
func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	if err := h.DB.MarkAllNotificationsRead(r.Context(), claims.UserID); err != nil {
		jsonError(w, "could not update notifications", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
