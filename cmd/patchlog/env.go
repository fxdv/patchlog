package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/fxdv/patchlog/internal/atomicfile"
	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/config"
)

func defaultConfigPath() string {
	if path := os.Getenv("PATCHLOG_CONFIG"); path != "" {
		return path
	}
	return "patchlog.yaml"
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := atomicfile.Write(path, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func resolveEnvVars(cfg *config.Config) {
	cfg.AI.APIKey = expandEnv(cfg.AI.APIKey)
	cfg.AI.BaseURL = expandEnv(cfg.AI.BaseURL)
	cfg.Provider.Token = expandEnv(cfg.Provider.Token)
	cfg.Provider.BaseURL = expandEnv(cfg.Provider.BaseURL)
	cfg.Jira.APIToken = expandEnv(cfg.Jira.APIToken)
	cfg.Jira.BaseURL = expandEnv(cfg.Jira.BaseURL)
	cfg.Confluence.APIToken = expandEnv(cfg.Confluence.APIToken)
	cfg.Confluence.BaseURL = expandEnv(cfg.Confluence.BaseURL)
}

func newAIClient(cfg config.Config) (ai.Client, error) {
	client, err := ai.NewClient(ai.Config{
		Provider:  cfg.AI.Provider,
		Model:     cfg.AI.Model,
		APIKey:    cfg.AI.APIKey,
		BaseURL:   cfg.AI.BaseURL,
		MaxTokens: cfg.AI.MaxTokens,
	})
	if err != nil {
		return nil, err
	}
	return ai.WithInputPolicy(client, ai.InputPolicy{MaxChars: cfg.AI.MaxInputChars}), nil
}

var disclosedAIEndpoints sync.Map

func newAIClientOrWarn(cfg config.Config, purpose string, quiet bool) ai.Client {
	client, err := newAIClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: AI client for %s unavailable (%v), skipping\n", purpose, err)
		return nil
	}
	endpoint := effectiveAIEndpoint(cfg.AI)
	if !isLocalAIEndpoint(endpoint) {
		key := purpose + "\x00" + endpoint
		if _, loaded := disclosedAIEndpoints.LoadOrStore(key, struct{}{}); !loaded {
			fmt.Fprintln(os.Stderr, ai.Disclosure(purpose, endpoint, cfg.AI.MaxInputChars, cfg.AI.ExcludeFiles))
		}
	}
	return client
}

func effectiveAIEndpoint(cfg config.AIConfig) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	switch cfg.Provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	default:
		return "http://localhost:11434"
	}
}

func isLocalAIEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func expandEnv(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "$$") {
		return s[1:]
	}
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		inner := s[2 : len(s)-1]
		if idx := strings.Index(inner, ":-"); idx >= 0 {
			name := inner[:idx]
			def := inner[idx+2:]
			if v, ok := os.LookupEnv(name); ok && v != "" {
				return v
			}
			return def
		}
		return os.Getenv(inner)
	}
	if strings.HasPrefix(s, "$") {
		return os.Getenv(s[1:])
	}
	return s
}
