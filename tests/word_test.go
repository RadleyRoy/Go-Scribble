// Package tests holds black-box tests that exercise the public API of the game
// package (kept in a separate folder from the source files by request).
package tests

import (
	"strings"
	"testing"

	"go-scribble/game"
)

func TestLocalWordProviderReturnsDistinctWords(t *testing.T) {
	p := game.NewLocalWordProvider()
	for _, topic := range []string{"animals", "food", "objects", "unknown-topic"} {
		words, err := p.Words(topic, 3)
		if err != nil {
			t.Fatalf("topic %q: unexpected error: %v", topic, err)
		}
		if len(words) != 3 {
			t.Fatalf("topic %q: expected 3 words, got %v", topic, words)
		}
		seen := map[string]bool{}
		for _, w := range words {
			if strings.TrimSpace(w) == "" {
				t.Fatalf("topic %q: got an empty word in %v", topic, words)
			}
			if seen[w] {
				t.Fatalf("topic %q: duplicate word %q in %v", topic, w, words)
			}
			seen[w] = true
		}
	}
}

func TestLocalWordProviderIsCaseInsensitive(t *testing.T) {
	p := game.NewLocalWordProvider()
	if words, err := p.Words("ANIMALS", 3); err != nil || len(words) == 0 {
		t.Fatalf("expected words for ANIMALS, got %v (err %v)", words, err)
	}
}
