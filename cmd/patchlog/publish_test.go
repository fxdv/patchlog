package main

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/config"
)

func TestResolveConfluenceConfigFullFallback(t *testing.T) {
	cfg := config.Config{
		Jira: config.JiraConfig{
			BaseURL:  "https://myorg.atlassian.net",
			Email:    "dev@myorg.com",
			APIToken: "token123",
		},
	}
	c := resolveConfluenceConfig(cfg)
	if c.BaseURL != "https://myorg.atlassian.net" {
		t.Errorf("base_url: got %q", c.BaseURL)
	}
	if c.Email != "dev@myorg.com" {
		t.Errorf("email: got %q", c.Email)
	}
	if c.APIToken != "token123" {
		t.Errorf("api_token: got %q", c.APIToken)
	}
}

func TestResolveConfluenceConfigNoFallback(t *testing.T) {
	cfg := config.Config{
		Confluence: config.ConfluenceConfig{
			BaseURL:  "https://confluence.example.com",
			Email:    "confluence@example.com",
			APIToken: "ctoken",
		},
		Jira: config.JiraConfig{
			BaseURL:  "https://jira.example.com",
			Email:    "jira@example.com",
			APIToken: "jtoken",
		},
	}
	c := resolveConfluenceConfig(cfg)
	if c.BaseURL != "https://confluence.example.com" {
		t.Errorf("base_url: got %q", c.BaseURL)
	}
	if c.Email != "confluence@example.com" {
		t.Errorf("email: got %q", c.Email)
	}
	if c.APIToken != "ctoken" {
		t.Errorf("api_token: got %q", c.APIToken)
	}
}

func TestResolveConfluenceConfigPartialFallback(t *testing.T) {
	cfg := config.Config{
		Confluence: config.ConfluenceConfig{
			BaseURL: "https://confluence.example.com",
		},
		Jira: config.JiraConfig{
			BaseURL:  "https://jira.example.com",
			Email:    "jira@example.com",
			APIToken: "jtoken",
		},
	}
	c := resolveConfluenceConfig(cfg)
	if c.BaseURL != "https://confluence.example.com" {
		t.Errorf("base_url should use confluence value: got %q", c.BaseURL)
	}
	if c.Email != "jira@example.com" {
		t.Errorf("email should fall back to jira: got %q", c.Email)
	}
	if c.APIToken != "jtoken" {
		t.Errorf("api_token should fall back to jira: got %q", c.APIToken)
	}
}

func TestResolveConfluenceConfigNoJira(t *testing.T) {
	cfg := config.Config{}
	c := resolveConfluenceConfig(cfg)
	if c.BaseURL != "" || c.Email != "" || c.APIToken != "" {
		t.Errorf("expected all empty when no config: %+v", c)
	}
}
