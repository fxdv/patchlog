package multirepo

import (
	"testing"
)

func TestRepoName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/projects/myapp", "myapp"},
		{"./repos/backend", "backend"},
		{"frontend/", "frontend"},
		{"/tmp/repo", "repo"},
	}
	for _, tt := range tests {
		got := repoName(tt.path)
		if got != tt.want {
			t.Errorf("repoName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFormatAggregateMarkdownEmpty(t *testing.T) {
	output := FormatAggregateMarkdown(nil, true, false)
	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestFormatAggregateMarkdownWithErrors(t *testing.T) {
	results := []RepoResult{
		{Name: "repo-a", Path: "/path/a", Error: errFake("fetch failed")},
	}
	output := FormatAggregateMarkdown(results, true, false)
	if len(output) == 0 {
		t.Error("expected non-empty output with errors")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
