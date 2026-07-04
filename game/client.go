package game

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// pongWait is how long we wait for a pong before treating the peer as gone.
	pongWait = 60 * time.Second
	// pingPeriod is how often we ping the peer. It must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize caps the size of an inbound message to protect the server.
	maxMessageSize = 8192
	// sendBufferSize is how many outbound messages may queue before a client is
	// considered too slow and disconnected.
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin allows connections from any origin. This is convenient for
	// local development; a production deployment should restrict it.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is a single WebSocket connection. Its send channel is written to and
// closed only by the Hub goroutine; readPump and writePump own the connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan Message
}

// trySend queues a message without blocking. It returns false if the client's
// buffer is full, signalling to the hub that the client should be dropped.
// Only the hub goroutine calls this.
func (c *Client) trySend(msg Message) bool {
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

// closeSend closes the outbound channel, ending writePump. Only the hub
// goroutine calls this, and only once per client.
func (c *Client) closeSend() {
	close(c.send)
}

// readPump reads messages from the connection and forwards the client-originated
// ones to the hub. It runs until the connection errors or closes.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
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

		// Only relay the message types a client is allowed to originate; the
		// server is the sole author of word/timer/history messages.
		switch msg.Type {
		case MessageDraw, MessageClear, MessageChat:
			c.hub.Broadcast(msg)
		}
	}
}

// writePump writes queued messages to the connection and sends periodic pings
// to keep the connection alive and detect dead peers.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel: tell the peer and stop.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
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

// ServeWS upgrades an HTTP request to a WebSocket connection, registers the new
// client with the hub, and starts its read and write pumps.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket upgrade error:", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan Message, sendBufferSize),
	}
	hub.register <- client

	go client.writePump()
	go client.readPump()
}
