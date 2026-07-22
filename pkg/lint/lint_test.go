package lint

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
)

func TestLintConventionalCommit(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "feat", Scope: "api", Header: "add pagination to user list", RawHeader: "feat(api): add pagination to user list"},
	}
	r := Lint(commits)
	if r.Errors > 0 {
		t.Errorf("expected 0 errors for conventional commit, got %d", r.Errors)
	}
}

func TestLintNonConventional(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "other", Header: "updated stuff", RawHeader: "updated stuff"},
	}
	r := Lint(commits)
	if r.Errors == 0 {
		t.Error("expected error for non-conventional commit")
	}
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "conventional-format" {
			found = true
		}
	}
	if !found {
		t.Error("expected conventional-format error")
	}
}

func TestLintShortDescription(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "fix", Header: "x", RawHeader: "fix: x"},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "short-description" {
			found = true
		}
	}
	if !found {
		t.Error("expected short-description warning")
	}
}

func TestLintUppercaseDescription(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "feat", Header: "Add new feature", RawHeader: "feat: Add new feature"},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "capitalization" {
			found = true
		}
	}
	if !found {
		t.Error("expected capitalization info")
	}
}

func TestLintTrailingPeriod(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "fix", Header: "fix the bug.", RawHeader: "fix: fix the bug."},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "trailing-period" {
			found = true
		}
	}
	if !found {
		t.Error("expected trailing-period info")
	}
}

func TestLintMissingBody(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "feat", Header: "add complex feature", RawHeader: "feat: add complex feature"},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "missing-body" {
			found = true
		}
	}
	if !found {
		t.Error("expected missing-body info for feat without body")
	}
}

func TestLintBreakingNoExplanation(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "feat", Breaking: true, Header: "rename endpoint", RawHeader: "feat!: rename endpoint"},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "breaking-no-explanation" {
			found = true
		}
	}
	if !found {
		t.Error("expected breaking-no-explanation warning")
	}
}

func TestLintFixWithoutJira(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "fix", Header: "handle null pointer", RawHeader: "fix: handle null pointer"},
	}
	r := Lint(commits)
	found := false
	for _, issue := range r.Issues {
		if issue.Rule == "missing-jira-ref" {
			found = true
		}
	}
	if !found {
		t.Error("expected missing-jira-ref info")
	}
}

func TestLintCleanCommit(t *testing.T) {
	commits := []commit.Commit{
		{
			Hash:      "abc123",
			Type:      "fix",
			Scope:     "api",
			Header:    "handle null pointer in user service",
			RawHeader: "fix(api): handle null pointer in user service",
			Body:      "The user service would crash when processing requests with null fields.",
			JiraKeys:  []string{"PROJ-123"},
		},
	}
	r := Lint(commits)
	if r.Errors > 0 {
		t.Errorf("expected 0 errors for clean commit, got %d", r.Errors)
	}
}

func TestLintMultipleCommits(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "feat", Header: "add feature", RawHeader: "feat: add feature"},
		{Hash: "def456", Type: "other", Header: "updated stuff", RawHeader: "updated stuff"},
		{Hash: "ghi789", Type: "fix", Header: "fix bug", RawHeader: "fix: fix bug", JiraKeys: []string{"PROJ-1"}, Body: "explanation"},
	}
	r := Lint(commits)
	if r.CommitsChecked != 3 {
		t.Errorf("expected 3 commits checked, got %d", r.CommitsChecked)
	}
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
}

func TestFormatResult(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc1234567890", Type: "other", Header: "updated stuff", RawHeader: "updated stuff"},
	}
	r := Lint(commits)
	output := FormatResult(r)
	if output == "" {
		t.Error("format result should not be empty")
	}
}

func TestSeverityString(t *testing.T) {
	if SeverityError.String() != "error" {
		t.Error("error severity string mismatch")
	}
	if SeverityWarning.String() != "warning" {
		t.Error("warning severity string mismatch")
	}
	if SeverityInfo.String() != "info" {
		t.Error("info severity string mismatch")
	}
}
