package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

type OpenAIClient struct {
	baseURL   string
	model     string
	apiKey    string
	maxTokens int
	client    *http.Client
}

type openAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message Message `json:"message"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Delta openAIDelta `json:"delta"`
}

type openAIDelta struct {
	Content string `json:"content"`
}

func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	body := openAIRequest{
		Model:     c.model,
		Messages:  []Message{{Role: "user", Content: prompt}},
		Stream:    false,
		MaxTokens: c.maxTokens,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai marshal: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := httpclient.DoWithRetry(c.client, req, httpclient.DefaultRetry)
	if err != nil {
		return "", fmt.Errorf("openai post: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return "", fmt.Errorf("openai read: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", parseAPIError("openai", resp.StatusCode, string(raw))
	}

	var result openAIResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("openai decode: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", &AIError{Provider: "openai", Kind: ErrKindEmptyResponse, Message: "no choices in response"}
	}

	if result.Choices[0].Message.Content == "" {
		return "", &AIError{Provider: "openai", Kind: ErrKindEmptyResponse, Message: "empty content in response"}
	}

	return result.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) StreamGenerate(ctx context.Context, prompt string, onToken func(string)) (string, error) {
	body := openAIRequest{
		Model:     c.model,
		Messages:  []Message{{Role: "user", Content: prompt}},
		Stream:    true,
		MaxTokens: c.maxTokens,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai marshal: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	text, err := streamSSEWithRetry(req, onToken, func(line string) (string, bool) {
		if !strings.HasPrefix(line, "data: ") {
			return "", false
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return "", true
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return "", false
		}
		var sb strings.Builder
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				sb.WriteString(choice.Delta.Content)
			}
		}
		return sb.String(), false
	}, streamOptions{maxRetries: 2, provider: "openai"})
	if err != nil {
		return "", err
	}
	return text, nil
}
