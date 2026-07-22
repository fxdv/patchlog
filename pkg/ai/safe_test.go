package ai

import (
	"context"
	"strings"
	"testing"
)

type capturingClient struct {
	prompt string
}

func (c *capturingClient) Generate(_ context.Context, prompt string) (string, error) {
	c.prompt = prompt
	return "ok", nil
}

func (c *capturingClient) StreamGenerate(_ context.Context, prompt string, _ func(string)) (string, error) {
	c.prompt = prompt
	return "ok", nil
}

func TestSafeClientRedactsSecrets(t *testing.T) {
	next := &capturingClient{}
	client := WithInputPolicy(next, InputPolicy{MaxChars: 1000})
	prompt := "Authorization: Bearer ghp_abcdefghijklmnopqrstuvwxyz123456\npassword=hunter2"
	if _, err := client.Generate(context.Background(), prompt); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(next.prompt, "hunter2") || strings.Contains(next.prompt, "ghp_") {
		t.Fatalf("secret reached provider: %q", next.prompt)
	}
}

func TestRedactSecretsAdditionalCredentialForms(t *testing.T) {
	input := strings.Join([]string{
		"https://user:password@example.com/path",
		"sk-1234567890abcdefghijklmnop",
		"eyJabcdefghijk.abcdefghijk.abcdefghijk",
	}, "\n")
	redacted := RedactSecrets(input)
	for _, secret := range []string{"password@example", "sk-1234567890", "eyJabcdefghijk"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("secret fragment %q remained in %q", secret, redacted)
		}
	}
}

func TestSafeClientLimitsInputAfterRedaction(t *testing.T) {
	next := &capturingClient{}
	client := WithInputPolicy(next, InputPolicy{MaxChars: 8})
	if _, err := client.Generate(context.Background(), "more than eight characters"); err == nil {
		t.Fatal("expected oversized prompt to fail")
	}
	if next.prompt != "" {
		t.Fatal("oversized prompt reached provider")
	}
}
