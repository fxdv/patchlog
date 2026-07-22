package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.AI.Provider != "ollama" {
		t.Errorf("default AI provider: got %q, want ollama", cfg.AI.Provider)
	}
	if cfg.Author.Show != true {
		t.Error("default Author.Show should be true")
	}
	if cfg.Bump.AutoDetect != true {
		t.Error("default Bump.AutoDetect should be true")
	}
	if _, ok := cfg.Sections["feat"]; !ok {
		t.Error("default sections missing feat")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("patchlog.yaml")
	if err != nil {
		t.Errorf("expected no error for missing default config, got %v", err)
	}
	if cfg.AI.Provider != "ollama" {
		t.Error("should return defaults for missing file")
	}
}

func TestLoadMissingCustomPath(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
	if cfg.AI.Provider != "ollama" {
		t.Error("should return defaults for missing file")
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("repo: myorg/myrepo\nai:\n  provider: openai\n  model: gpt-4\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repo != "myorg/myrepo" {
		t.Errorf("repo: got %q, want myorg/myrepo", cfg.Repo)
	}
	if cfg.AI.Provider != "openai" {
		t.Errorf("AI provider: got %q, want openai", cfg.AI.Provider)
	}
	if cfg.AI.Model != "gpt-4" {
		t.Errorf("AI model: got %q, want gpt-4", cfg.AI.Model)
	}
}

func TestLoadJiraConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("jira:\n  base_url: https://example.atlassian.net\n  email: dev@example.com\n  api_token: secret\n  project_key: PROJ\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jira.BaseURL != "https://example.atlassian.net" {
		t.Errorf("Jira base_url: got %q", cfg.Jira.BaseURL)
	}
	if cfg.Jira.ProjectKey != "PROJ" {
		t.Errorf("Jira project_key: got %q", cfg.Jira.ProjectKey)
	}
}

func TestLoadConfluenceConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("confluence:\n  space_key: ENG\n  parent_page_id: \"123456\"\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Confluence.SpaceKey != "ENG" {
		t.Errorf("Confluence space_key: got %q", cfg.Confluence.SpaceKey)
	}
	if cfg.Confluence.ParentPageID != "123456" {
		t.Errorf("Confluence parent_page_id: got %q", cfg.Confluence.ParentPageID)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("sections:\n  feat: New Features\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sections["feat"] != "New Features" {
		t.Errorf("overridden feat section: got %q", cfg.Sections["feat"])
	}
	if _, ok := cfg.Sections["fix"]; !ok {
		t.Error("default fix section should still be present")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("ai:\n  provder: openai\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected misspelled field to fail strict YAML decoding")
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("repo: first/repo\n---\nrepo: second/repo\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected multiple YAML documents to fail")
	}
}

func TestDefaultChangelogConfig(t *testing.T) {
	cfg := Default()
	if cfg.Changelog.Accumulate != false {
		t.Error("default Accumulate should be false")
	}
	if cfg.Changelog.Destination != "md" {
		t.Errorf("default destination: got %q, want md", cfg.Changelog.Destination)
	}
	if cfg.Changelog.File != "CHANGELOG.md" {
		t.Errorf("default file: got %q", cfg.Changelog.File)
	}
	if cfg.Changelog.Title != "Changelog" {
		t.Errorf("default title: got %q", cfg.Changelog.Title)
	}
	if cfg.Changelog.Slug != "changelog" {
		t.Errorf("default slug: got %q", cfg.Changelog.Slug)
	}
	if cfg.Changelog.Emojis != nil {
		t.Error("default Emojis should be nil (use runtime default)")
	}
}

func TestLoadChangelogConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("changelog:\n  accumulate: true\n  destination: wiki\n  title: Release Notes\n  slug: release-notes\n  emojis: false\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Changelog.Accumulate {
		t.Error("accumulate should be true")
	}
	if cfg.Changelog.Destination != "wiki" {
		t.Errorf("destination: got %q, want wiki", cfg.Changelog.Destination)
	}
	if cfg.Changelog.Title != "Release Notes" {
		t.Errorf("title: got %q", cfg.Changelog.Title)
	}
	if cfg.Changelog.Slug != "release-notes" {
		t.Errorf("slug: got %q", cfg.Changelog.Slug)
	}
	if cfg.Changelog.Emojis == nil || *cfg.Changelog.Emojis != false {
		t.Error("emojis should be false")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := Default()
	cfg.AI.Provider = "ollama"
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should not error: %v", err)
	}
}

func TestValidateInvalidAIProvider(t *testing.T) {
	cfg := Default()
	cfg.AI.Provider = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid AI provider")
	}
}

func TestValidateOpenAIRequiresAPIKey(t *testing.T) {
	cfg := Default()
	cfg.AI.Provider = "openai"
	cfg.AI.APIKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing openai api_key")
	}
}

func TestValidateInvalidProviderType(t *testing.T) {
	cfg := Default()
	cfg.Provider.Type = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid provider type")
	}
}

func TestValidateGiteaRequiresBaseURL(t *testing.T) {
	cfg := Default()
	cfg.Provider.Type = "gitea"
	cfg.Provider.BaseURL = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing gitea base_url")
	}
}

func TestValidateInvalidChangelogDestination(t *testing.T) {
	cfg := Default()
	cfg.Changelog.Destination = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid changelog destination")
	}
}

func TestValidateWikiRequiresGitLab(t *testing.T) {
	cfg := Default()
	cfg.Changelog.Destination = "wiki"
	cfg.Provider.Type = "github"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for wiki destination with non-gitlab provider")
	}
}

func TestValidateJiraRequiresToken(t *testing.T) {
	cfg := Default()
	cfg.Jira.BaseURL = "https://example.atlassian.net"
	cfg.Jira.APIToken = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for jira without api_token")
	}
}

func TestValidateRejectsCredentialsOverHTTP(t *testing.T) {
	cfg := Default()
	cfg.Provider.Type = "gitea"
	cfg.Provider.Repo = "org/repo"
	cfg.Provider.BaseURL = "http://git.example.com"
	cfg.Provider.Token = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected insecure credential transport to fail")
	}
}

func TestValidateAllowsExplicitInsecureCredentialOverride(t *testing.T) {
	cfg := Default()
	cfg.Provider.Type = "gitea"
	cfg.Provider.Repo = "org/repo"
	cfg.Provider.BaseURL = "http://git.example.com"
	cfg.Provider.Token = "secret"
	cfg.Security.AllowInsecureCredentials = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("explicit override should allow insecure transport: %v", err)
	}
}

func TestValidateAllowsLoopbackHTTP(t *testing.T) {
	cfg := Default()
	cfg.AI.Provider = "openai"
	cfg.AI.BaseURL = "http://127.0.0.1:8080/v1"
	cfg.AI.APIKey = "local-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("loopback HTTP should be allowed for local testing: %v", err)
	}
}

func TestValidateRejectsInheritedConfluenceCredentialOverHTTP(t *testing.T) {
	cfg := Default()
	cfg.Jira.BaseURL = "https://jira.example.com"
	cfg.Jira.APIToken = "jira-secret"
	cfg.Confluence.BaseURL = "http://confluence.example.com"
	cfg.Confluence.SpaceKey = "ENG"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected inherited Jira credential over insecure Confluence HTTP to fail")
	}
}

func TestValidateInvalidURL(t *testing.T) {
	cfg := Default()
	cfg.AI.BaseURL = "not-a-url"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid AI base_url")
	}
}

func TestValidateURLMissingScheme(t *testing.T) {
	cfg := Default()
	cfg.Jira.BaseURL = "example.atlassian.net"
	cfg.Jira.APIToken = "tok"
	cfg.Jira.Email = "dev@test.com"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for URL without scheme")
	}
}

func TestValidateNegativeMaxTokens(t *testing.T) {
	cfg := Default()
	cfg.AI.MaxTokens = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative max_tokens")
	}
}

func TestValidateNegativeMaxConcurrency(t *testing.T) {
	cfg := Default()
	cfg.Jira.MaxConcurrency = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative max_concurrency")
	}
}

func TestValidateEmptySectionHeading(t *testing.T) {
	cfg := Default()
	cfg.Sections["feat"] = "  "
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty section heading")
	}
}

func TestValidateInvalidIgnorePattern(t *testing.T) {
	cfg := Default()
	cfg.Ignore = []string{"[invalid"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid regex in ignore")
	}
}

func TestValidateProviderRequiresRepo(t *testing.T) {
	cfg := Default()
	cfg.Provider.Type = "github"
	cfg.Provider.Repo = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for provider without repo")
	}
}

func TestValidateNegativeClassifyThresholds(t *testing.T) {
	cfg := Default()
	cfg.Classify.LargeFeatureFiles = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative classify threshold")
	}
}

func TestValidateEmptyBumpFile(t *testing.T) {
	cfg := Default()
	cfg.Bump.Files = []string{"  "}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty bump file path")
	}
}

func TestLoadValidatedMissingFile(t *testing.T) {
	cfg, err := LoadValidated("patchlog.yaml")
	if err != nil {
		t.Errorf("expected no error for missing default config, got %v", err)
	}
	if cfg.AI.Provider != "ollama" {
		t.Error("should return defaults for missing file")
	}
}

func TestLoadValidatedInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("ai:\n  provider: invalid_provider\n")
	os.WriteFile(cfgPath, content, 0644)

	_, err := LoadValidated(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestLoadValidatedValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("ai:\n  provider: ollama\n  base_url: http://localhost:11434\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := LoadValidated(cfgPath)
	if err != nil {
		t.Fatalf("expected no error for valid config: %v", err)
	}
	if cfg.AI.Provider != "ollama" {
		t.Errorf("provider: got %q", cfg.AI.Provider)
	}
}

func TestDefaultDepsConfig(t *testing.T) {
	cfg := Default()
	if !cfg.Deps.FetchUpstream {
		t.Error("default Deps.FetchUpstream should be true")
	}
	if cfg.Deps.MaxDependencies != 10 {
		t.Errorf("default MaxDependencies: got %d, want 10", cfg.Deps.MaxDependencies)
	}
	if !cfg.Deps.GitHubReleases {
		t.Error("default Deps.GitHubReleases should be true")
	}
	if cfg.Deps.Registries["npm"] == "" {
		t.Error("default registries should include npm")
	}
}

func TestLoadDepsConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("deps:\n  enabled: true\n  fetch_upstream: false\n  max_dependencies: 5\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Deps.Enabled {
		t.Error("deps.enabled should be true")
	}
	if cfg.Deps.FetchUpstream {
		t.Error("deps.fetch_upstream should be false")
	}
	if cfg.Deps.MaxDependencies != 5 {
		t.Errorf("deps.max_dependencies: got %d, want 5", cfg.Deps.MaxDependencies)
	}
}

func TestValidateNegativeMaxDependencies(t *testing.T) {
	cfg := Default()
	cfg.Deps.MaxDependencies = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative deps.max_dependencies")
	}
}

func TestValidateInvalidDepsRegistryURL(t *testing.T) {
	cfg := Default()
	cfg.Deps.Registries["npm"] = "not-a-url"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid deps registry URL")
	}
}

func TestDefaultGateConfig(t *testing.T) {
	cfg := Default()
	if cfg.Gate.MinConventionalRatio != 0 {
		t.Errorf("default MinConventionalRatio: got %v, want 0", cfg.Gate.MinConventionalRatio)
	}
}

func TestLoadGateConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	content := []byte("gate:\n  enabled: true\n  min_conventional_ratio: 0.8\n")
	os.WriteFile(cfgPath, content, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Gate.Enabled {
		t.Error("gate.enabled should be true")
	}
	if cfg.Gate.MinConventionalRatio != 0.8 {
		t.Errorf("gate.min_conventional_ratio: got %v, want 0.8", cfg.Gate.MinConventionalRatio)
	}
}
