package game

// MessageType discriminates the payload carried by a Message. Using a named
// type with named constants keeps the wire-protocol strings in one place and
// lets the compiler catch typos that raw string literals would not.
type MessageType string

const (
	// MessageDraw is a single line segment drawn on the canvas.
	MessageDraw MessageType = "draw"
	// MessageClear wipes the canvas for everyone.
	MessageClear MessageType = "clear"
	// MessageChat is a chat line.
	MessageChat MessageType = "chat"
	// MessageTimer is a round countdown tick (Content holds the seconds left).
	MessageTimer MessageType = "timer"
	// MessageWord announces the word for the current round (Content holds it).
	MessageWord MessageType = "word"
	// MessageHistory replays the strokes drawn so far to a newly joined client.
	MessageHistory MessageType = "history"
)

// Message is the single envelope exchanged over the WebSocket connection in
// both directions. The Type field selects which of the remaining fields are
// meaningful; a single envelope keeps (de)serialisation trivial on both the
// Go and JavaScript sides.
//
// The drawing coordinates are always serialised (no omitempty) because 0 is a
// valid pixel position and must not be dropped from the JSON.
type Message struct {
	Type MessageType `json:"type"`

	// Drawing fields (Type == MessageDraw).
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	PrevX float64 `json:"prevX"`
	PrevY float64 `json:"prevY"`
	Color string  `json:"color,omitempty"`
	Size  int     `json:"size,omitempty"`

	// Text fields (Type == MessageChat/MessageWord/MessageTimer).
	Content string `json:"content,omitempty"` // chat text, word, or timer value
	Sender  string `json:"sender,omitempty"`  // chat author

	// Replay field (Type == MessageHistory): the strokes drawn so far, sent as
	// one message so the hub never has to stream them individually.
	History []Message `json:"history,omitempty"`
}
