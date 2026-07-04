package game

import (
	"context"
	"strconv"
	"time"
)

// Countdown broadcasts a MessageTimer tick once per second, counting from
// seconds down to 0. It blocks until the countdown finishes or ctx is
// cancelled, which lets a round abort cleanly on shutdown.
func Countdown(ctx context.Context, hub *Hub, seconds int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for remaining := seconds; remaining >= 0; remaining-- {
		hub.Broadcast(Message{Type: MessageTimer, Content: strconv.Itoa(remaining)})
		if remaining == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
