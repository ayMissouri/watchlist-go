package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

const minWrappedYear = 2025

type StatsHandler struct {
	Tracker *tracking.Service
}

// Profile godoc
// @Summary     Profile stats
// @Description Returns the authenticated user's all-time activity summary: totals, watch time, top genres and top titles.
// @Tags        stats
// @Produce     json
// @Success     200 {object} models.ProfileStats
// @Failure     401 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Security    BearerAuth
// @Router      /stats [get]
func (h *StatsHandler) Profile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	var stats models.ProfileStats
	stats, err := h.Tracker.Profile(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "could not compute stats", http.StatusInternalServerError)
		return
	}

	jsonOK(w, stats)
}

// Wrapped godoc
// @Summary     Year in review
// @Description Returns the authenticated user's "wrapped" stats for a year: watch time, counts, top genres/titles/decades, monthly/hourly/weekday histograms, streaks and night-owl flag. Defaults to the current year.
// @Tags        stats
// @Produce     json
// @Param       year query int false "Calendar year (UTC)" default(2026)
// @Success     200 {object} models.WrappedStats
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Security    BearerAuth
// @Router      /stats/wrapped [get]
func (h *StatsHandler) Wrapped(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	year := time.Now().UTC().Year()
	if y := r.URL.Query().Get("year"); y != "" {
		v, err := strconv.Atoi(y)
		if err != nil || v < minWrappedYear || v > year {
			jsonError(w, "invalid year", http.StatusBadRequest)
			return
		}
		year = v
	}

	var stats models.WrappedStats
	stats, err := h.Tracker.Wrapped(r.Context(), claims.UserID, year)
	if err != nil {
		jsonError(w, "could not compute wrapped", http.StatusInternalServerError)
		return
	}

	jsonOK(w, stats)
}
