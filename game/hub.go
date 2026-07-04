package game

// Hub is the single source of truth for connected clients and the current
// round state. Every field is read and written only by the goroutine running
// Run, so the state needs no locking: ownership, not mutexes, guarantees
// safety. Other goroutines interact with the hub exclusively through its
// channels (Register/Unregister/Broadcast).
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message

	// Round state replayed to clients that join mid-round.
	history []Message // strokes drawn since the last clear
	word    *Message  // the latest word announcement, if any
	timer   *Message  // the latest countdown tick, if any
}

// NewHub creates a hub ready to be started with Run.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message),
	}
}

// Broadcast queues a message for delivery to every connected client. It is
// safe to call from any goroutine.
func (h *Hub) Broadcast(msg Message) {
	h.broadcast <- msg
}

// Run processes registrations, disconnections and broadcasts on a single
// goroutine until the process exits.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.sendSnapshot(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.handleBroadcast(msg)
		}
	}
}

// handleBroadcast updates the retained round state and fans the message out to
// every client, dropping any client whose send buffer is full (a client too
// slow to keep up is disconnected rather than allowed to block the hub).
func (h *Hub) handleBroadcast(msg Message) {
	switch msg.Type {
	case MessageDraw:
		h.history = append(h.history, msg)
	case MessageClear:
		h.history = h.history[:0]
	case MessageWord:
		// A new round starts with a clean canvas.
		h.history = h.history[:0]
		w := msg
		h.word = &w
	case MessageTimer:
		t := msg
		h.timer = &t
	}

	for client := range h.clients {
		if !client.trySend(msg) {
			h.removeClient(client)
		}
	}
}

// sendSnapshot brings a freshly connected client up to date with the current
// round: the word, the last timer tick and the strokes drawn so far. The
// strokes are sent as one MessageHistory so the hub never streams potentially
// thousands of segments through the client's bounded send buffer (which was
// the original deadlock).
func (h *Hub) sendSnapshot(client *Client) {
	ok := true
	if ok && h.word != nil {
		ok = client.trySend(*h.word)
	}
	if ok && h.timer != nil {
		ok = client.trySend(*h.timer)
	}
	if ok && len(h.history) > 0 {
		replay := make([]Message, len(h.history))
		copy(replay, h.history)
		ok = client.trySend(Message{Type: MessageHistory, History: replay})
	}
	if !ok {
		h.removeClient(client)
	}
}

// removeClient unregisters a client and closes its send channel exactly once.
// Because it is only ever called from the Run goroutine and guards on map
// membership, a client can never be closed twice.
func (h *Hub) removeClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.closeSend()
	}
}
