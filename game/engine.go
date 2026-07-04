package game

import (
	"context"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Phase is the current stage of the game's lifecycle.
type Phase string

const (
	PhaseWaiting  Phase = "waiting"  // not enough players yet
	PhaseChoosing Phase = "choosing" // drawer is picking a word
	PhaseDrawing  Phase = "drawing"  // a turn is in progress
	PhaseReveal   Phase = "reveal"   // showing the word between turns
	PhaseGameOver Phase = "gameover" // final scoreboard before a new game
)

const (
	fallbackWordChoice   = "apple"
	wordChoiceCount      = 3     // words offered to the drawer each turn
	drawerPointsPerGuess = 30    // drawer earns this per correct guesser
	maxHistoryStrokes    = 20000 // safety cap on strokes retained per turn
	maxChatLen           = 200
	maxNameLen           = 20
)

func fallbackChoices() []string {
	return []string{"apple", "house", "tree", "car", "star", "fish", "boat", "clock"}
}

// Config tunes the game. Durations are kept configurable so tests can run a
// whole game in milliseconds.
type Config struct {
	Topic         string
	MaxRounds     int
	TurnSeconds   int
	ChooseSeconds int
	RevealSeconds int
	MinPlayers    int
}

// DefaultConfig returns sensible defaults for a public game.
func DefaultConfig() Config {
	return Config{
		Topic:         "animals",
		MaxRounds:     3,
		TurnSeconds:   80,
		ChooseSeconds: 15,
		RevealSeconds: 6,
		MinPlayers:    2,
	}
}

// inbound couples a message with the client that sent it.
type inbound struct {
	client *Client
	msg    Message
}

// wordResult carries an async word-provider result back to the engine goroutine.
// gen guards against a stale result arriving after the turn has moved on.
type wordResult struct {
	gen   int
	words []string
}

// Engine owns all game and connection state and runs it on a single goroutine.
// Every field is read and written only from Run, so no locking is needed:
// other goroutines interact solely through the channels (Ownership over Mutexes).
type Engine struct {
	words WordProvider
	cfg   Config

	register   chan *Client
	unregister chan *Client
	incoming   chan inbound
	wordsReady chan wordResult
	stopped    chan struct{} // closed when Run returns

	// onEmpty, if set, is called (on the engine goroutine) when the last client
	// leaves — the RoomManager uses it to tear the room down.
	onEmpty func()

	clients map[*Client]*Player // nil value means "connected but not joined"
	players []*Player           // join order; drives turn rotation
	nextID  int

	phase          Phase
	round          int
	turnsThisRound int
	drawerIdx      int // index into players; -1 when there is no drawer
	timeLeft       int

	choices      []string // word options for the current chooser; nil until fetched
	wordGen      int      // increments each turn to invalidate stale fetches
	word         string
	wordRunes    []rune
	revealed     []bool // which letters are revealed as hints
	guessedCount int    // guessers who have guessed this turn

	history []Message // strokes drawn this turn
}

// NewEngine creates an engine backed by the given word provider.
func NewEngine(words WordProvider, cfg Config) *Engine {
	return &Engine{
		words:      words,
		cfg:        cfg,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan inbound),
		wordsReady: make(chan wordResult),
		stopped:    make(chan struct{}),
		clients:    make(map[*Client]*Player),
		phase:      PhaseWaiting,
		drawerIdx:  -1,
	}
}

// Run drives the engine until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer close(e.stopped)

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-e.register:
			e.handleRegister(c)
		case c := <-e.unregister:
			e.handleUnregister(c)
		case in := <-e.incoming:
			e.handleMessage(in.client, in.msg)
		case res := <-e.wordsReady:
			e.handleWords(res)
		case <-ticker.C:
			e.tick()
		}
	}
}

// --- connection lifecycle ---------------------------------------------------

func (e *Engine) handleRegister(c *Client) {
	e.clients[c] = nil // spectator until it sends a join
	e.sendHistoryTo(c)
	e.sendState(c)
}

func (e *Engine) handleUnregister(c *Client) {
	p, ok := e.clients[c]
	if !ok {
		return
	}
	delete(e.clients, c)
	close(c.send)

	if p != nil {
		e.removePlayer(p)
	}
	if len(e.clients) == 0 && e.onEmpty != nil {
		e.onEmpty()
	}
}

func (e *Engine) removePlayer(p *Player) {
	idx := e.indexOfPlayer(p)
	if idx < 0 {
		return
	}
	wasDrawer := e.drawerIdx == idx && (e.phase == PhaseDrawing || e.phase == PhaseChoosing)
	wasChoosing := e.phase == PhaseChoosing

	e.players = append(e.players[:idx], e.players[idx+1:]...)
	// Keep drawerIdx pointing at the same logical position after the shift.
	if idx <= e.drawerIdx {
		e.drawerIdx--
	}
	e.systemChat(p.name + " left the game.")

	switch {
	case e.playerCount() < e.cfg.MinPlayers && e.phase != PhaseWaiting:
		e.toWaiting()
	case wasDrawer && wasChoosing:
		e.systemChat("The drawer left. Skipping the turn.")
		e.nextTurnOrEnd()
	case wasDrawer:
		e.endTurn()
	default:
		e.checkAllGuessed()
		e.broadcastState()
	}
}

// --- inbound messages -------------------------------------------------------

func (e *Engine) handleMessage(c *Client, msg Message) {
	switch msg.Type {
	case MsgJoin:
		e.handleJoin(c, msg)
	case MsgDraw:
		e.handleDraw(c, msg)
	case MsgClear:
		e.handleClear(c)
	case MsgChat:
		e.handleChat(c, msg)
	case MsgPick:
		e.handlePick(c, msg)
	}
}

func (e *Engine) handleJoin(c *Client, msg Message) {
	if _, ok := e.clients[c]; !ok {
		return // unknown connection
	}
	if e.clients[c] != nil {
		return // already joined
	}
	e.nextID++
	p := &Player{id: strconv.Itoa(e.nextID), name: sanitizeName(msg.Name), client: c}
	e.players = append(e.players, p)
	e.clients[c] = p

	e.systemChat(p.name + " joined the game.")
	e.broadcastState()
	e.maybeStartGame()
}

func (e *Engine) handleDraw(c *Client, msg Message) {
	p := e.clients[c]
	if p == nil || e.phase != PhaseDrawing || !e.isDrawer(p) {
		return
	}
	seg := sanitizeDraw(msg)
	if len(e.history) < maxHistoryStrokes {
		e.history = append(e.history, seg)
	}
	e.broadcast(seg)
}

func (e *Engine) handleClear(c *Client) {
	p := e.clients[c]
	if p == nil || e.phase != PhaseDrawing || !e.isDrawer(p) {
		return
	}
	e.history = nil
	e.broadcast(Message{Type: MsgClear})
}

func (e *Engine) handleChat(c *Client, msg Message) {
	p := e.clients[c]
	if p == nil {
		return
	}
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		return
	}
	if len(text) > maxChatLen {
		text = text[:maxChatLen]
	}

	// A guesser who hasn't guessed yet may score by naming the word.
	if e.phase == PhaseDrawing && !e.isDrawer(p) && !p.guessed {
		if equalsWord(text, e.word) {
			e.acceptGuess(c, p)
			return
		}
	}

	// Nobody may broadcast the exact word while a turn is live (no spoilers).
	if e.phase == PhaseDrawing && equalsWord(text, e.word) {
		e.send(c, ChatMessage{Type: MsgChat, Kind: ChatSystem, Content: "Shh — no spoilers!"})
		return
	}

	e.broadcast(ChatMessage{Type: MsgChat, Kind: ChatNormal, Sender: p.name, Content: text})
}

func (e *Engine) handlePick(c *Client, msg Message) {
	p := e.clients[c]
	if p == nil || e.phase != PhaseChoosing || !e.isDrawer(p) {
		return
	}
	chosen := strings.TrimSpace(msg.Content)
	for _, w := range e.choices {
		if strings.EqualFold(w, chosen) {
			e.startDrawing(w)
			return
		}
	}
}

func (e *Engine) acceptGuess(c *Client, p *Player) {
	p.guessed = true
	e.guessedCount++
	pts := e.guessPoints()
	p.score += pts

	e.systemChat(p.name + " guessed the word!")
	e.send(c, ChatMessage{
		Type:    MsgChat,
		Kind:    ChatCorrect,
		Content: "You guessed the word! +" + strconv.Itoa(pts),
	})
	e.broadcastState()
	e.checkAllGuessed()
}

// --- turn / round lifecycle -------------------------------------------------

func (e *Engine) maybeStartGame() {
	if e.phase != PhaseWaiting || e.playerCount() < e.cfg.MinPlayers {
		return
	}
	e.round = 1
	e.turnsThisRound = 0
	e.drawerIdx = -1
	for _, p := range e.players {
		p.score = 0
	}
	e.beginChoosing()
}

// beginChoosing advances to the next drawer and asks the word provider for
// candidates. The fetch runs in a goroutine so a slow (networked) provider can
// never freeze the room; the result arrives on wordsReady, guarded by wordGen.
func (e *Engine) beginChoosing() {
	if e.playerCount() < e.cfg.MinPlayers {
		e.toWaiting()
		return
	}

	e.drawerIdx = (e.drawerIdx + 1) % len(e.players)
	e.turnsThisRound++
	for _, p := range e.players {
		p.guessed = false
	}
	e.guessedCount = 0
	e.history = nil
	e.word = ""
	e.wordRunes = nil
	e.revealed = nil
	e.choices = nil // nil marks "still fetching"; the tick countdown waits on it

	e.phase = PhaseChoosing
	e.timeLeft = e.cfg.ChooseSeconds
	e.wordGen++
	gen := e.wordGen

	e.broadcast(Message{Type: MsgClear})
	e.systemChat(e.currentDrawer().name + " is choosing a word...")
	e.broadcastState()

	provider, topic, stopped := e.words, e.cfg.Topic, e.stopped
	go func() {
		words, err := provider.Words(topic, wordChoiceCount)
		if err != nil {
			log.Printf("word provider failed, using fallback words: %v", err)
		}
		if err != nil || len(words) == 0 {
			words = fallbackChoices()
		}
		select {
		case e.wordsReady <- wordResult{gen: gen, words: words}:
		case <-stopped:
		}
	}()
}

func (e *Engine) handleWords(res wordResult) {
	if res.gen != e.wordGen || e.phase != PhaseChoosing {
		return // stale — the turn already moved on
	}
	e.choices = normalizeChoices(res.words, wordChoiceCount)
	e.timeLeft = e.cfg.ChooseSeconds // give the drawer full time once options appear
	if d := e.currentDrawer(); d != nil {
		e.send(d.client, ChoicesMessage{Type: MsgChoices, Choices: e.choices})
	}
	e.broadcastState()
}

func (e *Engine) startDrawing(word string) {
	e.word = strings.ToLower(strings.TrimSpace(word))
	if e.word == "" {
		e.word = fallbackWordChoice
	}
	e.wordRunes = []rune(e.word)
	e.revealed = make([]bool, len(e.wordRunes))
	e.choices = nil

	e.phase = PhaseDrawing
	e.timeLeft = e.cfg.TurnSeconds
	e.systemChat(e.currentDrawer().name + " is drawing now!")
	e.broadcastState()
}

func (e *Engine) endTurn() {
	if d := e.currentDrawer(); d != nil {
		d.score += e.guessedCount * drawerPointsPerGuess
	}
	e.phase = PhaseReveal
	e.timeLeft = e.cfg.RevealSeconds
	e.systemChat("The word was: " + e.word)
	e.broadcastState()
}

func (e *Engine) nextTurnOrEnd() {
	if e.turnsThisRound >= len(e.players) {
		e.round++
		e.turnsThisRound = 0
		if e.round > e.cfg.MaxRounds {
			e.gameOver()
			return
		}
	}
	e.beginChoosing()
}

func (e *Engine) gameOver() {
	e.phase = PhaseGameOver
	e.timeLeft = e.cfg.RevealSeconds * 2
	if w := e.leader(); w != nil {
		e.systemChat("Game over! Winner: " + w.name + " with " + strconv.Itoa(w.score) + " points.")
	} else {
		e.systemChat("Game over!")
	}
	e.broadcastState()
}

func (e *Engine) toWaiting() {
	e.phase = PhaseWaiting
	e.word = ""
	e.wordRunes = nil
	e.revealed = nil
	e.choices = nil
	e.history = nil
	e.drawerIdx = -1
	e.broadcast(Message{Type: MsgClear})
	e.systemChat("Waiting for more players to join...")
	e.broadcastState()
}

func (e *Engine) resetToLobby() {
	e.phase = PhaseWaiting
	e.word = ""
	e.wordRunes = nil
	e.revealed = nil
	e.choices = nil
	e.history = nil
	e.drawerIdx = -1
	e.broadcast(Message{Type: MsgClear})
	e.broadcastState()
	e.maybeStartGame() // start a fresh game right away if players remain
}

// tick advances the countdown for the active phase once per second.
func (e *Engine) tick() {
	switch e.phase {
	case PhaseChoosing:
		if e.choices == nil {
			return // still fetching words; don't burn the clock
		}
		e.timeLeft--
		e.broadcast(Message{Type: MsgTimer, Content: strconv.Itoa(e.timeLeft)})
		if e.timeLeft <= 0 {
			e.startDrawing(e.choices[rand.Intn(len(e.choices))]) // auto-pick
		}
	case PhaseDrawing:
		e.timeLeft--
		e.broadcast(Message{Type: MsgTimer, Content: strconv.Itoa(e.timeLeft)})
		e.maybeRevealHint()
		if e.timeLeft <= 0 {
			e.endTurn()
		}
	case PhaseReveal:
		e.timeLeft--
		e.broadcast(Message{Type: MsgTimer, Content: strconv.Itoa(e.timeLeft)})
		if e.timeLeft <= 0 {
			e.nextTurnOrEnd()
		}
	case PhaseGameOver:
		e.timeLeft--
		if e.timeLeft <= 0 {
			e.resetToLobby()
		}
	}
}

// maybeRevealHint reveals letters of the word as the turn progresses, never
// revealing more than half of them.
func (e *Engine) maybeRevealHint() {
	letters := 0
	for _, r := range e.wordRunes {
		if r != ' ' {
			letters++
		}
	}
	maxHints := letters / 2
	if maxHints < 1 {
		return
	}

	elapsed := e.cfg.TurnSeconds - e.timeLeft
	target := 0
	if e.cfg.TurnSeconds > 0 {
		target = maxHints * elapsed / e.cfg.TurnSeconds
	}

	revealedCount := 0
	for _, r := range e.revealed {
		if r {
			revealedCount++
		}
	}

	changed := false
	for revealedCount < target {
		var hidden []int
		for i, r := range e.wordRunes {
			if r != ' ' && !e.revealed[i] {
				hidden = append(hidden, i)
			}
		}
		if len(hidden) == 0 {
			break
		}
		e.revealed[hidden[rand.Intn(len(hidden))]] = true
		revealedCount++
		changed = true
	}
	if changed {
		e.broadcastState()
	}
}

func (e *Engine) checkAllGuessed() {
	if e.phase != PhaseDrawing {
		return
	}
	guessers := e.playerCount() - 1 // everyone but the drawer
	if guessers > 0 && e.guessedCount >= guessers {
		e.endTurn()
	}
}

// --- delivery ---------------------------------------------------------------

// broadcast delivers a message to every connected client. A client whose buffer
// is full is force-closed; its read pump then unregisters it cleanly. The map
// is never mutated here, so ranging over it is safe.
func (e *Engine) broadcast(v interface{}) {
	for c := range e.clients {
		if !c.trySend(v) {
			c.conn.Close()
		}
	}
}

func (e *Engine) send(c *Client, v interface{}) {
	if !c.trySend(v) {
		c.conn.Close()
	}
}

func (e *Engine) systemChat(text string) {
	e.broadcast(ChatMessage{Type: MsgChat, Kind: ChatSystem, Content: text})
}

func (e *Engine) sendHistoryTo(c *Client) {
	if len(e.history) == 0 {
		return
	}
	replay := make([]Message, len(e.history))
	copy(replay, e.history)
	e.send(c, Message{Type: MsgHistory, History: replay})
}

func (e *Engine) broadcastState() {
	for c := range e.clients {
		e.sendState(c)
	}
}

func (e *Engine) sendState(c *Client) {
	e.send(c, e.stateFor(c))
}

func (e *Engine) stateFor(c *Client) StateMessage {
	me := e.clients[c]
	drawer := e.currentDrawer()

	st := StateMessage{
		Type:      MsgState,
		Phase:     e.phase,
		Round:     e.round,
		MaxRounds: e.cfg.MaxRounds,
		TimeLeft:  e.timeLeft,
		Players:   e.playerViews(),
	}
	if me != nil {
		st.YouID = me.id
	}
	if drawer != nil {
		st.DrawerID = drawer.id
		st.DrawerName = drawer.name
		st.IsDrawer = me == drawer
	}

	switch e.phase {
	case PhaseReveal, PhaseGameOver:
		st.Word = e.word // reveal the answer to everyone
	case PhaseDrawing:
		if st.IsDrawer {
			st.Word = e.word
		} else {
			st.Word = e.maskedWord()
			st.WordMasked = true
		}
	}
	return st
}

func (e *Engine) playerViews() []PlayerView {
	drawer := e.currentDrawer()
	views := make([]PlayerView, 0, len(e.players))
	for _, p := range e.players {
		views = append(views, PlayerView{
			ID:      p.id,
			Name:    p.name,
			Score:   p.score,
			Guessed: p.guessed,
			Drawing: p == drawer,
		})
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Score > views[j].Score })
	return views
}

// --- helpers ----------------------------------------------------------------

func (e *Engine) playerCount() int { return len(e.players) }

func (e *Engine) isDrawer(p *Player) bool {
	return p != nil && e.currentDrawer() == p
}

func (e *Engine) currentDrawer() *Player {
	switch e.phase {
	case PhaseChoosing, PhaseDrawing, PhaseReveal:
		if e.drawerIdx >= 0 && e.drawerIdx < len(e.players) {
			return e.players[e.drawerIdx]
		}
	}
	return nil
}

func (e *Engine) indexOfPlayer(p *Player) int {
	for i, pl := range e.players {
		if pl == p {
			return i
		}
	}
	return -1
}

func (e *Engine) leader() *Player {
	var best *Player
	for _, p := range e.players {
		if best == nil || p.score > best.score {
			best = p
		}
	}
	return best
}

func (e *Engine) guessPoints() int {
	if e.cfg.TurnSeconds <= 0 {
		return 100
	}
	return 100 + (e.timeLeft*100)/e.cfg.TurnSeconds // 100..200, faster is better
}

// maskedWord renders the word with hidden letters shown as underscores and
// revealed letters (hints) shown in place. Spaces are always shown.
func (e *Engine) maskedWord() string {
	var b strings.Builder
	for i, r := range e.wordRunes {
		switch {
		case r == ' ':
			b.WriteRune(' ')
		case e.revealed[i]:
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// normalizeChoices returns up to n distinct lowercase words, padding with
// fallbacks if the provider returned too few.
func normalizeChoices(words []string, n int) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(w string) {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" || seen[w] {
			return
		}
		seen[w] = true
		out = append(out, w)
	}
	for _, w := range words {
		if len(out) >= n {
			break
		}
		add(w)
	}
	for _, w := range fallbackChoices() {
		if len(out) >= n {
			break
		}
		add(w)
	}
	return out
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r < ' ' { // strip control characters / newlines
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > maxNameLen {
		name = string(r[:maxNameLen])
	}
	if name == "" {
		return "Player"
	}
	return name
}

// sanitizeDraw rebuilds a draw message from client input, keeping only drawing
// fields and clamping the brush size to a sane range.
func sanitizeDraw(msg Message) Message {
	size := msg.Size
	if size < 1 {
		size = 1
	}
	if size > 100 {
		size = 100
	}
	return Message{
		Type:  MsgDraw,
		X:     msg.X,
		Y:     msg.Y,
		PrevX: msg.PrevX,
		PrevY: msg.PrevY,
		Color: msg.Color,
		Size:  size,
	}
}

func equalsWord(guess, word string) bool {
	return strings.EqualFold(strings.TrimSpace(guess), strings.TrimSpace(word))
}
