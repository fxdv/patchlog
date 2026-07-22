package config

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	var parts []string
	for _, e := range ve {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "\n  ")
}

func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

func (c Config) Validate() error {
	var errs ValidationErrors

	if c.AI.Provider != "" {
		switch c.AI.Provider {
		case "ollama", "openai", "anthropic":
		default:
			errs = append(errs, ValidationError{
				Field:   "ai.provider",
				Message: fmt.Sprintf("must be ollama, openai, or anthropic (got %q)", c.AI.Provider),
			})
		}
	}

	if c.AI.Provider == "openai" && c.AI.APIKey == "" && c.AI.BaseURL == "" {
		errs = append(errs, ValidationError{
			Field:   "ai.api_key",
			Message: "required when ai.provider is openai (unless base_url is set for self-hosted)",
		})
	}
	if c.AI.Provider == "anthropic" && c.AI.APIKey == "" && c.AI.BaseURL == "" {
		errs = append(errs, ValidationError{
			Field:   "ai.api_key",
			Message: "required when ai.provider is anthropic (unless base_url is set for self-hosted)",
		})
	}

	if c.AI.MaxTokens < 0 {
		errs = append(errs, ValidationError{
			Field:   "ai.max_tokens",
			Message: "must be a positive integer",
		})
	}
	if c.AI.MaxInputChars <= 0 {
		errs = append(errs, ValidationError{
			Field:   "ai.max_input_chars",
			Message: "must be a positive integer",
		})
	}
	for i, pattern := range c.AI.ExcludeFiles {
		if strings.TrimSpace(pattern) == "" {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("ai.exclude_files[%d]", i), Message: "pattern must not be empty"})
			continue
		}
		if _, err := path.Match(pattern, "sample/path"); err != nil {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("ai.exclude_files[%d]", i), Message: fmt.Sprintf("invalid glob: %v", err)})
		}
	}

	if c.AI.BaseURL != "" {
		if err := validateURL("ai.base_url", c.AI.BaseURL); err != nil {
			errs = append(errs, *err)
		}
	}
	if err := validateCredentialTransport("ai.base_url", c.AI.BaseURL, c.AI.APIKey, c.Security.AllowInsecureCredentials); err != nil {
		errs = append(errs, *err)
	}

	if c.Provider.Type != "" {
		switch c.Provider.Type {
		case "github", "gitlab", "gitea":
		default:
			errs = append(errs, ValidationError{
				Field:   "provider.type",
				Message: fmt.Sprintf("must be github, gitlab, or gitea (got %q)", c.Provider.Type),
			})
		}
	}

	if c.Provider.Type == "gitea" && c.Provider.BaseURL == "" {
		errs = append(errs, ValidationError{
			Field:   "provider.base_url",
			Message: "required when provider.type is gitea",
		})
	}

	if c.Provider.BaseURL != "" {
		if err := validateURL("provider.base_url", c.Provider.BaseURL); err != nil {
			errs = append(errs, *err)
		}
	}
	if err := validateCredentialTransport("provider.base_url", c.Provider.BaseURL, c.Provider.Token, c.Security.AllowInsecureCredentials); err != nil {
		errs = append(errs, *err)
	}

	if c.Provider.Type != "" && c.Provider.Repo == "" {
		errs = append(errs, ValidationError{
			Field:   "provider.repo",
			Message: "required when provider.type is set",
		})
	}

	if c.Jira.BaseURL != "" {
		if err := validateURL("jira.base_url", c.Jira.BaseURL); err != nil {
			errs = append(errs, *err)
		}
		if c.Jira.APIToken == "" {
			errs = append(errs, ValidationError{
				Field:   "jira.api_token",
				Message: "required when jira.base_url is set",
			})
		}
	}
	if err := validateCredentialTransport("jira.base_url", c.Jira.BaseURL, c.Jira.APIToken, c.Security.AllowInsecureCredentials); err != nil {
		errs = append(errs, *err)
	}

	if c.Jira.MaxConcurrency < 0 {
		errs = append(errs, ValidationError{
			Field:   "jira.max_concurrency",
			Message: "must be a positive integer",
		})
	}

	if c.Confluence.SpaceKey != "" {
		if c.Confluence.BaseURL == "" && c.Jira.BaseURL == "" {
			errs = append(errs, ValidationError{
				Field:   "confluence.base_url",
				Message: "required when confluence.space_key is set (or configure jira.base_url for fallback)",
			})
		}
		if c.Confluence.APIToken == "" && c.Jira.APIToken == "" {
			errs = append(errs, ValidationError{
				Field:   "confluence.api_token",
				Message: "required when confluence.space_key is set (or configure jira.api_token for fallback)",
			})
		}
	}

	if c.Confluence.BaseURL != "" {
		if err := validateURL("confluence.base_url", c.Confluence.BaseURL); err != nil {
			errs = append(errs, *err)
		}
	}
	effectiveConfluenceURL := c.Confluence.BaseURL
	if effectiveConfluenceURL == "" {
		effectiveConfluenceURL = c.Jira.BaseURL
	}
	effectiveConfluenceToken := c.Confluence.APIToken
	if effectiveConfluenceToken == "" {
		effectiveConfluenceToken = c.Jira.APIToken
	}
	if err := validateCredentialTransport("confluence.base_url", effectiveConfluenceURL, effectiveConfluenceToken, c.Security.AllowInsecureCredentials); err != nil {
		errs = append(errs, *err)
	}

	if c.Changelog.Destination != "" {
		switch c.Changelog.Destination {
		case "md", "wiki", "confluence":
		default:
			errs = append(errs, ValidationError{
				Field:   "changelog.destination",
				Message: fmt.Sprintf("must be md, wiki, or confluence (got %q)", c.Changelog.Destination),
			})
		}
	}

	if c.Changelog.Destination == "wiki" && c.Provider.Type != "gitlab" && c.Provider.Type != "" {
		errs = append(errs, ValidationError{
			Field:   "changelog.destination",
			Message: "wiki destination requires provider.type to be gitlab",
		})
	}

	for typ, heading := range c.Sections {
		if strings.TrimSpace(typ) == "" {
			errs = append(errs, ValidationError{
				Field:   "sections",
				Message: "section type key must not be empty",
			})
		}
		if strings.TrimSpace(heading) == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("sections[%s]", typ),
				Message: "heading must not be empty",
			})
		}
	}

	for i, p := range c.Ignore {
		if _, err := regexp.Compile(p); err != nil {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("ignore[%d]", i),
				Message: fmt.Sprintf("invalid regex: %v", err),
			})
		}
	}

	for i, f := range c.Bump.Files {
		if strings.TrimSpace(f) == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("bump.files[%d]", i),
				Message: "file path must not be empty",
			})
		}
	}

	if c.Classify.LargeFeatureFiles < 0 || c.Classify.LargeFixFiles < 0 || c.Classify.LargeUnknownFiles < 0 {
		errs = append(errs, ValidationError{
			Field:   "classify",
			Message: "threshold values must be non-negative",
		})
	}

	if c.Trends.Thresholds.ReleaseCommitSpanWarning > 0 && c.Trends.Thresholds.ReleaseCommitSpanCritical > 0 {
		if c.Trends.Thresholds.ReleaseCommitSpanWarning >= c.Trends.Thresholds.ReleaseCommitSpanCritical {
			errs = append(errs, ValidationError{
				Field:   "trends.thresholds.release_commit_span_warning",
				Message: "must be less than release_commit_span_critical",
			})
		}
	}

	if c.Trends.Thresholds.TechDebtWarning > 0 && c.Trends.Thresholds.TechDebtCritical > 0 {
		if c.Trends.Thresholds.TechDebtWarning >= c.Trends.Thresholds.TechDebtCritical {
			errs = append(errs, ValidationError{
				Field:   "trends.thresholds.tech_debt_warning",
				Message: "must be less than tech_debt_critical",
			})
		}
	}

	if c.Trends.Thresholds.ReleaseContributionConcentrationMin < 0 {
		errs = append(errs, ValidationError{
			Field:   "trends.thresholds.release_contribution_concentration_min",
			Message: "must be non-negative",
		})
	}
	if c.Trends.Count < 0 {
		errs = append(errs, ValidationError{Field: "trends.count", Message: "must be non-negative"})
	}

	if c.Deps.MaxDependencies < 0 {
		errs = append(errs, ValidationError{
			Field:   "deps.max_dependencies",
			Message: "must be non-negative",
		})
	}

	allowedTypes := map[string]bool{
		"feat": true, "fix": true, "perf": true, "refactor": true,
		"docs": true, "test": true, "style": true, "ci": true, "chore": true,
	}
	for i, commitType := range c.Infer.Types {
		if !allowedTypes[commitType] {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("infer.types[%d]", i), Message: fmt.Sprintf("unsupported commit type %q", commitType)})
		}
	}
	if c.Semantic.MaxDiffChars < 0 {
		errs = append(errs, ValidationError{Field: "semantic.max_diff_chars", Message: "must be non-negative"})
	}

	for name, u := range c.Deps.Registries {
		if err := validateURL(fmt.Sprintf("deps.registries.%s", name), u); err != nil {
			errs = append(errs, *err)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateURL(field, raw string) *ValidationError {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must start with http:// or https:// (got %q)", raw),
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("invalid URL: %v", err),
		}
	}
	if u.Host == "" {
		return &ValidationError{
			Field:   field,
			Message: "URL must have a host",
		}
	}
	return nil
}

func validateCredentialTransport(field, rawURL, credential string, allowInsecure bool) *ValidationError {
	if credential == "" || rawURL == "" || allowInsecure || strings.HasPrefix(rawURL, "$") {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return nil // validateURL reports the structural error.
	}
	if u.Scheme == "https" || isLoopbackHost(u.Hostname()) {
		return nil
	}
	return &ValidationError{
		Field:   field,
		Message: "refuses to send credentials over insecure HTTP; use HTTPS or explicitly set security.allow_insecure_credentials",
	}
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
