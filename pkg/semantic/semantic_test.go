package semantic

import (
	"strings"
	"testing"
)

func TestTruncateDiff(t *testing.T) {
	diff := "line1\nline2\nline3\nline4\nline5\n"
	result := truncateDiff(diff, 15)
	if !strings.Contains(result, "truncated") {
		t.Error("should contain truncation marker")
	}
	if !strings.Contains(result, "line1") {
		t.Error("should contain first lines")
	}
}

func TestTruncateDiffShort(t *testing.T) {
	diff := "short diff"
	result := truncateDiff(diff, 100)
	if result != diff {
		t.Error("short diff should not be truncated")
	}
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("Features", []string{"add login", "add logout"}, "diff --git a/auth.go b/auth.go\n+func login() {}")
	if !strings.Contains(prompt, "Features") {
		t.Error("prompt should contain section heading")
	}
	if !strings.Contains(prompt, "add login") {
		t.Error("prompt should contain commit descriptions")
	}
	if !strings.Contains(prompt, "auth.go") {
		t.Error("prompt should contain diff content")
	}
	if !strings.Contains(prompt, "Semantic summary") {
		t.Error("prompt should end with summary instruction")
	}
}

func TestFormatMarkdownEmpty(t *testing.T) {
	result := FormatMarkdown(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatMarkdownWithSummaries(t *testing.T) {
	summaries := map[string]string{
		"Features":  "Added login and logout functionality",
		"Bug Fixes": "Fixed auth token expiry",
	}
	result := FormatMarkdown(summaries)
	if !strings.Contains(result, "## Semantic Summary") {
		t.Error("should contain heading")
	}
	if !strings.Contains(result, "Added login and logout") {
		t.Error("should contain feature summary")
	}
	if !strings.Contains(result, "Fixed auth token expiry") {
		t.Error("should contain fix summary")
	}
}
