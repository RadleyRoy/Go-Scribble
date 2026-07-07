package tests

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"go-scribble/game"
)

// newGameServer starts a websocket test server for the given room manager and
// returns the base ws:// URL (append a room code) plus a shutdown func.
func newGameServer(t *testing.T, m *game.RoomManager) (baseWS string, closeFn func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWS(m, w, r)
	})
	ts := httptest.NewServer(mux)
	return "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?room=", ts.Close
}

func joinPlayer(t *testing.T, url, name string) *websocket.Conn {
	t.Helper()
	c := dial(t, url)
	writeJSON(t, c, map[string]any{"type": "join", "name": name})
	return c
}

// TestDrawerLeavingAwardsNoBonus pins the fix for a scoring bug: when the
// drawer disconnects mid-turn, the drawer bonus must vanish with them, not be
// paid out to whichever player happened to precede them in join order.
func TestDrawerLeavingAwardsNoBonus(t *testing.T) {
	cfg := game.Config{Topic: "animals", MaxRounds: 2, TurnSeconds: 8, ChooseSeconds: 8, RevealSeconds: 1, MinPlayers: 2}
	m := game.NewRoomManager(game.NewLocalWordProvider(), cfg)
	base, closeFn := newGameServer(t, m)
	defer closeFn()
	url := base + m.Create()

	alice := joinPlayer(t, url, "Alice")
	defer alice.Close()
	bob := joinPlayer(t, url, "Bob")
	defer bob.Close()
	cara := joinPlayer(t, url, "Cara")
	defer cara.Close()

	// Turn 1: Alice draws; Bob and Cara both guess, ending the turn. Alice
	// legitimately earns 2 guessers x 30 = 60 drawer points.
	w1 := readUntil(t, alice, func(f frame) bool { return f.Type == "choices" }).Choices[0]
	writeJSON(t, alice, map[string]any{"type": "pick", "content": w1})
	readUntil(t, bob, func(f frame) bool { return f.Type == "state" && f.Phase == "drawing" })
	writeJSON(t, bob, map[string]any{"type": "chat", "content": w1})
	readUntil(t, bob, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })
	writeJSON(t, cara, map[string]any{"type": "chat", "content": w1})
	readUntil(t, cara, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })

	// Turn 2: Bob draws; Cara guesses correctly; then Bob disconnects mid-turn.
	w2 := readUntil(t, bob, func(f frame) bool { return f.Type == "choices" }).Choices[0]
	writeJSON(t, bob, map[string]any{"type": "pick", "content": w2})
	readUntil(t, cara, func(f frame) bool {
		return f.Type == "state" && f.Phase == "drawing" && f.DrawerName == "Bob"
	})
	writeJSON(t, cara, map[string]any{"type": "chat", "content": w2})
	readUntil(t, cara, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })
	bob.Close()

	// The turn ends in a reveal for the two remaining players. Alice must
	// still have exactly her 60 points: the departed drawer's bonus must not
	// have landed on her.
	st := readUntil(t, alice, func(f frame) bool {
		return f.Type == "state" && f.Phase == "reveal" && len(f.Players) == 2
	})
	found := false
	for _, p := range st.Players {
		if p.Name == "Alice" {
			found = true
			if p.Score != 60 {
				t.Fatalf("Alice's score changed when the drawer left: got %d, want 60", p.Score)
			}
		}
	}
	if !found {
		t.Fatal("Alice missing from the reveal state")
	}
}

// TestGuessedPlayerLeavingDoesNotEndTurn pins the fix for a premature turn
// end: when a player who had already guessed disconnects, the guessed count
// must shrink with them so the remaining players keep their chance to guess.
func TestGuessedPlayerLeavingDoesNotEndTurn(t *testing.T) {
	cfg := game.Config{Topic: "animals", MaxRounds: 1, TurnSeconds: 8, ChooseSeconds: 8, RevealSeconds: 1, MinPlayers: 2}
	m := game.NewRoomManager(game.NewLocalWordProvider(), cfg)
	base, closeFn := newGameServer(t, m)
	defer closeFn()
	url := base + m.Create()

	drawer := joinPlayer(t, url, "Drawer")
	defer drawer.Close()
	ben := joinPlayer(t, url, "Ben")
	defer ben.Close()
	cleo := joinPlayer(t, url, "Cleo")
	defer cleo.Close()
	dana := joinPlayer(t, url, "Dana")
	defer dana.Close()

	w := readUntil(t, drawer, func(f frame) bool { return f.Type == "choices" }).Choices[0]
	writeJSON(t, drawer, map[string]any{"type": "pick", "content": w})

	// Ben and Cleo guess correctly, then Cleo (already guessed) disconnects.
	readUntil(t, ben, func(f frame) bool { return f.Type == "state" && f.Phase == "drawing" })
	writeJSON(t, ben, map[string]any{"type": "chat", "content": w})
	readUntil(t, ben, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })
	writeJSON(t, cleo, map[string]any{"type": "chat", "content": w})
	readUntil(t, cleo, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })
	cleo.Close()
	time.Sleep(200 * time.Millisecond) // let the server process the disconnect

	// The turn must still be live for Dana: her guess is accepted as correct,
	// not echoed back as ordinary chat after a premature reveal.
	writeJSON(t, dana, map[string]any{"type": "chat", "content": w})
	readUntil(t, dana, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })
}

// TestInsiderChatIsHiddenFromActiveGuessers pins the chat routing: once a
// player has guessed the word (or is drawing), their chat goes only to others
// who already know the word, so hints can't leak to active guessers.
func TestInsiderChatIsHiddenFromActiveGuessers(t *testing.T) {
	cfg := game.Config{Topic: "animals", MaxRounds: 1, TurnSeconds: 8, ChooseSeconds: 8, RevealSeconds: 1, MinPlayers: 2}
	m := game.NewRoomManager(game.NewLocalWordProvider(), cfg)
	base, closeFn := newGameServer(t, m)
	defer closeFn()
	url := base + m.Create()

	art := joinPlayer(t, url, "Art") // drawer
	defer art.Close()
	beth := joinPlayer(t, url, "Beth") // guesses correctly, then hints
	defer beth.Close()
	cam := joinPlayer(t, url, "Cam") // still guessing; must not see the hint
	defer cam.Close()

	w := readUntil(t, art, func(f frame) bool { return f.Type == "choices" }).Choices[0]
	writeJSON(t, art, map[string]any{"type": "pick", "content": w})
	readUntil(t, beth, func(f frame) bool { return f.Type == "state" && f.Phase == "drawing" })
	writeJSON(t, beth, map[string]any{"type": "chat", "content": w})
	readUntil(t, beth, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })

	const hint = "psst the drawing is upside down"
	writeJSON(t, beth, map[string]any{"type": "chat", "content": hint})

	// The drawer sees Beth's message on the quiet channel...
	readUntil(t, art, func(f frame) bool {
		return f.Type == "chat" && f.Kind == "quiet" && f.Content == hint
	})
	// ...but Cam, still guessing, must never receive it.
	assertNeverReceives(t, cam, 700*time.Millisecond, func(f frame) bool {
		return f.Type == "chat" && f.Content == hint
	})
}

// assertNeverReceives reads frames for the given window and fails the test if
// any of them satisfies pred; a read timeout at the end of the window is the
// success path.
func assertNeverReceives(t *testing.T, c *websocket.Conn, window time.Duration, pred func(frame) bool) {
	t.Helper()
	deadline := time.Now().Add(window)
	_ = c.SetReadDeadline(deadline)
	for {
		var f frame
		if err := c.ReadJSON(&f); err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return // window elapsed without a match — success
			}
			t.Fatalf("read: %v", err)
		}
		if pred(f) {
			t.Fatal("received a message that should have been routed away")
		}
	}
}
