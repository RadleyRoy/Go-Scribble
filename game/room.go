package game

import (
	"context"
	"crypto/rand"
	"strings"
	"sync"
)

const (
	roomCodeLength = 4
	// Unambiguous alphabet (no 0/O, 1/I) for codes players type by hand.
	roomCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// RoomManager owns the set of private rooms, each backed by its own Engine
// running on its own goroutine. It is safe for concurrent use by HTTP handlers.
type RoomManager struct {
	mu    sync.Mutex
	rooms map[string]*room
	words WordProvider
	cfg   Config
}

type room struct {
	engine *Engine
	cancel context.CancelFunc
}

// NewRoomManager creates a manager whose rooms all share the given word
// provider and configuration.
func NewRoomManager(words WordProvider, cfg Config) *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*room),
		words: words,
		cfg:   cfg,
	}
}

// Create makes a new room with a unique code, starts its engine, and returns
// the code. The room is torn down automatically once its last client leaves.
func (m *RoomManager) Create() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	code := m.uniqueCodeLocked()
	ctx, cancel := context.WithCancel(context.Background())
	engine := NewEngine(m.words, m.cfg)
	engine.onEmpty = func() { m.remove(code) }

	m.rooms[code] = &room{engine: engine, cancel: cancel}
	go engine.Run(ctx)
	return code
}

// Get returns the engine for a room code (case-insensitive), if it exists.
func (m *RoomManager) Get(code string) (*Engine, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return nil, false
	}
	return r.engine, true
}

// remove stops a room's engine and forgets it. Safe to call more than once.
func (m *RoomManager) remove(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rooms[code]; ok {
		r.cancel()
		delete(m.rooms, code)
	}
}

func (m *RoomManager) uniqueCodeLocked() string {
	for {
		code := randomCode()
		if _, exists := m.rooms[code]; !exists {
			return code
		}
	}
}

func randomCode() string {
	b := make([]byte, roomCodeLength)
	_, _ = rand.Read(b)
	out := make([]byte, roomCodeLength)
	for i, c := range b {
		out[i] = roomCodeAlphabet[int(c)%len(roomCodeAlphabet)]
	}
	return string(out)
}
