package commit

import (
	"testing"
	"time"
)

func TestParseFeature(t *testing.T) {
	raw := RawCommit{
		Hash:      "abc123",
		Author:    "alice",
		Email:     "alice@example.com",
		Timestamp: time.Now(),
		Message:   "feat(api): add pagination to user list",
	}
	c := Parse(raw)
	if c.Type != "feat" {
		t.Errorf("expected type feat, got %s", c.Type)
	}
	if c.Scope != "api" {
		t.Errorf("expected scope api, got %s", c.Scope)
	}
	if c.Header != "add pagination to user list" {
		t.Errorf("unexpected header: %s", c.Header)
	}
	if c.Breaking {
		t.Error("should not be breaking")
	}
}

func TestParseFix(t *testing.T) {
	raw := RawCommit{Hash: "def456", Message: "fix: resolve null pointer in handler"}
	c := Parse(raw)
	if c.Type != "fix" {
		t.Errorf("expected type fix, got %s", c.Type)
	}
	if c.Scope != "" {
		t.Errorf("expected empty scope, got %s", c.Scope)
	}
}

func TestParseBreakingBang(t *testing.T) {
	raw := RawCommit{Hash: "ghi789", Message: "feat(api)!: rename /users to /accounts"}
	c := Parse(raw)
	if !c.Breaking {
		t.Error("expected breaking change")
	}
	if c.Type != "feat" {
		t.Errorf("expected type feat, got %s", c.Type)
	}
}

func TestParseBreakingFooter(t *testing.T) {
	raw := RawCommit{
		Hash:    "jkl012",
		Message: "feat(api): change response format\n\nDetails here.\n\nBREAKING CHANGE: response structure changed",
	}
	c := Parse(raw)
	if !c.Breaking {
		t.Error("expected breaking change from footer")
	}
}

func TestParseBodyAndFooter(t *testing.T) {
	raw := RawCommit{
		Hash:    "mno345",
		Message: "feat: add new feature\n\nThis is the body.\n\nCloses #123",
	}
	c := Parse(raw)
	if c.Body != "This is the body." {
		t.Errorf("unexpected body: %q", c.Body)
	}
	if c.Footer != "Closes #123" {
		t.Errorf("unexpected footer: %q", c.Footer)
	}
}

func TestParseNonConventional(t *testing.T) {
	raw := RawCommit{Hash: "pqr678", Message: "update something random"}
	c := Parse(raw)
	if c.Type != "other" {
		t.Errorf("expected type other, got %s", c.Type)
	}
	if c.Header != "update something random" {
		t.Errorf("unexpected header: %s", c.Header)
	}
}

func TestParseAllKnownTypes(t *testing.T) {
	types := []string{"feat", "fix", "build", "chore", "ci", "docs", "style", "refactor", "perf", "test"}
	for _, typ := range types {
		raw := RawCommit{Hash: "h", Message: typ + ": something"}
		c := Parse(raw)
		if c.Type != typ {
			t.Errorf("expected type %s, got %s", typ, c.Type)
		}
	}
}

func TestParseJiraKeys(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []string
	}{
		{"single key in header", "feat(api): add endpoint PROJ-123", []string{"PROJ-123"}},
		{"multiple keys", "feat: multi PROJ-123 and ENG-456", []string{"PROJ-123", "ENG-456"}},
		{"key in body", "feat: something\n\nRelates to PROJ-789", []string{"PROJ-789"}},
		{"duplicate keys deduplicated", "feat: PROJ-123 and PROJ-123 again", []string{"PROJ-123"}},
		{"no keys", "feat: no tickets here", nil},
		{"key with multi-digit number", "fix: resolve PROJ-12345", []string{"PROJ-12345"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := RawCommit{Hash: "h", Message: tt.message}
			c := Parse(raw)
			if len(c.JiraKeys) != len(tt.want) {
				t.Errorf("got %v, want %v", c.JiraKeys, tt.want)
				return
			}
			for i, k := range c.JiraKeys {
				if k != tt.want[i] {
					t.Errorf("key[%d]: got %s, want %s", i, k, tt.want[i])
				}
			}
		})
	}
}

func TestExtractJiraKeys(t *testing.T) {
	keys := extractJiraKeys("PROJ-123 and PROJ-456 and lowercase-1 and 123-ABC and PROJ-123")
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
	}
	if keys[0] != "PROJ-123" || keys[1] != "PROJ-456" {
		t.Errorf("unexpected keys: %v", keys)
	}
}

func TestParseScopeExtraction(t *testing.T) {
	tests := []struct {
		msg    string
		scope  string
		header string
	}{
		{"feat(api): add thing", "api", "add thing"},
		{"fix(ui): fix bug", "ui", "fix bug"},
		{"feat: no scope", "", "no scope"},
		{"refactor(core): optimize", "core", "optimize"},
	}
	for _, tt := range tests {
		c := Parse(RawCommit{Hash: "h", Message: tt.msg})
		if c.Scope != tt.scope {
			t.Errorf("scope: got %q, want %q", c.Scope, tt.scope)
		}
		if c.Header != tt.header {
			t.Errorf("header: got %q, want %q", c.Header, tt.header)
		}
	}
}
