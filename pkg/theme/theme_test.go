package theme

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/render"
)

func TestCollectEntries(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{Description: "DailyRunGpuUsage aggregation", Author: "Alice", Hash: "abc123", JiraKeys: []string{"KEXP-293"}},
					{Description: "Corrected budget compute", Author: "Bob", Hash: "def456"},
				},
			},
			{
				Heading: "Bug Fixes",
				Type:    "fix",
				Items: []render.Item{
					{Description: "Fix sorting", Author: "Charlie", Hash: "ghi789", JiraKeys: []string{"KEXP-420"}},
				},
			},
		},
	}

	entries := collectEntries(report)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Index != 1 {
		t.Errorf("expected index 1, got %d", entries[0].Index)
	}
	if entries[0].Description != "DailyRunGpuUsage aggregation" {
		t.Errorf("unexpected description: %s", entries[0].Description)
	}
	if entries[0].Author != "Alice" {
		t.Errorf("unexpected author: %s", entries[0].Author)
	}
	if entries[2].JiraKey != "KEXP-420" {
		t.Errorf("unexpected jira key: %s", entries[2].JiraKey)
	}
}

func TestParseResponse(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{Description: "GPU analytics", Hash: "a1"},
					{Description: "Budget report", Hash: "b2"},
				},
			},
			{
				Heading: "Bug Fixes",
				Type:    "fix",
				Items: []render.Item{
					{Description: "Fix sorting", Hash: "c3"},
				},
			},
		},
	}

	entries := collectEntries(report)

	jsonResp := `{"themes":[{"title":"GPU Budget Analytics","narrative":"Rebuilt the compute budget pipeline","items":[1,2]},{"title":"Bug Fixes","items":[3]}]}`

	themes, err := parseResponse(jsonResp, entries, report)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if len(themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(themes))
	}

	if themes[0].Title != "GPU Budget Analytics" {
		t.Errorf("expected 'GPU Budget Analytics', got %s", themes[0].Title)
	}
	if themes[0].Narrative != "Rebuilt the compute budget pipeline" {
		t.Errorf("unexpected narrative: %s", themes[0].Narrative)
	}
	if len(themes[0].Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(themes[0].Items))
	}
	if themes[1].Title != "Bug Fixes" {
		t.Errorf("expected 'Bug Fixes', got %s", themes[1].Title)
	}
}

func TestParseResponseWithMarkdown(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{
				Items: []render.Item{
					{Description: "feat 1", Hash: "a1"},
					{Description: "fix 1", Hash: "b2"},
				},
			},
		},
	}

	entries := collectEntries(report)

	jsonResp := "```json\n{\"themes\":[{\"title\":\"All Changes\",\"items\":[1,2]}]}\n```"

	themes, err := parseResponse(jsonResp, entries, report)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(themes))
	}
}

func TestParseResponseUnassigned(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{
				Items: []render.Item{
					{Description: "item 1", Hash: "a1"},
					{Description: "item 2", Hash: "b2"},
					{Description: "item 3", Hash: "c3"},
				},
			},
		},
	}

	entries := collectEntries(report)

	jsonResp := `{"themes":[{"title":"Theme A","items":[1]}]}`

	themes, err := parseResponse(jsonResp, entries, report)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if len(themes) != 2 {
		t.Fatalf("expected 2 themes (1 + unassigned), got %d", len(themes))
	}
	if themes[1].Title != "Other Changes" {
		t.Errorf("expected 'Other Changes', got %s", themes[1].Title)
	}
	if len(themes[1].Items) != 2 {
		t.Errorf("expected 2 unassigned items, got %d", len(themes[1].Items))
	}
}

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{`garbage {"a":1} more garbage`, `{"a":1}`},
		{`  {"a":1}  `, `{"a":1}`},
	}

	for _, tt := range tests {
		got := cleanJSON(tt.input)
		if got != tt.expect {
			t.Errorf("cleanJSON(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestFallbackReport(t *testing.T) {
	report := render.Report{
		Version: "v1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Items: []render.Item{{Description: "feat 1"}}},
			{Heading: "Bug Fixes", Items: []render.Item{{Description: "fix 1"}}},
		},
	}

	tr := fallbackReport(report)
	if tr.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", tr.Version)
	}
	if len(tr.Themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(tr.Themes))
	}
	if tr.Themes[0].Title != "Features" {
		t.Errorf("expected 'Features', got %s", tr.Themes[0].Title)
	}
}

func TestBuildPrompt(t *testing.T) {
	report := render.Report{
		Version: "v1.0.0",
		Date:    "2026-06-26",
		Sections: []render.Section{
			{Items: []render.Item{{Description: "feat 1", Author: "Alice"}}},
		},
	}

	entries := collectEntries(report)
	opts := Options{MinThemes: 3, MaxThemes: 7, IncludeNarrative: true}
	prompt := buildPrompt(report, entries, opts)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(prompt, "v1.0.0") {
		t.Error("prompt should contain version")
	}
	if !contains(prompt, "3-7") {
		t.Error("prompt should contain theme range")
	}
	if !contains(prompt, "narrative") {
		t.Error("prompt should mention narrative")
	}
	if !contains(prompt, "feat 1") {
		t.Error("prompt should contain commit description")
	}
	if !contains(prompt, "@Alice") {
		t.Error("prompt should contain author")
	}
}

func TestMarkdownRendering(t *testing.T) {
	tr := ThemedReport{
		Version:    "v1.0.0",
		Date:       "2026-06-26",
		CompareURL: "https://example.com/compare/v0.9...v1.0.0",
		Themes: []ThemeGroup{
			{
				Title:     "GPU Analytics",
				Narrative: "Rebuilt the analytics pipeline",
				Items: []render.Item{
					{Description: "DailyRunGpuUsage aggregation", Author: "Alice"},
				},
			},
		},
		ShowAuthor: true,
		Emojis:     true,
	}

	output, err := Markdown(tr)
	if err != nil {
		t.Fatalf("Markdown failed: %v", err)
	}

	s := string(output)
	if !contains(s, "# v1.0.0") {
		t.Error("should contain version heading")
	}
	if !contains(s, "## GPU Analytics") {
		t.Error("should contain theme heading")
	}
	if !contains(s, "*Rebuilt the analytics pipeline*") {
		t.Error("should contain narrative")
	}
	if !contains(s, "by @Alice") {
		t.Error("should contain author")
	}
	if !contains(s, "Full Changelog") {
		t.Error("should contain compare link")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
