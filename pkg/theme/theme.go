package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/render"
)

type ThemeGroup struct {
	Title     string
	Narrative string
	Items     []render.Item
}

type ThemedReport struct {
	Version           string
	Date              string
	Breaking          []render.Item
	Themes            []ThemeGroup
	CompareURL        string
	ShowAuthor        bool
	Emojis            bool
	CommitURLTemplate string
	IssueURLTemplate  string
	Repo              string
}

type Options struct {
	MinThemes        int
	MaxThemes        int
	IncludeNarrative bool
}

type commitEntry struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Author      string `json:"author"`
	JiraKey     string `json:"jira_key,omitempty"`
	JiraSummary string `json:"jira_summary,omitempty"`
}

type themeResponse struct {
	Themes []aiTheme `json:"themes"`
}

type aiTheme struct {
	Title     string `json:"title"`
	Narrative string `json:"narrative"`
	Items     []int  `json:"items"`
}

func GroupCommits(ctx context.Context, report render.Report, aiClient ai.Client, opts Options) (ThemedReport, error) {
	if aiClient == nil {
		return fallbackReport(report), nil
	}

	if opts.MinThemes <= 0 {
		opts.MinThemes = 3
	}
	if opts.MaxThemes <= 0 {
		opts.MaxThemes = 7
	}

	entries := collectEntries(report)
	if len(entries) == 0 {
		return fallbackReport(report), nil
	}

	prompt := buildPrompt(report, entries, opts)
	if prompt == "" {
		return fallbackReport(report), nil
	}

	text, err := aiClient.StreamGenerate(ctx, prompt, nil)
	if err != nil {
		return fallbackReport(report), fmt.Errorf("AI theme grouping failed: %w", err)
	}

	themes, err := parseResponse(text, entries, report)
	if err != nil {
		return fallbackReport(report), fmt.Errorf("AI theme parse failed: %w", err)
	}

	if len(themes) == 0 {
		return fallbackReport(report), nil
	}

	tr := ThemedReport{
		Version:           report.Version,
		Date:              report.Date,
		Breaking:          report.Breaking,
		Themes:            themes,
		CompareURL:        report.CompareURL,
		ShowAuthor:        report.ShowAuthor,
		Emojis:            report.Emojis,
		CommitURLTemplate: report.CommitURLTemplate,
		IssueURLTemplate:  report.IssueURLTemplate,
		Repo:              report.Repo,
	}
	return tr, nil
}

func collectEntries(report render.Report) []commitEntry {
	var entries []commitEntry
	idx := 0
	report.ForEachItem(func(item *render.Item) {
		idx++
		e := commitEntry{
			Index:       idx,
			Description: item.Description,
			Type:        item.Significance,
			Author:      item.Author,
		}
		if len(item.JiraIssues) > 0 {
			e.JiraKey = item.JiraIssues[0].Key
			e.JiraSummary = item.JiraIssues[0].Summary
		} else if len(item.JiraKeys) > 0 {
			e.JiraKey = item.JiraKeys[0]
		}
		entries = append(entries, e)
	})
	return entries
}

func buildPrompt(report render.Report, entries []commitEntry, opts Options) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "You are analyzing commits from release %s", report.Version)
	if report.Date != "" {
		fmt.Fprintf(&buf, " (%s)", report.Date)
	}
	buf.WriteString(".\n\n")

	fmt.Fprintf(&buf, "Group these %d commits into %d-%d thematic sections based on what they accomplish together.\n", len(entries), opts.MinThemes, opts.MaxThemes)
	buf.WriteString("Look at the actual work being done, not just the commit type.\n\n")

	buf.WriteString("Each theme should have:\n")
	buf.WriteString("- A concise title (2-4 words, no emoji)\n")
	if opts.IncludeNarrative {
		buf.WriteString("- A 1-sentence narrative explaining the theme\n")
	}
	buf.WriteString("- A list of item numbers belonging to this theme\n\n")

	buf.WriteString("Rules:\n")
	buf.WriteString("- Every item must appear in exactly one theme\n")
	buf.WriteString("- No empty themes\n")
	buf.WriteString("- Title should describe the feature area, not the commit type\n\n")

	buf.WriteString("Return ONLY valid JSON, no markdown code blocks, no commentary:\n")
	if opts.IncludeNarrative {
		buf.WriteString(`{"themes":[{"title":"...","narrative":"...","items":[1,3,5]}]}`)
	} else {
		buf.WriteString(`{"themes":[{"title":"...","items":[1,3,5]}]}`)
	}
	buf.WriteString("\n\nCommits:\n")

	for _, e := range entries {
		fmt.Fprintf(&buf, "%d. ", e.Index)
		if e.JiraKey != "" {
			fmt.Fprintf(&buf, "[%s] ", e.JiraKey)
		}
		buf.WriteString(e.Description)
		if e.JiraSummary != "" {
			fmt.Fprintf(&buf, " (%s)", e.JiraSummary)
		}
		if e.Author != "" {
			fmt.Fprintf(&buf, " by @%s", e.Author)
		}
		buf.WriteByte('\n')
	}

	return buf.String()
}

func parseResponse(text string, entries []commitEntry, report render.Report) ([]ThemeGroup, error) {
	cleaned := cleanJSON(text)
	var resp themeResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if len(resp.Themes) == 0 {
		return nil, fmt.Errorf("no themes in response")
	}

	itemByIndex := make(map[int]render.Item)
	idx := 0
	report.ForEachItem(func(item *render.Item) {
		idx++
		itemByIndex[idx] = *item
	})

	assigned := make(map[int]bool)
	var themes []ThemeGroup

	for _, t := range resp.Themes {
		if t.Title == "" {
			continue
		}
		var items []render.Item
		for _, i := range t.Items {
			if i < 1 || i > len(entries) {
				continue
			}
			if assigned[i] {
				continue
			}
			assigned[i] = true
			if item, ok := itemByIndex[i]; ok {
				items = append(items, item)
			}
		}
		if len(items) > 0 {
			themes = append(themes, ThemeGroup{
				Title:     strings.TrimSpace(t.Title),
				Narrative: strings.TrimSpace(t.Narrative),
				Items:     items,
			})
		}
	}

	var unassigned []render.Item
	for i := 1; i <= len(entries); i++ {
		if !assigned[i] {
			if item, ok := itemByIndex[i]; ok {
				unassigned = append(unassigned, item)
			}
		}
	}
	if len(unassigned) > 0 {
		themes = append(themes, ThemeGroup{
			Title: "Other Changes",
			Items: unassigned,
		})
	}

	return themes, nil
}

func cleanJSON(text string) string {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			s = s[start : end+1]
		}
	}
	return s
}

func fallbackReport(report render.Report) ThemedReport {
	return ThemedReport{
		Version:           report.Version,
		Date:              report.Date,
		Breaking:          report.Breaking,
		Themes:            reportToThemes(report),
		CompareURL:        report.CompareURL,
		ShowAuthor:        report.ShowAuthor,
		Emojis:            report.Emojis,
		CommitURLTemplate: report.CommitURLTemplate,
		IssueURLTemplate:  report.IssueURLTemplate,
		Repo:              report.Repo,
	}
}

func reportToThemes(report render.Report) []ThemeGroup {
	var themes []ThemeGroup
	for _, s := range report.Sections {
		if len(s.Items) == 0 && len(s.Scopes) == 0 {
			continue
		}
		var items []render.Item
		items = append(items, s.Items...)
		for _, sg := range s.Scopes {
			items = append(items, sg.Items...)
		}
		if len(items) > 0 {
			themes = append(themes, ThemeGroup{
				Title: s.Heading,
				Items: items,
			})
		}
	}
	return themes
}
