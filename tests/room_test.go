package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-scribble/game"
)

func TestRoomManagerCreateAndGet(t *testing.T) {
	m := game.NewRoomManager(game.NewLocalWordProvider(), game.DefaultConfig())

	code := m.Create()
	if code == "" {
		t.Fatal("expected a non-empty room code")
	}
	if _, ok := m.Get(code); !ok {
		t.Fatalf("room %q should exist", code)
	}
	if _, ok := m.Get(strings.ToLower(code)); !ok {
		t.Fatal("room lookup should be case-insensitive")
	}
	if _, ok := m.Get("nope"); ok {
		t.Fatal("unknown room should not be found")
	}
}

func TestRoomManagerCodesAreUnique(t *testing.T) {
	m := game.NewRoomManager(game.NewLocalWordProvider(), game.DefaultConfig())
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		code := m.Create()
		if seen[code] {
			t.Fatalf("duplicate room code %q", code)
		}
		seen[code] = true
	}
}

// TestRoomIsReapedWhenEmpty verifies the room (and its engine goroutine) is torn
// down once its last client disconnects.
func TestRoomIsReapedWhenEmpty(t *testing.T) {
	m := game.NewRoomManager(game.NewLocalWordProvider(), game.DefaultConfig())

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWS(m, w, r)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	code := m.Create()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?room=" + code

	conn := dial(t, wsURL)
	writeJSON(t, conn, map[string]any{"type": "join", "name": "Solo"})
	// Give the server a moment to register, then leave.
	time.Sleep(100 * time.Millisecond)
	conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := m.Get(code); !ok {
			return // reaped — success
		}
		if time.Now().After(deadline) {
			t.Fatal("empty room was not reaped")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
