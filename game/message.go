package game

type DrawMessage struct {
	Type    string  `json:"type"` // "draw", "clear", "chat"
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	PrevX   float64 `json:"prevX"`
	PrevY   float64 `json:"prevY"`
	Color   string  `json:"color"`
	Size    int     `json:"size"`
	Content string  `json:"content"`
	Sender  string  `json:"sender"`
}
