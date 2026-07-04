package game

import (
	"testing"
	"time"
)

// TestHubReplaysHistoryToLateJoiner reproduces the original deadlock: when the
// number of retained strokes exceeded a client's send buffer, replaying them
// one by one blocked the whole hub. The fix batches them into a single
// MessageHistory, so a late joiner must receive all strokes without the hub
// stalling.
func TestHubReplaysHistoryToLateJoiner(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// A first client with plenty of buffer so it is never dropped.
	first := &Client{send: make(chan Message, 4096)}
	hub.register <- first

	const strokes = 500 // deliberately larger than sendBufferSize (256)
	for i := 0; i < strokes; i++ {
		hub.Broadcast(Message{Type: MessageDraw, X: float64(i)})
	}

	// A client that joins mid-round.
	late := &Client{send: make(chan Message, 8)}
	hub.register <- late

	select {
	case msg := <-late.send:
		if msg.Type != MessageHistory {
			t.Fatalf("expected first message to be history, got %q", msg.Type)
		}
		if len(msg.History) != strokes {
			t.Fatalf("expected %d strokes replayed, got %d", strokes, len(msg.History))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for history (possible hub deadlock)")
	}
}

// TestHubClearResetsHistory verifies a cleared canvas is not replayed to a
// client that joins afterwards.
func TestHubClearResetsHistory(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	seed := &Client{send: make(chan Message, 16)}
	hub.register <- seed

	hub.Broadcast(Message{Type: MessageDraw, X: 1})
	hub.Broadcast(Message{Type: MessageClear})

	late := &Client{send: make(chan Message, 8)}
	hub.register <- late

	// With no retained strokes, the snapshot for the late joiner is empty.
	select {
	case msg := <-late.send:
		t.Fatalf("expected no snapshot after clear, got a %q message", msg.Type)
	case <-time.After(200 * time.Millisecond):
		// success: nothing was replayed
	}
}
