package game

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 16384
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allows any origin for easy local development; restrict in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is a single WebSocket connection. Its send channel carries the various
// outbound payload types (Message, ChatMessage, StateMessage) and is written to
// and closed only by the engine goroutine.
type Client struct {
	engine *Engine
	conn   *websocket.Conn
	send   chan interface{}
}

// trySend queues a payload without blocking. Only the engine goroutine calls it.
func (c *Client) trySend(v interface{}) bool {
	select {
	case c.send <- v:
		return true
	default:
		return false
	}
}

// readPump reads client frames and forwards them to the engine. Every channel
// send selects against engine.stopped: the engine may stop while this client
// is still attached (room teardown racing a late registration), and a plain
// send would then block this goroutine forever.
func (c *Client) readPump() {
	defer func() {
		select {
		case c.engine.unregister <- c:
		case <-c.engine.stopped:
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)
			}
			return
		}
		select {
		case c.engine.incoming <- inbound{client: c, msg: msg}:
		case <-c.engine.stopped:
			return
		}
	}
}

// writePump writes queued payloads to the connection and pings periodically.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case v, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(v); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWS upgrades an HTTP request to a WebSocket connection and registers the
// client with the room named by the "room" query parameter.
func ServeWS(rooms *RoomManager, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket upgrade error:", err)
		return
	}

	engine, ok := rooms.Get(r.URL.Query().Get("room"))
	if !ok {
		_ = conn.WriteJSON(ErrorMessage{Type: MsgError, Content: "Room not found"})
		conn.Close()
		return
	}

	client := &Client{
		engine: engine,
		conn:   conn,
		send:   make(chan interface{}, sendBufferSize),
	}

	// Guard against the room being torn down between lookup and registration.
	select {
	case engine.register <- client:
	case <-engine.stopped:
		_ = conn.WriteJSON(ErrorMessage{Type: MsgError, Content: "Room closed"})
		conn.Close()
		return
	}

	go client.writePump()
	go client.readPump()
}
