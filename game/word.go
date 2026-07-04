package game

import (
	"math/rand"
	"strings"
)

// WordProvider supplies candidate words for a drawing turn. Returning several
// candidates lets the drawer pick one of them. Depending on an interface rather
// than a concrete source keeps the game logic decoupled from where words come
// from — a static list, an AI service, a database (Dependency Inversion).
type WordProvider interface {
	// Words returns up to count distinct candidate words for the topic.
	Words(topic string, count int) ([]string, error)
}

// defaultWords is used when a topic has no dedicated list.
var defaultWords = []string{"apple", "house", "rocket", "flower", "guitar", "castle", "banana", "bridge"}

// LocalWordProvider serves words from an in-memory list keyed by topic. Having
// no external dependencies makes it the reliable default provider.
type LocalWordProvider struct {
	words map[string][]string
}

// NewLocalWordProvider returns a provider seeded with a handful of topics.
func NewLocalWordProvider() *LocalWordProvider {
	return &LocalWordProvider{
		words: map[string][]string{
			"animals": {"elephant", "giraffe", "penguin", "kangaroo", "octopus", "dolphin", "hedgehog", "flamingo", "squirrel", "tiger"},
			"food":    {"pizza", "burger", "spaghetti", "pancake", "cupcake", "avocado", "popcorn", "sandwich", "pretzel", "donut"},
			"objects": {"umbrella", "guitar", "telescope", "backpack", "lantern", "compass", "scissors", "anchor", "hammer", "kettle"},
		},
	}
}

// Words returns up to count distinct random words for the topic, falling back to
// a general list when the topic is unknown. It never returns an error.
func (p *LocalWordProvider) Words(topic string, count int) ([]string, error) {
	list, ok := p.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok || len(list) == 0 {
		list = defaultWords
	}

	pool := make([]string, len(list))
	copy(pool, list)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	if count > len(pool) {
		count = len(pool)
	}
	return pool[:count], nil
}
