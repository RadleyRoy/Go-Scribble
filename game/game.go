package game

import (
	"context"
	"log"
	"strings"
)

const fallbackWord = "apple"

// Game drives the round lifecycle. Each round it picks a word for the topic,
// clears the canvas, announces the word and runs the countdown. It depends on
// the WordProvider abstraction rather than any concrete word source, so the
// source can be swapped without touching this logic (Dependency Inversion).
type Game struct {
	hub          *Hub
	words        WordProvider
	topic        string
	roundSeconds int
}

// NewGame wires a game to a hub and a word provider.
func NewGame(hub *Hub, words WordProvider, topic string, roundSeconds int) *Game {
	return &Game{
		hub:          hub,
		words:        words,
		topic:        topic,
		roundSeconds: roundSeconds,
	}
}

// Run plays rounds back to back until ctx is cancelled.
func (g *Game) Run(ctx context.Context) {
	for ctx.Err() == nil {
		g.playRound(ctx)
	}
}

func (g *Game) playRound(ctx context.Context) {
	word, err := g.words.Word(g.topic)
	if err != nil {
		log.Printf("word generation failed, using fallback %q: %v", fallbackWord, err)
		word = fallbackWord
	}
	word = strings.ToLower(strings.TrimSpace(word))

	// Wipe the previous round's drawing, then announce the new word.
	g.hub.Broadcast(Message{Type: MessageClear})
	g.hub.Broadcast(Message{Type: MessageWord, Content: word})

	Countdown(ctx, g.hub, g.roundSeconds)
}
