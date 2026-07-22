package infer

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
)

func TestParseResponse(t *testing.T) {
	resp := `[
		{"hash": "abc1234", "type": "feat", "confidence": 0.9},
		{"hash": "def5678", "type": "fix", "confidence": 0.8}
	]`
	results := parseResponse(resp)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Hash != "abc1234" || results[0].Type != "feat" {
		t.Errorf("result[0]: got %+v", results[0])
	}
	if results[1].Hash != "def5678" || results[1].Type != "fix" {
		t.Errorf("result[1]: got %+v", results[1])
	}
}

func TestParseResponseWithMarkdown(t *testing.T) {
	resp := "```json\n[{\"hash\": \"abc\", \"type\": \"fix\", \"confidence\": 0.7}]\n```"
	results := parseResponse(resp)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != "fix" {
		t.Errorf("expected fix, got %s", results[0].Type)
	}
}

func TestParseResponseGarbage(t *testing.T) {
	results := parseResponse("not json at all")
	if results != nil {
		t.Errorf("expected nil for garbage, got %v", results)
	}
}

func TestParseResponseEmpty(t *testing.T) {
	results := parseResponse("")
	if results != nil {
		t.Errorf("expected nil for empty, got %v", results)
	}
}

func TestBuildPrompt(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc1234567", RawHeader: "обновил графики", Body: "changed chart components", ChangedFiles: 5, JiraKeys: []string{"KEXP-123"}},
	}
	prompt := buildPrompt(commits)
	if !contains(prompt, "abc1234") {
		t.Error("prompt should contain short hash")
	}
	if !contains(prompt, "обновил графики") {
		t.Error("prompt should contain commit message")
	}
	if !contains(prompt, "KEXP-123") {
		t.Error("prompt should contain Jira key")
	}
	if !contains(prompt, "files_changed: 5") {
		t.Error("prompt should contain file count")
	}
	if !contains(prompt, "feat") {
		t.Error("prompt should list valid types")
	}
}

func TestValidTypes(t *testing.T) {
	valid := []string{"feat", "fix", "perf", "refactor", "docs", "test", "style", "ci", "chore", "build", "revert"}
	for _, v := range valid {
		if !validTypes[v] {
			t.Errorf("%s should be valid type", v)
		}
	}
	if validTypes["other"] {
		t.Error("other should not be a valid inference target")
	}
	if validTypes["invalid"] {
		t.Error("invalid should not be valid")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
