package game

// MessageType discriminates every frame on the wire. Keeping the strings in one
// place lets the compiler catch typos that scattered literals would not.
type MessageType string

const (
	// Client -> server.
	MsgJoin  MessageType = "join"  // {name}
	MsgDraw  MessageType = "draw"  // a line segment (also server -> clients)
	MsgClear MessageType = "clear" // wipe the canvas
	MsgChat  MessageType = "chat"  // chat line / guess (also server -> clients)
	MsgPick  MessageType = "pick"  // drawer picks a word from the offered choices

	// Server -> client.
	MsgHistory MessageType = "history" // replay of the current strokes
	MsgState   MessageType = "state"   // full game snapshot (personalised)
	MsgTimer   MessageType = "timer"   // seconds left in the current phase
	MsgChoices MessageType = "choices" // word options sent only to the drawer
	MsgError   MessageType = "error"   // fatal problem, e.g. unknown room
)

// Chat kinds tell the client how to render a chat line.
const (
	ChatNormal  = "normal"  // an ordinary message: "<sender>: <content>"
	ChatSystem  = "system"  // a server announcement
	ChatCorrect = "correct" // private confirmation that you guessed the word
	ChatQuiet   = "quiet"   // drawer/guessed-players channel, hidden from active guessers
)

// LogicalWidth and LogicalHeight define the fixed drawing coordinate space.
// Every client draws into this space regardless of its window size, so drawings
// stay aligned across clients and survive window resizes. They are exported so
// the values can be shared/validated, and mirrored by the front end.
const (
	LogicalWidth  = 800
	LogicalHeight = 600
)

// Message is the envelope for drawing/chat/clear/join and the simple server
// pushes (timer, history). The drawing coordinates are always serialised (no
// omitempty) because 0 is a valid position.
type Message struct {
	Type MessageType `json:"type"`

	// Join.
	Name string `json:"name,omitempty"`

	// Draw (coordinates are in the logical 0..LogicalWidth/Height space).
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	PrevX float64 `json:"prevX"`
	PrevY float64 `json:"prevY"`
	Color string  `json:"color,omitempty"`
	Size  int     `json:"size,omitempty"`

	// Chat / timer.
	Content string `json:"content,omitempty"`

	// History replay (sent as one message).
	History []Message `json:"history,omitempty"`
}

// ChatMessage is a server-authored chat line. Kind selects the rendering.
type ChatMessage struct {
	Type    MessageType `json:"type"` // always MsgChat
	Kind    string      `json:"kind"`
	Sender  string      `json:"sender,omitempty"`
	Content string      `json:"content"`
}

// ChoicesMessage offers the drawer a set of words to pick from (sent privately).
type ChoicesMessage struct {
	Type    MessageType `json:"type"` // always MsgChoices
	Choices []string    `json:"choices"`
}

// ErrorMessage reports a fatal condition to a single client before closing.
type ErrorMessage struct {
	Type    MessageType `json:"type"` // always MsgError
	Content string      `json:"content"`
}

// PlayerView is the public, per-player information shown in the scoreboard.
type PlayerView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Guessed bool   `json:"guessed"` // has guessed the word this turn
	Drawing bool   `json:"drawing"` // is the current drawer
}

// StateMessage is a full, personalised snapshot of the game. It is sent to a
// client whenever something meaningful changes. It is personalised because the
// drawer sees the real word while guessers see a masked hint.
type StateMessage struct {
	Type       MessageType  `json:"type"` // always MsgState
	Phase      Phase        `json:"phase"`
	Round      int          `json:"round"`
	MaxRounds  int          `json:"maxRounds"`
	TimeLeft   int          `json:"timeLeft"`
	Players    []PlayerView `json:"players"`
	YouID      string       `json:"youId"`      // empty until this client joins
	IsDrawer   bool         `json:"isDrawer"`   // is this client the drawer
	DrawerID   string       `json:"drawerId"`   // current drawer, if any
	DrawerName string       `json:"drawerName"` // current drawer's name, if any
	Word       string       `json:"word"`       // full word or masked hint
	WordMasked bool         `json:"wordMasked"` // true when Word is a hint
}
