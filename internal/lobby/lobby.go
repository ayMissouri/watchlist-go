package lobby

import (
	"cmp"
	"crypto/rand"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

const (
	codeLen = 6
	seekTolerance = 2.0
	idleTTL = 10 * time.Minute
)

type Sub struct {
	member models.LobbyMember
	Wake   chan struct{}
}

type room struct {
	state models.Lobby
	subs  map[*Sub]struct{}
}

type Manager struct {
	mu    sync.Mutex
	rooms map[string]*room
}

func NewManager() *Manager {
	return &Manager{rooms: map[string]*room{}}
}

func (m *Manager) Create(host models.LobbyMember, req models.UpdateLobbyRequest) models.Lobby {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UnixMilli()
	maps.DeleteFunc(m.rooms, func(_ string, r *room) bool {
		return len(r.subs) == 0 && now-r.state.UpdatedAt > idleTTL.Milliseconds()
	})

	code := newCode()
	for m.rooms[code] != nil {
		code = newCode()
	}
	r := &room{state: models.Lobby{Code: code, Host: host.ID, UpdatedAt: now}, subs: map[*Sub]struct{}{}}
	apply(&r.state, req, now)
	m.rooms[code] = r
	return snapshot(r)
}

func (m *Manager) Join(code string, member models.LobbyMember) (*Sub, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.rooms[code]
	if r == nil {
		return nil, false
	}
	s := &Sub{member: member, Wake: make(chan struct{}, 1)}
	r.subs[s] = struct{}{}
	r.wakeAll()
	return s, true
}

func (m *Manager) Leave(code string, s *Sub) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.rooms[code]
	if r == nil {
		return
	}
	delete(r.subs, s)
	if len(r.subs) == 0 {
		delete(m.rooms, code)
		return
	}
	r.wakeAll()
}

func (m *Manager) Snapshot(code string) (models.Lobby, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.rooms[code]
	if r == nil {
		return models.Lobby{}, false
	}
	return snapshot(r), true
}

func (m *Manager) Update(code string, by models.LobbyMember, req models.UpdateLobbyRequest) (models.Lobby, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.rooms[code]
	if r == nil {
		return models.Lobby{}, false
	}
	if action := apply(&r.state, req, time.Now().UnixMilli()); action != "" {
		r.state.LastAction = models.LobbyAction{Type: action, By: by, At: r.state.UpdatedAt}
		r.wakeAll()
	}
	return snapshot(r), true
}

func apply(s *models.Lobby, req models.UpdateLobbyRequest, now int64) string {
	cur := expected(s, now)
	pos := cur
	if req.Position != nil {
		pos = *req.Position
	}

	var action string
	switch {
	case req.ItemID != "" && (req.ItemID != s.ItemID || req.Season != s.Season || req.Episode != s.Episode):
		s.ItemID, s.Season, s.Episode = req.ItemID, req.Season, req.Episode
		s.Playing = req.Playing != nil && *req.Playing
		if req.Position == nil {
			pos = 0
		}
		action = "change"
	case req.Playing != nil && *req.Playing != s.Playing:
		s.Playing = *req.Playing
		action = "pause"
		if s.Playing {
			action = "play"
		}
	case req.Position != nil && math.Abs(*req.Position-cur) > seekTolerance:
		action = "seek"
	default:
		return ""
	}
	s.Position, s.UpdatedAt = pos, now
	return action
}

func expected(s *models.Lobby, now int64) float64 {
	if !s.Playing {
		return s.Position
	}
	return s.Position + float64(now-s.UpdatedAt)/1000
}

func snapshot(r *room) models.Lobby {
	s := r.state
	s.Members = make([]models.LobbyMember, 0, len(r.subs))
	seen := map[string]bool{}
	for sub := range r.subs {
		if !seen[sub.member.ID] {
			seen[sub.member.ID] = true
			s.Members = append(s.Members, sub.member)
		}
	}
	slices.SortFunc(s.Members, func(a, b models.LobbyMember) int {
		return cmp.Compare(a.Username, b.Username)
	})
	return s
}

func (r *room) wakeAll() {
	for s := range r.subs {
		select {
		case s.Wake <- struct{}{}:
		default:
		}
	}
}

func newCode() string {
	return rand.Text()[:codeLen]
}
