package game

// Player is a single participant in the game. It is owned exclusively by the
// engine goroutine, so it needs no synchronisation of its own.
type Player struct {
	id      string
	name    string
	score   int
	guessed bool // has guessed the current word
	client  *Client
}
