package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
)

func TestAuditMissingChangelog(t *testing.T) {
	dir := t.TempDir()
	changelogPath := filepath.Join(dir, "CHANGELOG.md")

	commits := []commit.Commit{
		{Hash: "abc123def456", Type: "feat", Header: "add user authentication", RawHeader: "feat: add user authentication"},
	}
	result := AuditResult{
		ChangelogFile: changelogPath,
		TotalCommits:  len(commits),
	}

	result.MissingEntries = makeMissingFromAll(commits)
	if len(result.MissingEntries) != 1 {
		t.Errorf("expected 1 missing entry, got %d", len(result.MissingEntries))
	}
}

func TestAuditSkipsNonFeatureCommits(t *testing.T) {
	commits := []commit.Commit{
		{Hash: "abc123", Type: "docs", Header: "update readme", RawHeader: "docs: update readme"},
		{Hash: "def456", Type: "chore", Header: "update deps", RawHeader: "chore: update deps"},
	}
	missing := makeMissingFromAll(commits)
	if len(missing) != 0 {
		t.Errorf("expected 0 missing for skip types, got %d", len(missing))
	}
}

func TestMatchCommitInChangelog(t *testing.T) {
	c := commit.Commit{
		Hash:      "abc123def456",
		Type:      "feat",
		Header:    "add pagination to user list",
		RawHeader: "feat(api): add pagination to user list",
		JiraKeys:  []string{"PROJ-123"},
	}

	changelog := "# v1.0.0\n\n## Features\n\n- add pagination to user list (#42) [PROJ-123]\n"

	if !matchCommitInChangelog(c, changelog) {
		t.Error("expected commit to be found in changelog")
	}
}

func TestMatchCommitByJiraKey(t *testing.T) {
	c := commit.Commit{
		Hash:     "abc123def456",
		Type:     "fix",
		Header:   "completely different description",
		JiraKeys: []string{"ENG-456"},
	}

	changelog := "# v1.0.0\n\n- something else [ENG-456]\n"

	if !matchCommitInChangelog(c, changelog) {
		t.Error("expected commit to be found by Jira key")
	}
}

func TestMatchCommitByHash(t *testing.T) {
	c := commit.Commit{
		Hash:   "abc123def456789",
		Type:   "feat",
		Header: "completely different thing",
	}

	changelog := "# v1.0.0\n\n- something (abc123de)\n"

	if !matchCommitInChangelog(c, changelog) {
		t.Error("expected commit to be found by hash prefix")
	}
}

func TestMatchCommitNotFound(t *testing.T) {
	c := commit.Commit{
		Hash:   "abc123def456",
		Type:   "feat",
		Header: "add quantum computing support",
	}

	changelog := "# v1.0.0\n\n- add login page\n- fix bug in parser\n"

	if matchCommitInChangelog(c, changelog) {
		t.Error("expected commit to NOT be found")
	}
}

func TestCountChangelogEntries(t *testing.T) {
	changelog := "# v1.0.0\n\n## Features\n\n- add feature a\n- add feature b\n\n## Bug Fixes\n\n- fix bug c\n"
	count := countChangelogEntries(changelog)
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}
}

func TestIsPlaceholderEntry(t *testing.T) {
	placeholders := []string{"todo", "TODO", "TBD", "fixme", "placeholder", "coming soon", "n/a"}
	for _, p := range placeholders {
		if !isPlaceholderEntry(p) {
			t.Errorf("expected %q to be a placeholder", p)
		}
	}
	if isPlaceholderEntry("add real feature") {
		t.Error("expected real entry to not be placeholder")
	}
}

func TestExtractKeywords(t *testing.T) {
	kw := extractKeywords("add pagination to user list endpoint")
	if len(kw) < 3 {
		t.Errorf("expected at least 3 keywords, got %d: %v", len(kw), kw)
	}

	hasPagination := false
	for _, k := range kw {
		if k == "pagination" {
			hasPagination = true
		}
	}
	if !hasPagination {
		t.Error("expected 'pagination' in keywords")
	}
}

func TestFormatResult(t *testing.T) {
	result := AuditResult{
		ChangelogFile:    "CHANGELOG.md",
		TotalCommits:     10,
		ChangelogEntries: 8,
	}
	output := FormatResult(result)
	if output == "" {
		t.Error("format result should not be empty")
	}
}

func TestFormatResultWithIssues(t *testing.T) {
	result := AuditResult{
		ChangelogFile:    "CHANGELOG.md",
		TotalCommits:     5,
		ChangelogEntries: 3,
		MissingEntries: []MissingEntry{
			{Commit: commit.Commit{Hash: "abc123", RawHeader: "feat: add thing"}, Reason: "not found"},
		},
	}
	output := FormatResult(result)
	if output == "" {
		t.Error("format result should not be empty")
	}
}

func TestAuditWithRealChangelog(t *testing.T) {
	dir := t.TempDir()
	changelogPath := filepath.Join(dir, "CHANGELOG.md")

	changelog := `# Changelog

## [1.0.0]

### Features

- add user authentication
- add pagination to user list
`
	os.WriteFile(changelogPath, []byte(changelog), 0644)

	commits := []commit.Commit{
		{Hash: "aaa111", Type: "feat", Header: "add user authentication", RawHeader: "feat: add user authentication"},
		{Hash: "bbb222", Type: "feat", Header: "add pagination to user list", RawHeader: "feat: add pagination to user list"},
		{Hash: "ccc333", Type: "feat", Header: "add dark mode", RawHeader: "feat: add dark mode"},
	}

	data, _ := os.ReadFile(changelogPath)
	cl := string(data)

	var missing []MissingEntry
	skipTypes := map[string]bool{"docs": true, "test": true, "style": true, "ci": true, "chore": true}
	for _, c := range commits {
		if skipTypes[c.Type] {
			continue
		}
		if !matchCommitInChangelog(c, cl) {
			missing = append(missing, MissingEntry{Commit: c, Reason: "not found"})
		}
	}

	if len(missing) != 1 {
		t.Errorf("expected 1 missing entry (dark mode), got %d", len(missing))
	}
	if missing[0].Commit.Header != "add dark mode" {
		t.Errorf("expected missing entry to be 'add dark mode', got %q", missing[0].Commit.Header)
	}
}
