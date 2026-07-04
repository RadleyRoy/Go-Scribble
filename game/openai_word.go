package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIWordProvider generates words via the OpenAI chat completions API. It is
// an optional WordProvider; the game works fine without it via LocalWordProvider.
type OpenAIWordProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIWordProvider builds a provider that authenticates with the given API
// key and uses a bounded HTTP timeout so a slow API can never hang a round.
func NewOpenAIWordProvider(apiKey string) *OpenAIWordProvider {
	return &OpenAIWordProvider{
		apiKey: apiKey,
		model:  "gpt-4o-mini",
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// The request/response shapes are declared as concrete types so decoding is
// total and safe: a malformed or error response yields an error instead of the
// panics the original untyped map assertions could produce.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Word asks the model for a single word for the topic.
func (p *OpenAIWordProvider) Word(topic string) (string, error) {
	reqBody := openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{{
			Role:    "user",
			Content: "Reply with exactly ONE simple, common, lowercase noun (no punctuation or extra words) suitable for a drawing game. Topic: " + topic,
		}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, openAIEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call openai: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("openai error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices (status %d)", resp.StatusCode)
	}

	word := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if word == "" {
		return "", fmt.Errorf("openai returned an empty word")
	}
	return word, nil
}
