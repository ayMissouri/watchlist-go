package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/ayMissouri/watchlist-go.git/internal/auth"
	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
	"github.com/ayMissouri/watchlist-go.git/internal/models"
	"github.com/ayMissouri/watchlist-go.git/internal/tracking"
)

type AuthHandler struct {
	DB      *db.DB
	Tracker *tracking.Service
}

// Login godoc
// @Summary     Discord OAuth2 login
// @Description Redirects the user to Discord's OAuth2 login screen
// @Tags        auth
// @Success     307
// @Router      /auth/login [get]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Login redirects the user to discords auth consent screen.
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "production",
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, auth.OAuthConfig.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// Callback godoc
// @Summary     OAuth2 callback
// @Description Handles the Discord redirect, issues a JWT, and redirects to the frontend
// @Tags        auth
// @Param       code  query string true "OAuth2 code from Discord"
// @Param       state query string true "CSRF state token"
// @Success     307
// @Failure     400 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /auth/callback [get]
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Callback handles the redirect back from discord after the user approves.
	// Validate the cookie matches what discord sent back
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, `{"error":"invalid oauth state"}`, http.StatusBadRequest)
		return
	}

	// Clear the cookie (they are single use)
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, `{"error":"missing code"}`, http.StatusBadRequest)
		return
	}

	// Exchange the code for a discord user profile
	discordUser, err := auth.FetchDiscordUser(r.Context(), code)
	if err != nil {
		http.Error(w, `{"error":"discord auth failed"}`, http.StatusInternalServerError)
		return
	}

	// Save or update the user in the DB
	user := &models.User{
		ID:       discordUser.ID,
		Username: discordUser.Username,
		Avatar:   discordUser.Avatar,
	}
	if err := h.DB.UpsertUser(r.Context(), user); err != nil {
		http.Error(w, `{"error":"could not save user"}`, http.StatusInternalServerError)
		return
	}

	// Issue a JWT and hand the user back to the frontend with it
	token, err := auth.IssueJWT(user.ID, user.Username)
	if err != nil {
		http.Error(w, `{"error":"could not issue token"}`, http.StatusInternalServerError)
		return
	}

	// Record the login for session/device stats
	if h.Tracker != nil {
		h.Tracker.Record(r.Context(), models.UserEvent{
			UserID:    user.ID,
			EventType: models.EventLogin,
			Metadata: map[string]any{
				"ip":         clientIP(r),
				"user_agent": r.UserAgent(),
			},
		})
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	redirectURL := frontendURL + "/auth/callback?token=" + url.QueryEscape(token)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// Me godoc
// @Summary     Get current user
// @Description Returns the authenticated users profile
// @Tags        auth
// @Produce     json
// @Success     200 {object} models.User
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    BearerAuth
// @Router      /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Me returns the current authenticated users profile
	claims := middleware.ClaimsFromCtx(r)

	user, err := h.DB.GetUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	user.Avatar = auth.AvatarURL(user.ID, user.Avatar)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}
}

const maxDisplayNameLen = 50

// UpdateMe godoc
// @Summary     Update current user
// @Description Updates the authenticated user's profile (display name). An empty display name clears it, falling back to the username.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body models.UpdateUserRequest true "Profile fields to update"
// @Success     200 {object} models.User
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Security    BearerAuth
// @Router      /auth/me [patch]
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r)

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.DisplayName)
	if len([]rune(name)) > maxDisplayNameLen {
		jsonError(w, "display name too long", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateDisplayName(r.Context(), claims.UserID, name); err != nil {
		jsonError(w, "could not update profile", http.StatusInternalServerError)
		return
	}

	user, err := h.DB.GetUser(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	user.Avatar = auth.AvatarURL(user.ID, user.Avatar)

	jsonOK(w, user)
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
