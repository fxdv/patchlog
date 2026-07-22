package render

import (
	"strings"
	"testing"
)

func TestMarkdownBasic(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add login"}}},
		},
	}
	out, err := Markdown(report)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# 1.0.0") {
		t.Error("should contain version heading")
	}
	if !strings.Contains(s, "2024-01-15") {
		t.Error("should contain date")
	}
	if !strings.Contains(s, "## Features") {
		t.Error("should contain section heading")
	}
	if !strings.Contains(s, "- add login") {
		t.Error("should contain item")
	}
}

func TestMarkdownBreaking(t *testing.T) {
	report := Report{
		Version:  "2.0.0",
		Breaking: []Item{{Description: "removed API"}},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "## Breaking Changes") {
		t.Error("should contain breaking section")
	}
	if !strings.Contains(string(out), "removed API") {
		t.Error("should contain breaking item")
	}
}

func TestMarkdownBreakingEmoji(t *testing.T) {
	report := Report{
		Version:  "2.0.0",
		Emojis:   true,
		Breaking: []Item{{Description: "removed API"}},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "## ⚠️ Breaking Changes") {
		t.Error("should contain emoji breaking heading")
	}
}

func TestMarkdownWithScope(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing", Scope: "api"}}},
		},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "**api**") {
		t.Error("should contain bold scope")
	}
}

func TestMarkdownWithAuthor(t *testing.T) {
	report := Report{
		Version:    "1.0.0",
		ShowAuthor: true,
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing", Author: "alice"}}},
		},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "by @alice") {
		t.Error("should contain author attribution")
	}
}

func TestMarkdownAuthorHidden(t *testing.T) {
	report := Report{
		Version:    "1.0.0",
		ShowAuthor: false,
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing", Author: "alice"}}},
		},
	}
	out, _ := Markdown(report)
	if strings.Contains(string(out), "by @alice") {
		t.Error("author should be hidden")
	}
}

func TestMarkdownWithRef(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing", Ref: "#42"}}},
		},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "(#42)") {
		t.Error("should contain ref")
	}
}

func TestMarkdownWithJira(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []Item{
					{
						Description: "add endpoint",
						JiraIssues: []*JiraInfo{
							{Key: "PROJ-123", Summary: "Add endpoint", URL: "https://example.com/browse/PROJ-123"},
						},
					},
				},
			},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "[PROJ-123](https://example.com/browse/PROJ-123)") {
		t.Error("should contain linked Jira key")
	}
	if !strings.Contains(s, "Add endpoint") {
		t.Error("should contain Jira summary")
	}
}

func TestMarkdownJiraNoURL(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []Item{
					{Description: "add thing", JiraIssues: []*JiraInfo{{Key: "PROJ-123"}}},
				},
			},
		},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "[PROJ-123]") {
		t.Error("should contain plain Jira key without URL")
	}
}

func TestMarkdownCompareURL(t *testing.T) {
	report := Report{
		Version:    "1.0.0",
		CompareURL: "https://github.com/org/repo/compare/v0.9...v1.0",
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "[Full Changelog]") {
		t.Error("should contain changelog link")
	}
}

func TestMarkdownScopeGroups(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{
				Heading: "Features",
				Type:    "feat",
				Scopes: []ScopeGroup{
					{Name: "api", Items: []Item{{Description: "add endpoint"}}},
				},
			},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "### api") {
		t.Error("should contain scope sub-heading")
	}
	if !strings.Contains(s, "add endpoint") {
		t.Error("should contain scoped item")
	}
}

func TestMarkdownEmptySectionsSkipped(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing"}}},
			{Heading: "Bug Fixes", Type: "fix"},
		},
	}
	out, _ := Markdown(report)
	if strings.Contains(string(out), "## Bug Fixes") {
		t.Error("empty sections should be skipped")
	}
}

func TestMarkdownDefaultDate(t *testing.T) {
	report := Report{Version: "1.0.0"}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "202") {
		t.Error("should contain auto-generated date")
	}
}

func TestMarkdownEmojiSectionHeadings(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Emojis:  true,
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "x"}}},
			{Heading: "Bug Fixes", Type: "fix", Items: []Item{{Description: "y"}}},
			{Heading: "Performance Improvements", Type: "perf", Items: []Item{{Description: "z"}}},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "## ✨ Features") {
		t.Error("should contain feat emoji")
	}
	if !strings.Contains(s, "## 🐛 Bug Fixes") {
		t.Error("should contain fix emoji")
	}
	if !strings.Contains(s, "## ⚡ Performance Improvements") {
		t.Error("should contain perf emoji")
	}
}

func TestMarkdownNoEmojiByDefault(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "x"}}},
		},
	}
	out, _ := Markdown(report)
	if strings.Contains(string(out), "✨") {
		t.Error("should not contain emoji when Emojis is false")
	}
}

func TestMarkdownSectionRender(t *testing.T) {
	report := Report{
		Version: "1.2.0",
		Date:    "2024-03-01",
		Emojis:  true,
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing"}}},
		},
	}
	out, _ := MarkdownSection(report, true)
	s := string(out)
	if !strings.Contains(s, "## [1.2.0] — 2024-03-01") {
		t.Error("section should contain version with date")
	}
	if !strings.Contains(s, "### ✨ Features") {
		t.Error("section should contain emoji heading")
	}
}

func TestAccumulateMarkdownEmpty(t *testing.T) {
	section, _ := MarkdownSection(Report{
		Version: "1.0.0",
		Date:    "2024-01-01",
	}, false)
	result := AccumulateMarkdown(nil, section, "Changelog")
	s := string(result)
	if !strings.HasPrefix(s, "# Changelog") {
		t.Error("should start with changelog header")
	}
	if !strings.Contains(s, "All notable changes") {
		t.Error("should contain description")
	}
	if !strings.Contains(s, "## [1.0.0]") {
		t.Error("should contain version section")
	}
}

func TestAccumulateMarkdownPrepend(t *testing.T) {
	existing := []byte("# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n---\n\n## [1.0.0] — 2024-01-01\n\n### Features\n\n- old feature\n")
	section, _ := MarkdownSection(Report{
		Version: "2.0.0",
		Date:    "2024-02-01",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "new feature"}}},
		},
	}, false)
	result := AccumulateMarkdown(existing, section, "Changelog")
	s := string(result)
	v2Idx := strings.Index(s, "## [2.0.0]")
	v1Idx := strings.Index(s, "## [1.0.0]")
	if v2Idx < 0 || v1Idx < 0 {
		t.Fatal("should contain both versions")
	}
	if v2Idx > v1Idx {
		t.Error("new version should be prepended before old version")
	}
	if !strings.HasPrefix(s, "# Changelog") {
		t.Error("should start with changelog header")
	}
}

func TestAccumulateMarkdownNoHeader(t *testing.T) {
	existing := []byte("## [1.0.0] — 2024-01-01\n\n### Features\n\n- old feature\n")
	section, _ := MarkdownSection(Report{
		Version: "2.0.0",
		Date:    "2024-02-01",
	}, false)
	result := AccumulateMarkdown(existing, section, "Changelog")
	s := string(result)
	if !strings.HasPrefix(s, "# Changelog") {
		t.Error("should add changelog header if missing")
	}
}

func TestJSONBasic(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing"}}},
		},
	}
	out, err := JSON(report)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "1.0.0") {
		t.Error("JSON should contain version")
	}
	if !strings.Contains(s, "Features") {
		t.Error("JSON should contain section heading")
	}
}

func TestMarkdownDependencies(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Dependencies: []DependencyChange{
			{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: "npm", Manifest: "package.json"},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "Dependencies") {
		t.Error("should contain Dependencies heading")
	}
	if !strings.Contains(s, "react") {
		t.Error("should contain dependency name")
	}
	if !strings.Contains(s, "^18.2.0 → ^18.3.1") {
		t.Error("should contain version change arrow")
	}
	if !strings.Contains(s, "npm") {
		t.Error("should contain ecosystem")
	}
}

func TestMarkdownDependenciesEmoji(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Emojis:  true,
		Dependencies: []DependencyChange{
			{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: "npm", Manifest: "package.json"},
		},
	}
	out, _ := Markdown(report)
	if !strings.Contains(string(out), "## 📦 Dependencies") {
		t.Error("should contain emoji dependencies heading")
	}
}

func TestMarkdownDependenciesNewDep(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Dependencies: []DependencyChange{
			{Name: "axios", OldVersion: "", NewVersion: "^1.6.0", Ecosystem: "npm", Manifest: "package.json", ChangelogURL: "https://www.npmjs.com/package/axios/v/1.6.0"},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "axios") {
		t.Error("should contain dependency name")
	}
	if !strings.Contains(s, "^1.6.0") {
		t.Error("should contain new version")
	}
	if !strings.Contains(s, "https://www.npmjs.com/package/axios/v/1.6.0") {
		t.Error("should contain changelog URL")
	}
}

func TestMarkdownDependenciesWithChangelog(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Dependencies: []DependencyChange{
			{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: "npm", Manifest: "package.json", Changelog: "Removed legacy defaultProps"},
		},
	}
	out, _ := Markdown(report)
	s := string(out)
	if !strings.Contains(s, "<details>") {
		t.Error("should contain details tag for changelog")
	}
	if !strings.Contains(s, "Removed legacy defaultProps") {
		t.Error("should contain changelog text")
	}
}

func TestMarkdownDependenciesEmpty(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Sections: []Section{
			{Heading: "Features", Type: "feat", Items: []Item{{Description: "add thing"}}},
		},
	}
	out, _ := Markdown(report)
	if strings.Contains(string(out), "Dependencies") {
		t.Error("should not contain Dependencies section when empty")
	}
}

func TestMarkdownSectionDependencies(t *testing.T) {
	report := Report{
		Version: "1.2.0",
		Dependencies: []DependencyChange{
			{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: "npm", Manifest: "package.json"},
		},
	}
	out, _ := MarkdownSection(report, true)
	s := string(out)
	if !strings.Contains(s, "### 📦 Dependencies") {
		t.Error("section should contain emoji dependencies heading")
	}
	if !strings.Contains(s, "react") {
		t.Error("should contain dependency name")
	}
}

func TestJSONDependencies(t *testing.T) {
	report := Report{
		Version: "1.0.0",
		Dependencies: []DependencyChange{
			{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: "npm", Manifest: "package.json"},
		},
	}
	out, _ := JSON(report)
	s := string(out)
	if !strings.Contains(s, "react") {
		t.Error("JSON should contain dependency name")
	}
	if !strings.Contains(s, "^18.2.0") {
		t.Error("JSON should contain old version")
	}
	if !strings.Contains(s, "^18.3.1") {
		t.Error("JSON should contain new version")
	}
}
