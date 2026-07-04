package game

import "testing"

// Compile-time proof that both providers satisfy the WordProvider interface.
var (
	_ WordProvider = (*LocalWordProvider)(nil)
	_ WordProvider = (*OpenAIWordProvider)(nil)
)

func TestLocalWordProviderReturnsWordFromTopic(t *testing.T) {
	p := NewLocalWordProvider()

	valid := make(map[string]bool)
	for _, w := range p.words["animals"] {
		valid[w] = true
	}

	// Draw many times so a bad index or empty list would surface.
	for i := 0; i < 100; i++ {
		word, err := p.Word("Animals") // also checks case-insensitivity
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !valid[word] {
			t.Fatalf("word %q is not in the animals list", word)
		}
	}
}

func TestLocalWordProviderUnknownTopicFallsBack(t *testing.T) {
	p := NewLocalWordProvider()

	word, err := p.Word("this-topic-does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if word == "" {
		t.Fatal("expected a fallback word, got empty string")
	}
}
