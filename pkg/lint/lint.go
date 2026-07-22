// Package lint validates commits against conventional commit standards with optional AI suggestions.
package lint

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/commit"
)

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

type Issue struct {
	Commit     commit.Commit
	Severity   Severity
	Rule       string
	Message    string
	Suggestion string
}

type Result struct {
	CommitsChecked int
	Issues         []Issue
	Errors         int
	Warnings       int
}

func (r Result) HasErrors() bool {
	return r.Errors > 0
}

var knownTypes = map[string]bool{
	"feat": true, "fix": true, "perf": true, "refactor": true,
	"docs": true, "test": true, "style": true, "ci": true,
	"chore": true, "build": true, "revert": true,
}

var imperativeVerbs = map[string]bool{
	"add": true, "fix": true, "update": true, "remove": true, "delete": true,
	"create": true, "implement": true, "refactor": true, "improve": true,
	"change": true, "move": true, "rename": true, "replace": true,
	"support": true, "enable": true, "disable": true, "configure": true,
	"upgrade": true, "downgrade": true, "bump": true, "drop": true,
	"introduce": true, "extend": true, "simplify": true, "optimize": true,
	"handle": true, "prevent": true, "avoid": true, "ensure": true,
	"allow": true, "use": true, "switch": true, "migrate": true,
	"extract": true, "inline": true, "merge": true, "split": true,
	"revert": true, "restore": true, "cleanup": true, "document": true,
}

func Lint(commits []commit.Commit) Result {
	var r Result
	r.CommitsChecked = len(commits)

	for _, c := range commits {
		issues := lintCommit(c)
		for _, issue := range issues {
			r.Issues = append(r.Issues, issue)
			switch issue.Severity {
			case SeverityError:
				r.Errors++
			case SeverityWarning:
				r.Warnings++
			}
		}
	}

	return r
}

func lintCommit(c commit.Commit) []Issue {
	var issues []Issue

	if c.Type == "other" {
		issues = append(issues, Issue{
			Commit:   c,
			Severity: SeverityError,
			Rule:     "conventional-format",
			Message:  "commit does not follow conventional commit format (type(scope): description)",
		})
	} else {
		if !knownTypes[c.Type] {
			issues = append(issues, Issue{
				Commit:   c,
				Severity: SeverityWarning,
				Rule:     "unknown-type",
				Message:  fmt.Sprintf("commit type %q is not a standard type", c.Type),
			})
		}
	}

	if c.Header != "" && len(c.Header) < 10 {
		issues = append(issues, Issue{
			Commit:   c,
			Severity: SeverityWarning,
			Rule:     "short-description",
			Message:  "description is too short (less than 10 characters)",
		})
	}

	if c.Header != "" {
		first := []rune(c.Header)[0]
		if unicode.IsUpper(first) {
			issues = append(issues, Issue{
				Commit:   c,
				Severity: SeverityInfo,
				Rule:     "capitalization",
				Message:  "description starts with uppercase (conventional commits use lowercase)",
			})
		}
	}

	if c.Header != "" && strings.HasSuffix(c.Header, ".") {
		issues = append(issues, Issue{
			Commit:   c,
			Severity: SeverityInfo,
			Rule:     "trailing-period",
			Message:  "description ends with a period (conventional commits don't)",
		})
	}

	if c.Header != "" && c.Type != "other" {
		firstWord := strings.Fields(c.Header)
		if len(firstWord) > 0 {
			lower := strings.ToLower(firstWord[0])
			if !imperativeVerbs[lower] && !isPastTense(firstWord[0]) {
				issues = append(issues, Issue{
					Commit:   c,
					Severity: SeverityInfo,
					Rule:     "imperative-mood",
					Message:  fmt.Sprintf("description may not use imperative mood (starts with %q)", firstWord[0]),
				})
			}
		}
	}

	if c.Type == "feat" || c.Type == "fix" || c.Type == "refactor" {
		if c.Body == "" {
			issues = append(issues, Issue{
				Commit:   c,
				Severity: SeverityInfo,
				Rule:     "missing-body",
				Message:  fmt.Sprintf("%s commits should have a body explaining the change", c.Type),
			})
		}
	}

	if c.Breaking && c.Body == "" && !strings.Contains(c.Footer, "BREAKING") {
		issues = append(issues, Issue{
			Commit:   c,
			Severity: SeverityWarning,
			Rule:     "breaking-no-explanation",
			Message:  "breaking change should explain what breaks and how to migrate",
		})
	}

	if c.Type == "fix" && len(c.JiraKeys) == 0 {
		issues = append(issues, Issue{
			Commit:   c,
			Severity: SeverityInfo,
			Rule:     "missing-jira-ref",
			Message:  "fix commits should reference a Jira ticket",
		})
	}

	return issues
}

func isPastTense(word string) bool {
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "ed") && len(lower) > 3 {
		return true
	}
	if strings.HasSuffix(lower, "ing") && len(lower) > 4 {
		return true
	}
	return false
}

func AISuggest(ctx context.Context, issues []Issue, aiClient ai.Client) []Issue {
	if aiClient == nil {
		return issues
	}

	commitIssues := make(map[string][]Issue)
	var commitOrder []string
	for _, issue := range issues {
		if issue.Commit.Type == "other" || issue.Rule == "short-description" {
			key := issue.Commit.Hash
			if _, exists := commitIssues[key]; !exists {
				commitOrder = append(commitOrder, key)
			}
			commitIssues[key] = append(commitIssues[key], issue)
		}
	}

	for _, hash := range commitOrder {
		cIssues := commitIssues[hash]
		if len(cIssues) == 0 {
			continue
		}

		commit := cIssues[0].Commit
		prompt := buildSuggestionPrompt(commit)
		if prompt == "" {
			continue
		}

		suggestion, err := aiClient.Generate(ctx, prompt)
		if err != nil {
			continue
		}
		suggestion = strings.TrimSpace(suggestion)

		if suggestion != "" {
			for i := range issues {
				if issues[i].Commit.Hash == hash {
					issues[i].Suggestion = suggestion
				}
			}
		}
	}

	return issues
}

func buildSuggestionPrompt(c commit.Commit) string {
	var buf strings.Builder
	buf.WriteString("Rewrite this commit message to follow the conventional commit format.\n")
	buf.WriteString("Use the format: type(scope): description\n\n")
	buf.WriteString("Rules:\n")
	buf.WriteString("- Type must be one of: feat, fix, perf, refactor, docs, test, style, ci, chore\n")
	buf.WriteString("- Description should be lowercase, imperative mood, no trailing period\n")
	buf.WriteString("- Keep it concise but descriptive (max 72 characters in the header)\n\n")
	fmt.Fprintf(&buf, "Current commit message:\n%s\n\n", c.RawHeader)
	if c.Body != "" {
		fmt.Fprintf(&buf, "Commit body:\n%s\n\n", c.Body)
	}
	buf.WriteString("Return ONLY the rewritten commit header (first line), nothing else.\n")
	return buf.String()
}

func FormatResult(r Result) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Lint Results: %d commits checked, %d errors, %d warnings\n\n", r.CommitsChecked, r.Errors, r.Warnings)

	currentHash := ""
	for _, issue := range r.Issues {
		if issue.Commit.Hash != currentHash {
			currentHash = issue.Commit.Hash
			hashDisplay := issue.Commit.Hash
			if len(hashDisplay) > 8 {
				hashDisplay = hashDisplay[:8]
			}
			fmt.Fprintf(&buf, "  %s %s\n", hashDisplay, issue.Commit.RawHeader)
		}

		severityIcon := "●"
		switch issue.Severity {
		case SeverityError:
			severityIcon = "✗"
		case SeverityWarning:
			severityIcon = "⚠"
		case SeverityInfo:
			severityIcon = "ℹ"
		}

		fmt.Fprintf(&buf, "    %s [%s] %s\n", severityIcon, issue.Severity, issue.Message)
		if issue.Suggestion != "" {
			fmt.Fprintf(&buf, "    → Suggested: %s\n", issue.Suggestion)
		}
	}

	if r.Errors == 0 && r.Warnings == 0 {
		buf.WriteString("\n  All commits pass linting.\n")
	}

	return buf.String()
}
