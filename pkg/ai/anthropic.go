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

type AnthropicClient struct {
	baseURL   string
	model     string
	apiKey    string
	maxTokens int
	client    *http.Client
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type anthropicStreamEvent struct {
	Type  string               `json:"type"`
	Delta anthropicStreamDelta `json:"delta"`
}

type anthropicStreamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *AnthropicClient) Generate(ctx context.Context, prompt string) (string, error) {
	body := anthropicRequest{
		Model: c.model,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: c.maxTokens,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("anthropic marshal: %w", err)
	}

	url := c.baseURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpclient.DoWithRetry(c.client, req, httpclient.DefaultRetry)
	if err != nil {
		return "", fmt.Errorf("anthropic post: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return "", fmt.Errorf("anthropic read: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", parseAPIError("anthropic", resp.StatusCode, string(raw))
	}

	var result anthropicResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("anthropic decode: %w", err)
	}

	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	if text == "" {
		return "", &AIError{Provider: "anthropic", Kind: ErrKindEmptyResponse, Message: "empty response from model"}
	}

	return text, nil
}

func (c *AnthropicClient) StreamGenerate(ctx context.Context, prompt string, onToken func(string)) (string, error) {
	body := anthropicRequest{
		Model: c.model,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: c.maxTokens,
		Stream:    true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("anthropic marshal: %w", err)
	}

	url := c.baseURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	text, err := streamSSEWithRetry(req, onToken, func(line string) (string, bool) {
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			if eventType == "message_stop" {
				return "", true
			}
			return "", false
		}
		if !strings.HasPrefix(line, "data: ") {
			return "", false
		}
		payload := strings.TrimPrefix(line, "data: ")
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return "", false
		}
		if event.Type == "content_block_delta" {
			return event.Delta.Text, false
		}
		return "", false
	}, streamOptions{maxRetries: 2, provider: "anthropic"})
	if err != nil {
		return "", err
	}
	return text, nil
}
