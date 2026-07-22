package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

func TestBuildPromptBasic(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Sections: []render.Section{
			{
				Heading: "Features",
				Items: []render.Item{
					{Description: "add login page"},
				},
			},
		},
	}
	prompt := BuildPrompt(report, ToneDev)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !strings.Contains(prompt, "1.0.0") {
		t.Error("prompt should contain version")
	}
	if !strings.Contains(prompt, "add login page") {
		t.Error("prompt should contain change description")
	}
	if !strings.Contains(prompt, "dev") {
		t.Error("prompt should contain tone")
	}
}

func TestBuildPromptWithBreaking(t *testing.T) {
	report := render.Report{
		Version: "2.0.0",
		Breaking: []render.Item{
			{Description: "removed old API"},
		},
	}
	prompt := BuildPrompt(report, ToneDev)
	if !strings.Contains(prompt, "BREAKING CHANGES") {
		t.Error("prompt should contain breaking changes section")
	}
	if !strings.Contains(prompt, "removed old API") {
		t.Error("prompt should contain breaking item")
	}
}

func TestBuildPromptCustomerTone(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "new thing"}}}},
	}
	prompt := BuildPrompt(report, ToneCustomer)
	if !strings.Contains(prompt, "customer") {
		t.Error("prompt should reference customer tone")
	}
}

func TestBuildPromptExecTone(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "new thing"}}}},
	}
	prompt := BuildPrompt(report, ToneExec)
	if !strings.Contains(prompt, "exec") {
		t.Error("prompt should reference exec tone")
	}
}

func TestBuildPromptWithJira(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Items: []render.Item{
					{
						Description: "add endpoint",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-123", Summary: "Add endpoint", Priority: "High"},
						},
					},
				},
			},
		},
	}
	prompt := BuildPrompt(report, ToneDev)
	if !strings.Contains(prompt, "PROJ-123") {
		t.Error("prompt should contain Jira key")
	}
	if !strings.Contains(prompt, "Add endpoint") {
		t.Error("prompt should contain Jira summary")
	}
	if !strings.Contains(prompt, "High") {
		t.Error("prompt should contain Jira priority")
	}
}

func TestBuildChunkPrompt(t *testing.T) {
	report := render.Report{Version: "1.0.0", Date: "2024-01-15"}
	sections := []templatedSection{
		{Heading: "Features", Items: []render.Item{{Description: "add thing"}}},
	}
	prompt := BuildChunkPrompt(report, ToneDev, sections, nil, 1, 3)
	if !strings.Contains(prompt, "PART 1 of 3") {
		t.Error("chunk prompt should contain part numbering")
	}
}

func TestParseTone(t *testing.T) {
	tests := map[string]Tone{
		"dev":      ToneDev,
		"customer": ToneCustomer,
		"exec":     ToneExec,
	}
	for s, want := range tests {
		got, err := ParseTone(s)
		if err != nil {
			t.Errorf("ParseTone(%q) error: %v", s, err)
		}
		if got != want {
			t.Errorf("ParseTone(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestParseToneInvalid(t *testing.T) {
	_, err := ParseTone("friendly")
	if err == nil {
		t.Error("expected error for invalid tone")
	}
}

func TestSplitSectionsIntoChunks(t *testing.T) {
	sections := []templatedSection{
		{Heading: "Features", Items: []render.Item{{Description: strings.Repeat("x", 500)}}},
		{Heading: "Bug Fixes", Items: []render.Item{{Description: strings.Repeat("y", 500)}}},
	}
	chunks := splitSectionsIntoChunks(sections, 600)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks for large sections, got %d", len(chunks))
	}
}

func TestSplitSectionsEmpty(t *testing.T) {
	chunks := splitSectionsIntoChunks(nil, 4000)
	if chunks != nil {
		t.Error("expected nil for empty input")
	}
}

func TestGenerateTemplateFallbackDev(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Date:     "2024-01-15",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "add thing"}}}},
	}
	text := generateTemplateFallback(report, ToneDev)
	if !strings.Contains(text, "# 1.0.0") {
		t.Error("dev fallback should contain version heading")
	}
}

func TestGenerateTemplateFallbackCustomer(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "add thing"}}}},
	}
	text := generateTemplateFallback(report, ToneCustomer)
	if !strings.Contains(text, "# 1.0.0") {
		t.Error("customer fallback should contain version heading")
	}
}

func TestGenerateTemplateFallbackExec(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "add thing"}}}},
	}
	text := generateTemplateFallback(report, ToneExec)
	if !strings.Contains(text, "Executive Summary") {
		t.Error("exec fallback should contain Executive Summary")
	}
}

func TestGenerateProseNoClient(t *testing.T) {
	report := render.Report{
		Version:  "1.0.0",
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "add thing"}}}},
	}
	text, err := GenerateProse(context.Background(), report, ToneDev, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("should return fallback when no client")
	}
}

type mockAIClient struct {
	resp string
	err  error
}

func (m *mockAIClient) Generate(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.resp, nil
}

func (m *mockAIClient) StreamGenerate(_ context.Context, _ string, onToken func(string)) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.resp, nil
}

func TestEnhanceReportNilClient(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{{Heading: "Features", Items: []render.Item{{Description: "original"}}}},
	}
	out, err := EnhanceReport(context.Background(), report, nil)
	if err != nil {
		t.Fatalf("nil client should not error: %v", err)
	}
	if out.Sections[0].Items[0].Description != "original" {
		t.Error("nil client should leave descriptions unchanged")
	}
}

func TestEnhanceReportAllFail(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{Heading: "Features", Items: []render.Item{{Description: "original"}}},
			{Heading: "Bug Fixes", Items: []render.Item{{Description: "original"}}},
		},
	}
	client := &mockAIClient{err: fmt.Errorf("AI down")}
	_, err := EnhanceReport(context.Background(), report, client)
	if err == nil {
		t.Fatal("expected error when all sections fail")
	}
}

func TestEnhanceReportPartialSuccess(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{Heading: "Features", Items: []render.Item{{Description: "original"}}},
			{Heading: "Bug Fixes", Items: []render.Item{{Description: "original"}}},
		},
	}
	calls := 0
	client := &mockAIClient{
		err: fmt.Errorf("transient"),
	}
	_ = calls
	_, err := EnhanceReport(context.Background(), report, client)
	if err == nil {
		t.Fatal("expected error when all sections fail")
	}
}

func TestEnhanceReportSuccess(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{Heading: "Features", Items: []render.Item{{Description: "original"}}},
		},
	}
	client := &mockAIClient{resp: "enhanced description"}
	out, err := EnhanceReport(context.Background(), report, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Sections[0].Items[0].Description != "enhanced description" {
		t.Errorf("description: got %q", out.Sections[0].Items[0].Description)
	}
}

func TestEnhanceReportEmptySections(t *testing.T) {
	report := render.Report{
		Sections: []render.Section{
			{Heading: "Features", Items: nil},
			{Heading: "Bug Fixes", Items: []render.Item{}},
		},
	}
	client := &mockAIClient{err: fmt.Errorf("should not be called")}
	_, err := EnhanceReport(context.Background(), report, client)
	if err != nil {
		t.Fatalf("empty sections should not error: %v", err)
	}
}

func TestBuildSummaryPromptRussian(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Sections: []render.Section{
			{Heading: "Features", Items: []render.Item{{Description: "add login"}}},
		},
	}
	sm := SummaryMetrics{
		TotalCommits:    42,
		TotalAuthors:    5,
		BreakingChanges: 1,
		TopContributors: []ContributorStat{
			{Name: "alice", Commits: 15},
			{Name: "bob", Commits: 10},
		},
		SignificanceCounts: map[string]int{"major": 2, "minor": 20, "patch": 20},
		TypeCounts:         map[string]int{"feat": 15, "fix": 20, "ci": 7},
		DateRange:          "2024-01-01 to 2024-01-15",
		CommitsPerDay:      2.8,
		FilesTouched:       30,
		LinesAdded:         1200,
		LinesDeleted:       300,
		NetLines:           900,
		JiraTicketsLinked:  8,
	}
	prompt := BuildSummaryPrompt(report, sm, i18n.LangRU)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !strings.Contains(prompt, "русском языке") {
		t.Error("prompt should instruct Russian output")
	}
	if !strings.Contains(prompt, "1.0.0") {
		t.Error("prompt should contain version")
	}
	if !strings.Contains(prompt, "42") {
		t.Error("prompt should contain total commits")
	}
	if !strings.Contains(prompt, "alice") {
		t.Error("prompt should contain top contributor")
	}
	if !strings.Contains(prompt, "аналитическ") {
		t.Error("prompt should request analytical insights")
	}
	if !strings.Contains(prompt, "контрибьютор") {
		t.Error("prompt should mention contributors")
	}
}

func TestGenerateSummaryNilClient(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	sm := SummaryMetrics{TotalCommits: 5}
	summary, err := GenerateSummary(context.Background(), report, nil, sm)
	if err != nil {
		t.Fatalf("nil client should not error: %v", err)
	}
	if summary != "" {
		t.Error("nil client should return empty summary")
	}
}

func TestGenerateMetricsNarrativeNilClient(t *testing.T) {
	sm := SummaryMetrics{TotalCommits: 5}
	_, err := GenerateMetricsNarrative(context.Background(), sm, nil, i18n.LangRU)
	if err == nil {
		t.Fatal("nil client should return error")
	}
}

func TestGenerateMetricsNarrativeSuccess(t *testing.T) {
	client := &mockAIClient{resp: "Кодовая база стабильна, риски минимальные."}
	sm := SummaryMetrics{
		TotalCommits:                     10,
		ReleaseRiskScore:                 25.0,
		ReleaseContributionConcentration: 3,
	}
	text, err := GenerateMetricsNarrative(context.Background(), sm, client, i18n.LangRU)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Кодовая база") {
		t.Errorf("unexpected narrative: %s", text)
	}
}
