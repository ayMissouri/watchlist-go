package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/db"
	"github.com/ayMissouri/watchlist-go.git/internal/gemini"
	"github.com/ayMissouri/watchlist-go.git/internal/middleware"
)

const (
	// Each AMI question is two calls: reading the request, then answering it.
	amiCallLimit  = 40
	amiCallWindow = 10 * time.Minute
	amiMaxBody    = 64 << 10
	amiMaxTokens  = 4096
	// Room for a few models to be tried in turn.
	amiTimeout = 60 * time.Second
)

// AMIModel answers AMI's steps: a *gemini.Client or a *claude.Client, both
// failing with *gemini.Error.
type AMIModel interface {
	Configured() bool
	Generate(ctx context.Context, req gemini.Request) (json.RawMessage, error)
}

// AMIHandler runs AMI's model calls for the apps, so the API key stays on the
// server. AMI is for accounts with watch access, like playback.
type AMIHandler struct {
	Model AMIModel
	limit  *windowLimiter
	// hasAccess reports whether the account may watch (users.has_access).
	hasAccess func(ctx context.Context, userID string) (bool, error)
}

func NewAMIHandler(database *db.DB, model AMIModel) *AMIHandler {
	return &AMIHandler{
		Model: model,
		limit:  newWindowLimiter(amiCallLimit, amiCallWindow),
		hasAccess: func(ctx context.Context, userID string) (bool, error) {
			user, err := database.GetUser(ctx, userID)
			if err != nil {
				return false, err
			}
			return user.HasAccess, nil
		},
	}
}

// Generate godoc
// @Summary     Ask AMI's model
// @Description One step of an AMI (aWatch Media Intelligence) question: the model (Claude Haiku 5.5 when ANTHROPIC_API_KEY is set, otherwise Gemini) reads `prompt` under `instructions` and replies with JSON that follows `schema`, a JSON Schema object. The apps make two calls per question, one to read the request and one to answer it. On Gemini, when a model is busy or out of its free quota the next one answers.
// @Description Only for accounts with watch access, and limited to 40 calls per 10 minutes per account. Errors: `no_access` (403, the account can't watch), `slow_down` (429, that limit), `crowded` (429, the quota frees up within a minute or so), `exhausted` (429, it doesn't), `blocked` (422, the model wouldn't answer), `busy` (503), `unavailable` (503, no working key).
// @Tags        ami
// @Accept      json
// @Produce     json
// @Param       body body gemini.Request true "The step"
// @Success     200 {object} map[string]any "The model's reply, shaped by the schema"
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Failure     403 {object} map[string]string
// @Failure     422 {object} map[string]string
// @Failure     429 {object} map[string]string
// @Failure     503 {object} map[string]string
// @Security    BearerAuth
// @Router      /ami [post]
func (h *AMIHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.Model.Configured() {
		jsonError(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var req gemini.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, amiMaxBody)).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Instructions) == "" || strings.TrimSpace(req.Prompt) == "" ||
		!bytes.HasPrefix(bytes.TrimSpace(req.Schema), []byte("{")) {
		jsonError(w, "instructions, prompt and a schema object are required", http.StatusBadRequest)
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 2048
	}
	req.MaxTokens = min(req.MaxTokens, amiMaxTokens)

	userID := middleware.ClaimsFromCtx(r).UserID
	switch ok, err := h.hasAccess(r.Context(), userID); {
	case err != nil:
		log.Printf("ami: could not check access for %s: %v", userID, err)
		jsonError(w, "busy", http.StatusServiceUnavailable)
		return
	case !ok:
		jsonError(w, "no_access", http.StatusForbidden)
		return
	}
	if !h.limit.allow(userID, time.Now()) {
		jsonError(w, "slow_down", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), amiTimeout)
	defer cancel()
	reply, err := h.Model.Generate(ctx, req)
	if err != nil {
		code, status := amiError(err)
		jsonError(w, code, status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(reply)
}

// amiError is the code the apps turn into a message.
func amiError(err error) (string, int) {
	var e *gemini.Error
	if !errors.As(err, &e) {
		// Out of time trying models.
		return "busy", http.StatusServiceUnavailable
	}
	switch e.Kind {
	case gemini.Quota:
		if e.RetryAfter > 0 && e.RetryAfter <= 90*time.Second {
			return "crowded", http.StatusTooManyRequests
		}
		return "exhausted", http.StatusTooManyRequests
	case gemini.Blocked:
		return "blocked", http.StatusUnprocessableEntity
	case gemini.Unauthorized:
		log.Print("ami: the model turned its API key down")
		return "unavailable", http.StatusServiceUnavailable
	default:
		return "busy", http.StatusServiceUnavailable
	}
}

// windowLimiter allows `limit` calls per key per window.
// ponytail: fixed windows in memory, so counts reset on deploy and aren't shared between instances; move them to Postgres if the API runs more than one.
type windowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*hitWindow
}

type hitWindow struct {
	start time.Time
	n     int
}

func newWindowLimiter(limit int, window time.Duration) *windowLimiter {
	return &windowLimiter{limit: limit, window: window, hits: map[string]*hitWindow{}}
}

// allow counts a call against key, and says no once the window's used up.
func (l *windowLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	hw := l.hits[key]
	if hw == nil || now.Sub(hw.start) >= l.window {
		// Forget finished windows now and then, so the map doesn't grow forever.
		if len(l.hits) > 10_000 {
			for k, v := range l.hits {
				if now.Sub(v.start) >= l.window {
					delete(l.hits, k)
				}
			}
		}
		hw = &hitWindow{start: now}
		l.hits[key] = hw
	}
	if hw.n >= l.limit {
		return false
	}
	hw.n++
	return true
}
