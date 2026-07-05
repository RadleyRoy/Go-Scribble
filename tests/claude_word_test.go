package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"go-scribble/game"
)

// TestClaudeWordProviderParsesResponse points the provider at a stub Anthropic
// server and checks that it sends auth and cleans up the model's reply into a
// tidy word list (stripping list markers, casing, and punctuation).
func TestClaudeWordProviderParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Errorf("expected x-api-key header on the request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_1",
			"type": "message",
			"role": "assistant",
			"model": "claude-opus-4-8",
			"content": [{"type": "text", "text": "1. Cat\n2. Dog\n- bird!\n"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 5}
		}`))
	}))
	defer server.Close()

	p := game.NewClaudeWordProvider("test-key", option.WithBaseURL(server.URL))
	words, err := p.Words("animals", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The provider samples/shuffles its pool, so assert on set membership rather
	// than order.
	want := map[string]bool{"cat": true, "dog": true, "bird": true}
	if len(words) != len(want) {
		t.Fatalf("expected %d words from %v, got %v", len(want), want, words)
	}
	for _, w := range words {
		if !want[w] {
			t.Fatalf("unexpected word %q (all: %v)", w, words)
		}
	}
}

// TestClaudeWordProviderFallsBackOnError verifies that when the API call fails
// the provider returns varied local words instead of erroring — which is what
// keeps the game from repeating the same fixed words every turn.
func TestClaudeWordProviderFallsBackOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest) // 400 is not retried by the SDK
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"credit balance too low"}}`))
	}))
	defer server.Close()

	p := game.NewClaudeWordProvider("test-key", option.WithBaseURL(server.URL))
	words, err := p.Words("animals", 3)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if len(words) != 3 {
		t.Fatalf("expected 3 fallback words, got %v", words)
	}
	seen := map[string]bool{}
	for _, w := range words {
		if w == "" || seen[w] {
			t.Fatalf("expected distinct non-empty words, got %v", words)
		}
		seen[w] = true
	}
}

// compile-time check that both providers satisfy the interface.
var (
	_ game.WordProvider = (*game.LocalWordProvider)(nil)
	_ game.WordProvider = (*game.ClaudeWordProvider)(nil)
)
