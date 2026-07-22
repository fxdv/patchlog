package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// InputPolicy is enforced immediately before any prompt leaves the process.
type InputPolicy struct {
	MaxChars int
}

type safeClient struct {
	next   Client
	policy InputPolicy
}

var secretPatterns = []struct {
	re          *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?is)-----BEGIN [^-\r\n]*PRIVATE KEY-----.*?-----END [^-\r\n]*PRIVATE KEY-----`), `[REDACTED PRIVATE KEY]`},
	{regexp.MustCompile(`(?i)(https?://[^\s/:@]+:)[^\s/@]+@`), `${1}[REDACTED]@`},
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{12,}`), `Bearer [REDACTED]`},
	{regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|glpat-[A-Za-z0-9_-]{12,})\b`), `[REDACTED TOKEN]`},
	{regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|AIza[0-9A-Za-z_-]{20,}|xox[baprs]-[0-9A-Za-z-]{10,})\b`), `[REDACTED TOKEN]`},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), `[REDACTED AWS KEY]`},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), `[REDACTED JWT]`},
	{regexp.MustCompile(`(?im)(\b(?:api[_-]?key|api[_-]?token|access[_-]?token|client[_-]?secret|password|passwd|secret)\b\s*[:=]\s*)[^\s#;,]+`), `${1}[REDACTED]`},
}

// RedactSecrets removes common credential formats from arbitrary AI input.
func RedactSecrets(input string) string {
	redacted := input
	for _, pattern := range secretPatterns {
		redacted = pattern.re.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}

// WithInputPolicy wraps a provider so every generation path shares the same
// redaction and volume controls.
func WithInputPolicy(client Client, policy InputPolicy) Client {
	if client == nil {
		return nil
	}
	return &safeClient{next: client, policy: policy}
}

func (c *safeClient) sanitize(prompt string) (string, error) {
	prompt = RedactSecrets(prompt)
	if c.policy.MaxChars > 0 && len(prompt) > c.policy.MaxChars {
		return "", fmt.Errorf("AI input exceeds configured ai.max_input_chars (%d > %d); narrow the release range or exclusions", len(prompt), c.policy.MaxChars)
	}
	return prompt, nil
}

func (c *safeClient) Generate(ctx context.Context, prompt string) (string, error) {
	safePrompt, err := c.sanitize(prompt)
	if err != nil {
		return "", err
	}
	return c.next.Generate(ctx, safePrompt)
}

func (c *safeClient) StreamGenerate(ctx context.Context, prompt string, onToken func(string)) (string, error) {
	safePrompt, err := c.sanitize(prompt)
	if err != nil {
		return "", err
	}
	return c.next.StreamGenerate(ctx, safePrompt, onToken)
}

// Disclosure describes the outbound AI data boundary without exposing a key.
func Disclosure(purpose, endpoint string, maxChars int, exclusions []string) string {
	details := "commit text, linked issue text, metrics, and selected code diffs used by the requested AI feature"
	if len(exclusions) == 0 {
		return fmt.Sprintf("AI data disclosure (%s): sends redacted %s to %s; request limit %d characters", purpose, details, endpoint, maxChars)
	}
	return fmt.Sprintf("AI data disclosure (%s): sends redacted %s to %s; excludes %s; request limit %d characters", purpose, details, endpoint, strings.Join(exclusions, ", "), maxChars)
}
