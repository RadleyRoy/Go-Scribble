package game

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeWordProvider generates drawing words with the Claude API via the
// official Anthropic Go SDK. It is an optional WordProvider; the game works
// offline through LocalWordProvider when no API key is configured.
type ClaudeWordProvider struct {
	client anthropic.Client
	model  anthropic.Model
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
		client: anthropic.NewClient(append(base, opts...)...),
		model:  anthropic.ModelClaudeOpus4_8,
	}
}

// Words asks Claude for count simple, drawable words on the topic.
func (p *ClaudeWordProvider) Words(topic string, count int) ([]string, error) {
	prompt := fmt.Sprintf(
		"List %d different, simple, common nouns that are easy to sketch in a "+
			"Pictionary-style drawing game, on the topic %q. Reply with ONLY the "+
			"words — one per line, all lowercase, no numbering, punctuation, or extra text.",
		count, topic,
	)

	// Thinking is left off: this is a trivial generation task and the turn is
	// waiting on the response, so latency matters more than deliberation.
	resp, err := p.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 256,
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

	words := parseWordList(b.String(), count)
	if len(words) == 0 {
		return nil, fmt.Errorf("claude returned no usable words")
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
