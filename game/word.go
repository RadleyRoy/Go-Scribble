package game

import (
	"math/rand"
	"strings"
)

// WordProvider supplies the word to be drawn for a given topic. Depending on an
// interface rather than a concrete source lets the game draw words from a
// static list, a remote AI service, a database, and so on without any change to
// the game logic (Dependency Inversion Principle).
type WordProvider interface {
	Word(topic string) (string, error)
}

// defaultWords is used when a topic has no dedicated list.
var defaultWords = []string{"apple", "house", "rocket", "flower", "guitar", "castle"}

// LocalWordProvider serves words from an in-memory list keyed by topic. Having
// no external dependencies makes it the reliable default provider.
type LocalWordProvider struct {
	words map[string][]string
}

// NewLocalWordProvider returns a provider seeded with a handful of topics.
func NewLocalWordProvider() *LocalWordProvider {
	return &LocalWordProvider{
		words: map[string][]string{
			"animals": {"elephant", "giraffe", "penguin", "kangaroo", "octopus", "dolphin", "hedgehog", "flamingo"},
			"food":    {"pizza", "burger", "spaghetti", "pancake", "cupcake", "avocado", "popcorn", "sandwich"},
			"objects": {"umbrella", "guitar", "telescope", "backpack", "lantern", "compass", "scissors", "anchor"},
		},
	}
}

// Word returns a random word for the topic, falling back to a general list when
// the topic is unknown. It never returns an error.
func (p *LocalWordProvider) Word(topic string) (string, error) {
	list, ok := p.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok || len(list) == 0 {
		list = defaultWords
	}
	return list[rand.Intn(len(list))], nil
}
