// Package commit parses conventional commit messages into structured data with type, scope, breaking flag, and Jira keys.
package commit

import (
	"regexp"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/internal/pattern"
)

type Commit struct {
	Hash         string
	Author       string
	Email        string
	Timestamp    time.Time
	Type         string
	Scope        string
	Breaking     bool
	Header       string
	RawHeader    string
	Body         string
	Footer       string
	ChangedFiles int
	JiraKeys     []string
	Significance string
	ClassReason  string
}

type RawCommit struct {
	Hash      string
	Author    string
	Email     string
	Timestamp time.Time
	Message   string
}

var conventionalRe = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.*)`)

var knownTypes = map[string]bool{
	"feat": true, "fix": true, "build": true, "chore": true,
	"ci": true, "docs": true, "style": true, "refactor": true,
	"perf": true, "test": true, "revert": true, "other": true,
}

func Parse(raw RawCommit) Commit {
	parts := strings.SplitN(raw.Message, "\n\n", 2)
	header := strings.TrimRight(parts[0], "\n\r")

	c := Commit{
		Hash:      raw.Hash,
		Author:    raw.Author,
		Email:     raw.Email,
		Timestamp: raw.Timestamp,
		Header:    header,
		RawHeader: header,
		JiraKeys:  extractJiraKeys(raw.Message),
	}

	if len(parts) > 1 {
		rest := parts[1]
		if idx := strings.LastIndex(rest, "\n\n"); idx >= 0 {
			c.Footer = strings.TrimSpace(rest[idx+2:])
			c.Body = strings.TrimSpace(rest[:idx])
		} else if containsFooterKeyword(rest) {
			c.Footer = strings.TrimSpace(rest)
		} else {
			c.Body = strings.TrimSpace(rest)
		}
	}

	matches := conventionalRe.FindStringSubmatch(header)
	if matches != nil {
		typ := strings.ToLower(matches[1])
		if knownTypes[typ] {
			c.Type = typ
			c.Scope = matches[2]
			c.Header = matches[4]
			if matches[3] == "!" || strings.Contains(c.Footer, "BREAKING CHANGE") || strings.Contains(c.Footer, "BREAKING-CHANGE") {
				c.Breaking = true
			}
			return c
		}
	}

	c.Type = "other"
	return c
}

func containsFooterKeyword(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "BREAKING CHANGE") ||
			strings.HasPrefix(trimmed, "BREAKING-CHANGE") ||
			strings.HasPrefix(trimmed, "Closes ") ||
			strings.HasPrefix(trimmed, "Fixes ") ||
			strings.HasPrefix(trimmed, "Refs ") ||
			strings.HasPrefix(trimmed, "Relates ") {
			return true
		}
	}
	return false
}

func extractJiraKeys(msg string) []string {
	return pattern.ExtractKeys(msg)
}
