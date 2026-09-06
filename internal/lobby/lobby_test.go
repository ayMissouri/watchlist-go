package lobby

import (
	"testing"

	"github.com/ayMissouri/watchlist-go.git/internal/models"
)

func TestApplyNamesTheAction(t *testing.T) {
	yes, no := true, false
	f := func(v float64) *float64 { return &v }

	s := models.Lobby{ItemID: "tt1", Playing: true, Position: 100}
	if got := apply(&s, models.UpdateLobbyRequest{Position: f(105.5)}, 5000); got != "" || s.Position != 100 {
		t.Fatalf("drift: action %q, position %v", got, s.Position)
	}
	if got := apply(&s, models.UpdateLobbyRequest{Position: f(130)}, 5000); got != "seek" || s.Position != 130 || s.UpdatedAt != 5000 {
		t.Fatalf("seek: action %q, state %+v", got, s)
	}
	if got := apply(&s, models.UpdateLobbyRequest{Playing: &no}, 7000); got != "pause" || s.Playing || s.Position != 132 {
		t.Fatalf("pause: action %q, state %+v", got, s)
	}
	if got := apply(&s, models.UpdateLobbyRequest{Playing: &no}, 8000); got != "" || s.UpdatedAt != 7000 {
		t.Fatalf("repeat pause should be a no-op: action %q, state %+v", got, s)
	}
	if got := apply(&s, models.UpdateLobbyRequest{Playing: &yes, Position: f(140)}, 9000); got != "play" || !s.Playing || s.Position != 140 {
		t.Fatalf("play: action %q, state %+v", got, s)
	}
	if got := apply(&s, models.UpdateLobbyRequest{ItemID: "tt1", Season: 1, Episode: 2}, 9500); got != "change" || s.Playing || s.Position != 0 || s.Episode != 2 {
		t.Fatalf("change: action %q, state %+v", got, s)
	}
}

func TestRoomLifecycle(t *testing.T) {
	m := NewManager()
	alice := models.LobbyMember{ID: "1", Username: "alice"}
	bob := models.LobbyMember{ID: "2", Username: "bob"}
	yes := true

	l := m.Create(alice, models.UpdateLobbyRequest{ItemID: "tt1"})
	a, ok := m.Join(l.Code, alice)
	if !ok {
		t.Fatal("join failed")
	}
	b, _ := m.Join(l.Code, bob)
	drain := func() {
		for _, s := range []*Sub{a, b} {
			select {
			case <-s.Wake:
			default:
			}
		}
	}
	drain()

	l, _ = m.Update(l.Code, bob, models.UpdateLobbyRequest{Playing: &yes})
	if l.LastAction.Type != "play" || l.LastAction.By.ID != bob.ID || len(l.Members) != 2 {
		t.Fatalf("play by bob: %+v", l)
	}
	select {
	case <-a.Wake:
	default:
		t.Fatal("alice was not woken")
	}
	drain()

	pos := l.Position
	if l2, _ := m.Update(l.Code, alice, models.UpdateLobbyRequest{Position: &pos}); l2.LastAction != l.LastAction {
		t.Fatalf("echo counted as an action: %+v", l2.LastAction)
	}
	select {
	case <-b.Wake:
		t.Fatal("no-op woke bob")
	default:
	}

	m.Leave(l.Code, a)
	if s, _ := m.Snapshot(l.Code); len(s.Members) != 1 || s.Members[0].ID != bob.ID {
		t.Fatalf("after alice left: %+v", s.Members)
	}
	m.Leave(l.Code, b)
	if _, ok := m.Snapshot(l.Code); ok {
		t.Fatal("empty room should be gone")
	}
}
