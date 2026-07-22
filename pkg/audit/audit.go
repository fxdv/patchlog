// Package audit compares a changelog file against git history to find missing or stale entries.
package audit

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/gitlog"
)

var refRe = regexp.MustCompile(`#(\d+)`)

var skipTypes = map[string]bool{
	"docs": true, "test": true, "style": true, "ci": true, "chore": true,
}

type AuditResult struct {
	ChangelogFile    string
	TotalCommits     int
	ChangelogEntries int
	MissingEntries   []MissingEntry
	StaleEntries     []StaleEntry
	Unversioned      []commit.Commit
}

type MissingEntry struct {
	Commit commit.Commit
	Reason string
}

type StaleEntry struct {
	Line   string
	Reason string
}

func (r AuditResult) HasIssues() bool {
	return len(r.MissingEntries) > 0 || len(r.StaleEntries) > 0
}

func Audit(ctx context.Context, fetcher *gitlog.Fetcher, changelogPath, from, to string) (AuditResult, error) {
	var result AuditResult
	result.ChangelogFile = changelogPath

	rawCommits, err := fetcher.FetchLog(ctx, from, to)
	if err != nil {
		return result, fmt.Errorf("fetch log: %w", err)
	}

	var commits []commit.Commit
	for _, rc := range rawCommits {
		commits = append(commits, commit.Parse(rc))
	}
	result.TotalCommits = len(commits)

	data, err := os.ReadFile(changelogPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.MissingEntries = makeMissingFromAll(commits)
			return result, nil
		}
		return result, fmt.Errorf("read changelog: %w", err)
	}

	changelogStr := string(data)
	result.ChangelogEntries = countChangelogEntries(changelogStr)

	for _, c := range commits {
		if skipTypes[c.Type] {
			continue
		}

		found := matchCommitInChangelog(c, changelogStr)
		if !found {
			result.MissingEntries = append(result.MissingEntries, MissingEntry{
				Commit: c,
				Reason: "commit not found in changelog",
			})
		}
	}

	lines := strings.Split(changelogStr, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		entryText := strings.TrimPrefix(trimmed, "- ")
		if isPlaceholderEntry(entryText) {
			result.StaleEntries = append(result.StaleEntries, StaleEntry{
				Line:   trimmed,
				Reason: "placeholder or empty entry",
			})
		}
	}

	return result, nil
}

func makeMissingFromAll(commits []commit.Commit) []MissingEntry {
	var missing []MissingEntry
	for _, c := range commits {
		if skipTypes[c.Type] {
			continue
		}
		missing = append(missing, MissingEntry{
			Commit: c,
			Reason: "changelog file does not exist",
		})
	}
	return missing
}

func matchCommitInChangelog(c commit.Commit, changelog string) bool {
	descLower := strings.ToLower(c.Header)
	if len(descLower) > 5 && strings.Contains(strings.ToLower(changelog), descLower[:5]) {
		return true
	}

	keywords := extractKeywords(c.Header)
	if len(keywords) >= 2 {
		matches := 0
		lowerCL := strings.ToLower(changelog)
		for _, kw := range keywords {
			if strings.Contains(lowerCL, strings.ToLower(kw)) {
				matches++
			}
		}
		if matches >= 2 {
			return true
		}
	}

	if c.JiraKeys != nil {
		for _, key := range c.JiraKeys {
			if strings.Contains(changelog, key) {
				return true
			}
		}
	}

	if c.Hash != "" && len(c.Hash) >= 8 {
		if strings.Contains(changelog, c.Hash[:8]) {
			return true
		}
	}

	matches := refRe.FindStringSubmatch(c.RawHeader)
	if matches != nil && strings.Contains(changelog, "#"+matches[1]) {
		return true
	}

	return false
}

func extractKeywords(header string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "to": true, "in": true,
		"for": true, "of": true, "on": true, "with": true, "and": true,
		"or": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "by": true, "at": true, "from": true,
	}

	var keywords []string
	for _, word := range strings.Fields(header) {
		w := strings.ToLower(strings.Trim(word, ".,;:!?()[]{}"))
		if len(w) < 3 {
			continue
		}
		if stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

func countChangelogEntries(changelog string) int {
	count := 0
	for _, line := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			count++
		}
	}
	return count
}

func isPlaceholderEntry(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	placeholders := []string{"todo", "tbd", "fixme", "placeholder", "coming soon", "n/a"}
	for _, p := range placeholders {
		if lower == p {
			return true
		}
	}
	return false
}

func FormatResult(r AuditResult) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Audit Results: %s\n", r.ChangelogFile)
	fmt.Fprintf(&buf, "  Commits in range: %d\n", r.TotalCommits)
	fmt.Fprintf(&buf, "  Changelog entries: %d\n", r.ChangelogEntries)

	if len(r.MissingEntries) > 0 {
		fmt.Fprintf(&buf, "\n  Missing Entries (%d):\n", len(r.MissingEntries))
		for _, m := range r.MissingEntries {
			hash := m.Commit.Hash
			if len(hash) > 8 {
				hash = hash[:8]
			}
			fmt.Fprintf(&buf, "    %s %s\n", hash, m.Commit.RawHeader)
			fmt.Fprintf(&buf, "      → %s\n", m.Reason)
		}
	}

	if len(r.StaleEntries) > 0 {
		fmt.Fprintf(&buf, "\n  Stale Entries (%d):\n", len(r.StaleEntries))
		for _, s := range r.StaleEntries {
			fmt.Fprintf(&buf, "    %s\n", s.Line)
			fmt.Fprintf(&buf, "      → %s\n", s.Reason)
		}
	}

	if !r.HasIssues() {
		buf.WriteString("\n  ✓ Changelog is up to date.\n")
	}

	return buf.String()
}
