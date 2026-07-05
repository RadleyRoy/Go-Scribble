package game

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeWordProvider generates drawing words with the Claude API via the
// official Anthropic Go SDK. If a request fails (no credit, network, etc.) it
// falls back to the offline word list so the game keeps serving varied,
// topic-appropriate words instead of getting stuck on a fixed set.
type ClaudeWordProvider struct {
	client   anthropic.Client
	model    anthropic.Model
	fallback *LocalWordProvider
}

// NewClaudeWordProvider builds a provider authenticated with the given API key.
// A bounded request timeout ensures a slow API can never hang a turn. Extra
// request options (e.g. a custom base URL) can be supplied, which tests use to
// point the client at a stub server.
func NewClaudeWordProvider(apiKey string, opts ...option.RequestOption) *ClaudeWordProvider {
	base := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(15 * time.Second),
	}
	return &ClaudeWordProvider{
		client:   anthropic.NewClient(append(base, opts...)...),
		model:    anthropic.ModelClaudeOpus4_8,
		fallback: NewLocalWordProvider(),
	}
}

// Words asks Claude for count words, falling back to the local list on failure.
// It never returns an error so a failing API degrades gracefully rather than
// pinning the game to the same fixed words each turn.
func (p *ClaudeWordProvider) Words(topic string, count int) ([]string, error) {
	words, err := p.generate(topic, count)
	if err != nil {
		log.Printf("Claude word generation failed, using local words: %v", err)
		return p.fallback.Words(topic, count)
	}
	return words, nil
}

// Verify makes one minimal request to confirm the API key works and the account
// can reach the model, returning the underlying error if it cannot. It is used
// at startup to surface auth/credit/model problems clearly.
func (p *ClaudeWordProvider) Verify() error {
	_, err := p.generate("animals", 1)
	return err
}

// generate performs the actual Claude request and returns an error on failure.
//
// Opus 4.8 rejects the temperature/top_p sampling knobs, so asking for exactly
// count words makes the model collapse onto the few most obvious answers (cat,
// dog, fish…) every turn. Instead we request a deliberately larger, varied pool
// and randomly sample count from it here. A per-call variety token nudges the
// model off its default answer without any sampling parameters.
func (p *ClaudeWordProvider) generate(topic string, count int) ([]string, error) {
	pool := count * 4
	if pool < 12 {
		pool = 12
	}

	prompt := fmt.Sprintf(
		"Brainstorm %d different, simple, common nouns that are easy to sketch in a "+
			"Pictionary-style drawing game, on the topic %q. Mix obvious and less "+
			"obvious choices for variety, and avoid repeating the same predictable "+
			"picks every time. Reply with ONLY the words — one per line, all "+
			"lowercase, no numbering, punctuation, or extra text.\n"+
			"(variety token: %d — ignore its value, it just seeds fresh choices.)",
		pool, topic, rand.Int63(),
	)

	// Thinking is left off: this is a trivial generation task and the turn is
	// waiting on the response, so latency matters more than deliberation.
	resp, err := p.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude request: %w", err)
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(text.Text)
			b.WriteByte('\n')
		}
	}

	words := parseWordList(b.String(), pool)
	if len(words) == 0 {
		return nil, fmt.Errorf("claude returned no usable words")
	}

	// Sample count words from the larger pool so successive turns differ even
	// when the model returns a similar list.
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	if len(words) > count {
		words = words[:count]
	}
	return words, nil
}

// parseWordList extracts up to max distinct clean words from a newline-separated
// model reply, tolerating stray list markers and punctuation.
func parseWordList(text string, max int) []string {
	seen := make(map[string]bool)
	var words []string
	for _, line := range strings.Split(text, "\n") {
		w := cleanWord(line)
		if w == "" || seen[w] {
			continue
		}
		seen[w] = true
		words = append(words, w)
		if len(words) >= max {
			break
		}
	}
	return words
}

func cleanWord(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Drop leading list markers such as "1.", "-", "*", ")".
	s = strings.TrimLeft(s, "0123456789.-*) \t")
	// Keep only letters, spaces and hyphens.
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r == ' ', r == '-':
			return r
		default:
			return -1
		}
	}, s)
	return strings.TrimSpace(s)
}
