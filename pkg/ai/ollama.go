package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

type OllamaClient struct {
	baseURL   string
	model     string
	maxTokens int
	client    *http.Client
}

type ollamaChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  *ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

func (c *OllamaClient) options() *ollamaOptions {
	if c.maxTokens <= 0 {
		return nil
	}
	return &ollamaOptions{NumPredict: c.maxTokens}
}

type ollamaChatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	body := ollamaChatRequest{
		Model:    c.model,
		Messages: []Message{{Role: "user", Content: prompt}},
		Stream:   false,
		Options:  c.options(),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama marshal: %w", err)
	}

	url := c.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.client, req, httpclient.DefaultRetry)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return "", fmt.Errorf("ollama read: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", parseAPIError("ollama", resp.StatusCode, string(raw))
	}

	var result ollamaChatResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}

	if result.Message.Content == "" {
		return "", &AIError{Provider: "ollama", Kind: ErrKindEmptyResponse, Message: "empty response from model"}
	}

	return result.Message.Content, nil
}

func (c *OllamaClient) StreamGenerate(ctx context.Context, prompt string, onToken func(string)) (string, error) {
	body := ollamaChatRequest{
		Model:    c.model,
		Messages: []Message{{Role: "user", Content: prompt}},
		Stream:   true,
		Options:  c.options(),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama marshal: %w", err)
	}

	url := c.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	text, err := streamSSEWithRetry(req, onToken, func(line string) (string, bool) {
		if line == "" {
			return "", false
		}
		var chunk ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return "", false
		}
		return chunk.Message.Content, chunk.Done
	}, streamOptions{maxRetries: 2, provider: "ollama"})
	if err != nil {
		return "", err
	}
	return text, nil
}
