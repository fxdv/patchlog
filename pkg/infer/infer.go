// Package infer uses AI to classify uncategorized commits into conventional commit types.
package infer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/commit"
)

type Options struct {
	Threshold float64
	Types     []string
}

type Result struct {
	Hash       string  `json:"hash"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
}

var validTypes = map[string]bool{
	"feat": true, "fix": true, "perf": true, "refactor": true,
	"docs": true, "test": true, "style": true, "ci": true,
	"chore": true, "build": true, "revert": true,
}

func InferTypes(ctx context.Context, commits []commit.Commit, client ai.Client, opts Options) []commit.Commit {
	if client == nil {
		return commits
	}

	var others []commit.Commit
	for _, c := range commits {
		if c.Type == "other" {
			others = append(others, c)
		}
	}
	if len(others) == 0 {
		return commits
	}

	threshold := opts.Threshold
	if threshold <= 0 {
		threshold = 0.6
	}

	allowed := validTypes
	if len(opts.Types) > 0 {
		allowed = make(map[string]bool, len(opts.Types))
		for _, commitType := range opts.Types {
			if validTypes[commitType] {
				allowed[commitType] = true
			}
		}
	}
	prompt := buildPromptWithTypes(others, allowed)
	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return commits
	}

	results := parseResponse(resp)
	if len(results) == 0 {
		return commits
	}

	resultMap := make(map[string]Result, len(results))
	for _, r := range results {
		resultMap[r.Hash] = r
	}

	for i := range commits {
		if commits[i].Type != "other" {
			continue
		}
		if r, ok := resultMap[commits[i].Hash]; ok {
			if allowed[r.Type] && r.Confidence >= threshold {
				commits[i].Type = r.Type
			}
		}
	}

	return commits
}

func buildPrompt(commits []commit.Commit) string {
	return buildPromptWithTypes(commits, validTypes)
}

func buildPromptWithTypes(commits []commit.Commit, allowed map[string]bool) string {
	var sb strings.Builder
	sb.WriteString("You are a commit classifier. Analyze each commit and infer its conventional commit type.\n")
	var types []string
	for _, commitType := range []string{"feat", "fix", "perf", "refactor", "docs", "test", "style", "ci", "chore", "build", "revert"} {
		if allowed[commitType] {
			types = append(types, commitType)
		}
	}
	sb.WriteString("Allowed types: " + strings.Join(types, ", ") + "\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- feat: adds new functionality or user-facing feature\n")
	sb.WriteString("- fix: fixes a bug or incorrect behavior\n")
	sb.WriteString("- perf: improves performance without changing behavior\n")
	sb.WriteString("- refactor: restructures code without changing behavior\n")
	sb.WriteString("- docs: documentation only changes\n")
	sb.WriteString("- test: adds or modifies tests\n")
	sb.WriteString("- style: formatting, whitespace, semicolons (no logic change)\n")
	sb.WriteString("- ci: CI/CD pipeline changes\n")
	sb.WriteString("- chore: maintenance, dependencies, config (non-user-facing)\n")
	sb.WriteString("- build: build system or dependencies\n")
	sb.WriteString("- revert: reverts a previous commit\n\n")
	sb.WriteString("For each commit, respond with a JSON array. Each element:\n")
	sb.WriteString(`{"hash": "<short-hash>", "type": "<type>", "confidence": 0.0-1.0}` + "\n\n")
	sb.WriteString("Commits to classify:\n\n")

	for _, c := range commits {
		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		fmt.Fprintf(&sb, "hash: %s\n", shortHash)
		fmt.Fprintf(&sb, "message: %s\n", c.RawHeader)
		if c.Body != "" {
			body := c.Body
			if len(body) > 200 {
				body = body[:200] + "..."
			}
			fmt.Fprintf(&sb, "body: %s\n", body)
		}
		if c.ChangedFiles > 0 {
			fmt.Fprintf(&sb, "files_changed: %d\n", c.ChangedFiles)
		}
		if len(c.JiraKeys) > 0 {
			fmt.Fprintf(&sb, "jira: %s\n", strings.Join(c.JiraKeys, ", "))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with ONLY the JSON array, no other text.")
	return sb.String()
}

func parseResponse(resp string) []Result {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	start := strings.Index(resp, "[")
	end := strings.LastIndex(resp, "]")
	if start < 0 || end < 0 || end <= start {
		return nil
	}

	var results []Result
	if err := json.Unmarshal([]byte(resp[start:end+1]), &results); err != nil {
		return nil
	}
	return results
}
