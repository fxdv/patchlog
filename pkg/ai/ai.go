// Package ai provides AI provider clients (Ollama, OpenAI, Anthropic) for
// prose generation, description enhancement, and metric interpretation.
package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

// Tone controls the audience and style of AI-generated prose.
type Tone string

const (
	ToneDev      Tone = "dev"
	ToneCustomer Tone = "customer"
	ToneExec     Tone = "exec"
)

// Config holds AI provider connection settings.
type Config struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	MaxTokens int    `yaml:"max_tokens"`
}

func ParseTone(s string) (Tone, error) {
	switch s {
	case "dev":
		return ToneDev, nil
	case "customer":
		return ToneCustomer, nil
	case "exec":
		return ToneExec, nil
	default:
		return ToneDev, fmt.Errorf("unknown tone %q, use dev/customer/exec", s)
	}
}

// Client is the interface for AI text generation providers.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
	StreamGenerate(ctx context.Context, prompt string, onToken func(string)) (string, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "", "ollama":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://localhost:11434"
		}
		if cfg.Model == "" {
			cfg.Model = "llama3.2"
		}
		return &OllamaClient{baseURL: cfg.BaseURL, model: cfg.Model, maxTokens: cfg.MaxTokens, client: httpclient.Default()}, nil
	case "openai":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
		if cfg.Model == "" {
			cfg.Model = "gpt-4o-mini"
		}
		return &OpenAIClient{baseURL: cfg.BaseURL, model: cfg.Model, apiKey: cfg.APIKey, maxTokens: cfg.MaxTokens, client: httpclient.Default()}, nil
	case "anthropic":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.anthropic.com/v1"
		}
		if cfg.Model == "" {
			cfg.Model = "claude-3-haiku-20240307"
		}
		if cfg.APIKey == "" {
			return nil, errors.New("anthropic requires api_key in config")
		}
		maxTokens := cfg.MaxTokens
		if maxTokens == 0 {
			maxTokens = 4096
		}
		return &AnthropicClient{baseURL: cfg.BaseURL, model: cfg.Model, apiKey: cfg.APIKey, maxTokens: maxTokens, client: httpclient.Default()}, nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q", cfg.Provider)
	}
}
