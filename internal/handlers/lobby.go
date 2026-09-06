package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/lobby"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	maxLobbyItemIDLen = 64
	lobbyPingInterval = 20 * time.Second
)

type LobbyHandler struct {
	DB      *db.DB
	Lobbies *lobby.Manager
}

// Create godoc
// @Summary     Create a watch-together lobby
// @Description Opens a lobby with a shareable 6-character code. The body optionally seeds the video and playback state.
// @Tags        lobbies
// @Accept      json
// @Produce     json
// @Param       body body models.UpdateLobbyRequest false "Initial state"
// @Success     200 {object} models.Lobby
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Security    BearerAuth
// @Router      /lobbies [post]
func (h *LobbyHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLobbyRequest(w, r)
	if !ok {
		return
	}
	jsonOK(w, h.Lobbies.Create(h.member(r), req))
}

// Update godoc
// @Summary     Play, pause, seek, or switch video
// @Description Every member has the remote. Send only what changed such as `playing`, `position` (seconds), or `item_id` with `season`/`episode` to switch video.
// @Tags        lobbies
// @Accept      json
// @Produce     json
// @Param       code path string true "Lobby code"
// @Param       body body models.UpdateLobbyRequest true "What changed"
// @Success     200 {object} models.Lobby
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /lobbies/{code}/state [post]
func (h *LobbyHandler) Update(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLobbyRequest(w, r)
	if !ok {
		return
	}
	snap, ok := h.Lobbies.Update(chi.URLParam(r, "code"), h.member(r), req)
	if !ok {
		jsonError(w, "lobby not found", http.StatusNotFound)
		return
	}
	jsonOK(w, snap)
}

// Stream godoc
// @Summary     Join a lobby and follow its state
// @Description Server-Sent Events. You are a member for as long as this connection is open.
// @Tags        lobbies
// @Produce     text/event-stream
// @Param       code path string true "Lobby code"
// @Success     200 {object} models.Lobby "One `state` event per change"
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /lobbies/{code}/stream [get]
func (h *LobbyHandler) Stream(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	sub, ok := h.Lobbies.Join(code, h.member(r))
	if !ok {
		jsonError(w, "lobby not found", http.StatusNotFound)
		return
	}
	defer h.Lobbies.Leave(code, sub)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	rc := http.NewResponseController(w)
	ping := time.Tick(lobbyPingInterval)

	for {
		var msg string
		select {
		case <-r.Context().Done():
			return
		case <-ping:
			msg = ": ping\n\n"
		case <-sub.Wake:
			snap, ok := h.Lobbies.Snapshot(code)
			if !ok {
				return
			}
			data, err := json.Marshal(snap)
			if err != nil {
				return
			}
			msg = "event: state\ndata: " + string(data) + "\n\n"
		}
		if _, err := io.WriteString(w, msg); err != nil {
			return
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}

func (h *LobbyHandler) member(r *http.Request) models.LobbyMember {
	claims := middleware.ClaimsFromCtx(r)
	m := models.LobbyMember{ID: claims.UserID, Username: claims.Username}
	if h.DB != nil && h.DB.Pool != nil {
		if u, err := h.DB.GetUser(r.Context(), claims.UserID); err == nil {
			m.DisplayName, m.Avatar = u.DisplayName, u.Avatar
		}
	}
	return m
}

func decodeLobbyRequest(w http.ResponseWriter, r *http.Request) (models.UpdateLobbyRequest, bool) {
	var req models.UpdateLobbyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return req, false
	}
	if len(req.ItemID) > maxLobbyItemIDLen || (req.Position != nil && *req.Position < 0) {
		jsonError(w, "invalid item_id or position", http.StatusBadRequest)
		return req, false
	}
	return req, true
}
