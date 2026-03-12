package game

import "sync"

type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan DrawMessage
	Register   chan *Client
	Unregister chan *Client
	History    []DrawMessage
	Mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan DrawMessage),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		History:    []DrawMessage{},
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Mu.Lock()
			h.Clients[client] = true
			for _, msg := range h.History {
				client.Send <- msg
			}
			h.Mu.Unlock()
		case client := <-h.Unregister:
			h.Mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.Mu.Unlock()
		case message := <-h.Broadcast:
			h.Mu.Lock()
			// We only save DRAWING to history, but we broadcast EVERYTHING.
			if message.Type == "draw" {
				h.History = append(h.History, message)
			}

			// This loop must run for 'chat' messages too!
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}
			h.Mu.Unlock()
		}
	}
}
