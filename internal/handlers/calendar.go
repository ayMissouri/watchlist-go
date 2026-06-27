package handlers

import (
	"log"
	"net/http"

	"github.com/ayMissouri/watchlist-go.git/internal/calendar"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

type CalendarHandler struct {
	DB       *db.DB
	Calendar *calendar.Service
}

// GetAll godoc
// @Summary     Get calendar
// @Description Returns the user's upcoming releases (unreleased movies and not-yet-aired episodes of watchlisted shows), soonest first.
// @Tags        calendar
// @Produce     json
// @Success     200 {object} models.CalendarResponse
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /calendar [get]
func (h *CalendarHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	if err := h.Calendar.ProcessReleasedForUser(r.Context(), claims.UserID); err != nil {
		log.Printf("calendar: process released for %s: %v", claims.UserID, err)
	}

	items, err := h.DB.GetCalendar(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "could not fetch calendar", http.StatusInternalServerError)
		return
	}

	for i := range items {
		items[i].Link = detailLink(items[i].MediaType, items[i].ImdbID)
	}

	jsonOK(w, models.CalendarResponse{Items: items})
}

// Refresh godoc
// @Summary     Refresh calendar
// @Description Re-syncs the user's calendar from their watchlist against the meta service, then returns the updated list. Useful right after adding an upcoming title.
// @Tags        calendar
// @Produce     json
// @Success     200 {object} models.CalendarResponse
// @Failure     401 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Security    BearerAuth
// @Router      /calendar/refresh [post]
func (h *CalendarHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	if err := h.Calendar.SyncUser(r.Context(), claims.UserID); err != nil {
		jsonError(w, "could not refresh calendar", http.StatusInternalServerError)
		return
	}

	items, err := h.DB.GetCalendar(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "could not fetch calendar", http.StatusInternalServerError)
		return
	}

	for i := range items {
		items[i].Link = detailLink(items[i].MediaType, items[i].ImdbID)
	}

	jsonOK(w, models.CalendarResponse{Items: items})
}
