package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"go-scribble/game"
)

// frame is a permissive view over any server -> client message, letting one
// struct decode states, chats, choices and simple pushes alike.
type frame struct {
	Type       string   `json:"type"`
	Phase      string   `json:"phase"`
	IsDrawer   bool     `json:"isDrawer"`
	DrawerName string   `json:"drawerName"`
	Word       string   `json:"word"`
	WordMasked bool     `json:"wordMasked"`
	YouID      string   `json:"youId"`
	Kind       string   `json:"kind"`
	Content    string   `json:"content"`
	Choices    []string `json:"choices"`
	Players    []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Score int    `json:"score"`
	} `json:"players"`
}

// TestGameFlow drives a real two-player game inside a private room over
// WebSockets: the game starts, the drawer picks one of three offered words and
// sees it while the guesser sees a mask, and a correct guess is scored.
func TestGameFlow(t *testing.T) {
	cfg := game.Config{Topic: "animals", MaxRounds: 1, TurnSeconds: 5, ChooseSeconds: 5, RevealSeconds: 1, MinPlayers: 2}
	rooms := game.NewRoomManager(game.NewLocalWordProvider(), cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWS(rooms, w, r)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	code := rooms.Create()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?room=" + code

	alice := dial(t, wsURL)
	defer alice.Close()
	writeJSON(t, alice, map[string]any{"type": "join", "name": "Alice"})

	bob := dial(t, wsURL)
	defer bob.Close()
	writeJSON(t, bob, map[string]any{"type": "join", "name": "Bob"})

	// Alice joined first, so she is the drawer and is offered a set of words.
	choices := readUntil(t, alice, func(f frame) bool { return f.Type == "choices" })
	if len(choices.Choices) == 0 {
		t.Fatal("expected the drawer to be offered word choices")
	}
	// She picks the first one.
	word := choices.Choices[0]
	writeJSON(t, alice, map[string]any{"type": "pick", "content": word})

	// Now the round is drawing: Alice sees the real word.
	aliceState := readUntil(t, alice, func(f frame) bool { return f.Type == "state" && f.Phase == "drawing" })
	if !aliceState.IsDrawer {
		t.Fatal("expected first joiner (Alice) to be the drawer")
	}
	if aliceState.Word != word || aliceState.WordMasked {
		t.Fatalf("drawer should see the chosen word %q; got %q masked=%v", word, aliceState.Word, aliceState.WordMasked)
	}

	// Bob is a guesser and must only see a mask.
	bobState := readUntil(t, bob, func(f frame) bool { return f.Type == "state" && f.Phase == "drawing" })
	if bobState.IsDrawer {
		t.Fatal("Bob should not be the drawer")
	}
	if !bobState.WordMasked {
		t.Fatal("guesser should see a masked word")
	}

	// Bob guesses correctly.
	writeJSON(t, bob, map[string]any{"type": "chat", "content": word})

	// He gets a private confirmation...
	readUntil(t, bob, func(f frame) bool { return f.Type == "chat" && f.Kind == "correct" })

	// ...and his score becomes positive.
	readUntil(t, bob, func(f frame) bool {
		if f.Type != "state" {
			return false
		}
		for _, p := range f.Players {
			if p.Name == "Bob" && p.Score > 0 {
				return true
			}
		}
		return false
	})
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func writeJSON(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	if err := c.WriteJSON(v); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readUntil reads frames until pred is satisfied or the budget expires.
func readUntil(t *testing.T, c *websocket.Conn, pred func(frame) bool) frame {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var f frame
		if err := c.ReadJSON(&f); err != nil {
			t.Fatalf("read: %v", err)
		}
		if pred(f) {
			return f
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for expected frame")
		}
	}
}
