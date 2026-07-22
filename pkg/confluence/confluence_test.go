package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/render"
)

func TestRenderStorageFormatBasic(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items:   []render.Item{{Description: "add login page"}},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "<h1>1.0.0") {
		t.Error("should contain version heading")
	}
	if !strings.Contains(html, "2024-01-15") {
		t.Error("should contain date")
	}
	if !strings.Contains(html, "<h2>Features</h2>") {
		t.Error("should contain section heading")
	}
	if !strings.Contains(html, "add login page") {
		t.Error("should contain item description")
	}
}

func TestRenderStorageFormatBreaking(t *testing.T) {
	report := render.Report{
		Version: "2.0.0",
		Breaking: []render.Item{
			{Description: "removed old API"},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "Breaking Changes") {
		t.Error("should contain breaking changes heading")
	}
	if !strings.Contains(html, "removed old API") {
		t.Error("should contain breaking item")
	}
	if !strings.Contains(html, "ac:structured-macro") {
		t.Error("should contain info macro for breaking")
	}
	if !strings.Contains(html, "ac:name=\"info\"") {
		t.Error("should use info panel for breaking changes")
	}
}

func TestRenderStorageFormatWithJira(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{
						Description: "add endpoint",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-123", Summary: "Add endpoint", URL: "https://example.com/browse/PROJ-123", Priority: "High"},
						},
					},
				},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "PROJ-123") {
		t.Error("should contain Jira key")
	}
	if !strings.Contains(html, "https://example.com/browse/PROJ-123") {
		t.Error("should contain Jira URL as link")
	}
	if !strings.Contains(html, "Add endpoint") {
		t.Error("should contain Jira summary")
	}
	if !strings.Contains(html, "High") {
		t.Error("should contain priority")
	}
}

func TestRenderAnalyticsPanel(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:    42,
		TotalAuthors:    5,
		BreakingChanges: 1,
		TopContributors: []ContributorEntry{
			{Name: "alice", Commits: 15},
			{Name: "bob", Commits: 10},
		},
		SignificanceCounts:     map[string]int{"major": 2, "minor": 20, "patch": 20},
		TypeCounts:             map[string]int{"feat": 15, "fix": 20, "ci": 7},
		DateRange:              "2024-01-01 to 2024-01-15",
		CommitsPerDay:          2.8,
		FilesTouched:           30,
		LinesAdded:             1200,
		LinesDeleted:           300,
		NetLines:               900,
		JiraTicketsLinked:      8,
		HotspotDensity:         35.0,
		ChurnFactor:            5.2,
		ComplexityPerFeat:      120.0,
		ReleaseCommitSpanHours: 360.0,
		ReleaseAgeHours:        480.0,
		OwnershipConc:          45.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "ac:structured-macro") {
		t.Error("should contain structured-macro for info panel")
	}
	if !strings.Contains(panel, "Метрики и аналитика") {
		t.Error("should contain analytics title")
	}
	if !strings.Contains(panel, "Топ контрибьюторов") {
		t.Error("should contain contributors table heading")
	}
	if !strings.Contains(panel, "alice") {
		t.Error("should contain contributor name")
	}
	if !strings.Contains(panel, "Обзор релиза") {
		t.Error("should contain overview table heading")
	}
	if !strings.Contains(panel, "42") {
		t.Error("should contain total commits")
	}
	if !strings.Contains(panel, "Распределение изменений") {
		t.Error("should contain impact distribution heading")
	}
	if !strings.Contains(panel, "Метрики кода") {
		t.Error("should contain code metrics heading")
	}
	idxCode := strings.Index(panel, "Метрики кода")
	idxContrib := strings.Index(panel, "Топ контрибьюторов")
	if idxCode < 0 || idxContrib < 0 {
		t.Fatal("missing required sections")
	}
	if idxCode >= idxContrib {
		t.Error("code metrics table should appear before contributors table")
	}
}

func TestRenderAnalyticsPanelEmpty(t *testing.T) {
	panel := RenderAnalyticsPanel(AnalyticsData{})
	if panel != "" {
		t.Error("should return empty string for zero commits")
	}
}

func TestRenderCodeMetricsTable(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:                     10,
		HotspotDensity:                   55.0,
		ChurnFactor:                      22.5,
		ComplexityPerFeat:                1500.0,
		ReleaseCommitSpanHours:           200.0,
		ReleaseAgeHours:                  400.0,
		OwnershipConc:                    65.0,
		ReleaseContributionConcentration: 1,
		FixToFeatureRatio:                3.5,
		TestToSourceRatio:                15.0,
		RefactoringRatio:                 45.0,
		APISurfaceChange:                 5,
		ReleaseRiskScore:                 75.0,
		BatchFactor:                      2.5,
		RevertRate:                       8.0,
		ScopeIsolation:                   60.0,
		CrossCuttingPct:                  40.0,
		FileVolatility:                   2.8,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "Метрики кода") {
		t.Error("should contain code metrics heading")
	}
	if !strings.Contains(panel, "Release Risk Score") {
		t.Error("should contain release risk score")
	}
	if !strings.Contains(panel, "75/100") {
		t.Error("should contain risk score value")
	}
	if !strings.Contains(panel, "Release Commit Span") {
		t.Error("should contain release commit span metric")
	}
	if !strings.Contains(panel, "Batch Factor") {
		t.Error("should contain batch factor metric")
	}
	if !strings.Contains(panel, "Пачечные коммиты") {
		t.Error("should interpret high batch factor")
	}
	if !strings.Contains(panel, "Release Contribution Concentration") {
		t.Error("should contain release contribution concentration metric")
	}
	if !strings.Contains(panel, ">1</td><td>Один участник составил не менее 80%") {
		t.Error("should interpret release contribution concentration of 1")
	}
	if !strings.Contains(panel, "Fix-to-Feature Ratio") {
		t.Error("should contain fix-to-feature ratio")
	}
	if !strings.Contains(panel, "3.5:1") {
		t.Error("should contain fix-to-feature value")
	}
	if !strings.Contains(panel, "Test-to-Source Ratio") {
		t.Error("should contain test-to-source ratio")
	}
	if !strings.Contains(panel, "Refactoring Ratio") {
		t.Error("should contain refactoring ratio")
	}
	if !strings.Contains(panel, "API Surface Change") {
		t.Error("should contain API surface change")
	}
	if !strings.Contains(panel, "Revert Rate") {
		t.Error("should contain revert rate")
	}
	if !strings.Contains(panel, "Scope Isolation") {
		t.Error("should contain scope isolation")
	}
	if !strings.Contains(panel, "Cross-Cutting Concerns") {
		t.Error("should contain cross-cutting concerns")
	}
	if !strings.Contains(panel, "File Volatility") {
		t.Error("should contain file volatility")
	}
}

func TestRenderCodeMetricsTableWithAIInterpretations(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:                     10,
		ReleaseContributionConcentration: 2,
		ReleaseRiskScore:                 45.0,
		Interpretations: map[string]string{
			"release_contribution_concentration": "Два разработчика держат 80% кода",
			"release_risk_score":                 "Умеренный риск, стоит усилить ревью",
		},
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "Два разработчика держат 80% кода") {
		t.Error("should use AI interpretation for release contribution concentration")
	}
	if !strings.Contains(panel, "Умеренный риск, стоит усилить ревью") {
		t.Error("should use AI interpretation for risk score")
	}
	if strings.Contains(panel, "Ограниченное владение кодом") {
		t.Error("should not use fallback when AI interpretation is present")
	}
}

func TestRenderCodeMetricsTableEmpty(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits: 5,
	}
	panel := RenderAnalyticsPanel(ad)
	if strings.Contains(panel, "Метрики кода") {
		t.Error("should not render code metrics table when all values are zero")
	}
}

func TestRenderNewCodeMetrics(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:           10,
		HotspotScore:           55.0,
		ChangeComplexityProxy:  7.5,
		CrossCuttingChangeRisk: 42.0,
		TechnicalDebtUSD:       4500,
		TouchedTestFileRatio:   30.0,
		OwnershipEntropy:       0.85,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "Hotspot Score") {
		t.Error("should contain Hotspot Score metric")
	}
	if !strings.Contains(panel, "Change Complexity Proxy") {
		t.Error("should contain Change Complexity Proxy metric")
	}
	if !strings.Contains(panel, "Cross-Cutting Change Risk") {
		t.Error("should contain Cross-Cutting Change Risk metric")
	}
	if !strings.Contains(panel, "Technical Debt") {
		t.Error("should contain Technical Debt metric")
	}
	if !strings.Contains(panel, "$4,500") || !strings.Contains(panel, "$4500") {
		if !strings.Contains(panel, "4500") {
			t.Error("should contain technical debt dollar value")
		}
	}
	if !strings.Contains(panel, "Touched Test File Ratio") {
		t.Error("should contain Touched Test File Ratio metric")
	}
	if !strings.Contains(panel, "Ownership Entropy") {
		t.Error("should contain Ownership Entropy metric")
	}
	if !strings.Contains(panel, "0.85") {
		t.Error("should contain ownership entropy value")
	}
}

func TestRenderHotspotScoreGauge(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits: 10,
		HotspotScore: 75.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "width: 75%") {
		t.Error("hotspot score gauge should reflect percentage")
	}
	if !strings.Contains(panel, "Высокий") {
		t.Error("should label high hotspot score")
	}
}

func TestRenderCrossCuttingChangeRiskGauge(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:           10,
		CrossCuttingChangeRisk: 65.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "width: 65%") {
		t.Error("cross-cutting change risk gauge should reflect percentage")
	}
	if !strings.Contains(panel, "Высокая доля") {
		t.Error("should label high cross-cutting change risk")
	}
}

func TestRenderTouchedTestFileRatioGauge(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:         10,
		TouchedTestFileRatio: 60.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "width: 60%") {
		t.Error("touched test file ratio gauge should reflect percentage")
	}
	if !strings.Contains(panel, "Высокая доля затронутых") {
		t.Error("should label good touched test file ratio")
	}
}

func TestRenderMetricsNarrative(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:     10,
		ReleaseRiskScore: 45.0,
		MetricsNarrative: "Кодовая база в хорошем состоянии, риски умеренные.",
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, "Кодовая база в хорошем состоянии") {
		t.Error("should contain metrics narrative text")
	}
	idxTable := strings.Index(panel, "</tbody></table>")
	idxNarrative := strings.Index(panel, "Кодовая база")
	if idxTable < 0 || idxNarrative < 0 {
		t.Fatal("missing table or narrative")
	}
	if idxNarrative < idxTable {
		t.Error("narrative should appear after the code metrics table")
	}
	if !strings.Contains(panel, "border-left") {
		t.Error("narrative should have accent border styling")
	}
}

func TestRenderMetricsNarrativeEmpty(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:     10,
		ReleaseRiskScore: 45.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if strings.Contains(panel, "border-left: 3px") {
		t.Error("should not render narrative block when MetricsNarrative is empty")
	}
}

func TestRenderSectionTableLayout(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{
						Description:  "add feature A",
						Significance: "minor",
						Hash:         "abc1234def",
						Author:       "alice",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-1", URL: "https://jira.example.com/PROJ-1", Status: "Done"},
						},
					},
					{
						Description:  "add feature B",
						Significance: "patch",
						Author:       "bob",
					},
				},
			},
		},
	}
	report.ShowAuthor = true
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "<table") {
		t.Error("should render table for section items")
	}
	if !strings.Contains(html, "Scope</th>") && !strings.Contains(html, "Change</th>") {
		t.Error("should have table headers")
	}
	if !strings.Contains(html, "Ticket</th>") {
		t.Error("should have Ticket column when items have Jira issues")
	}
	if !strings.Contains(html, "Impact</th>") {
		t.Error("should have Impact column when items have significance")
	}
	if !strings.Contains(html, "Commit</th>") {
		t.Error("should have Commit column when items have hash")
	}
	if !strings.Contains(html, "Author</th>") {
		t.Error("should have Author column when items have authors")
	}
	if !strings.Contains(html, "PROJ-1") {
		t.Error("should contain Jira key in table")
	}
	if !strings.Contains(html, "add feature A") {
		t.Error("should contain item description in table cell")
	}
	if !strings.Contains(html, "by @alice") {
		t.Error("should contain author in table")
	}
}

func TestRenderSectionTableOmitsEmptyColumns(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items:   []render.Item{{Description: "simple item"}},
			},
		},
	}
	html := RenderStorageFormat(report)
	if strings.Contains(html, "Ticket</th>") {
		t.Error("should not have Ticket column when no items have Jira or Ref")
	}
	if strings.Contains(html, "Impact</th>") {
		t.Error("should not have Impact column when no items have significance")
	}
	if strings.Contains(html, "Commit</th>") {
		t.Error("should not have Commit column when no items have hash")
	}
	if strings.Contains(html, "Author</th>") {
		t.Error("should not have Author column when ShowAuthor is false")
	}
}

func TestRenderSectionTableWithScopes(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Scopes: []render.ScopeGroup{
					{Name: "api", Items: []render.Item{{Description: "add endpoint"}}},
					{Name: "ui", Items: []render.Item{{Description: "add button"}}},
				},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "Scope</th>") {
		t.Error("should have Scope column when scopes present")
	}
	if !strings.Contains(html, "api") {
		t.Error("should contain scope name")
	}
	if !strings.Contains(html, "ui") {
		t.Error("should contain scope name")
	}
}

func TestRenderBreakingChangesTable(t *testing.T) {
	report := render.Report{
		Version: "2.0.0",
		Breaking: []render.Item{
			{Description: "removed old API", Hash: "def5678abc"},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "Breaking Changes") {
		t.Error("should contain breaking changes heading")
	}
	if !strings.Contains(html, "removed old API") {
		t.Error("should contain breaking item")
	}
	if !strings.Contains(html, "<table") {
		t.Error("breaking changes should use table layout")
	}
	if !strings.Contains(html, "<strong>removed old API</strong>") {
		t.Error("breaking items should have bold description")
	}
}

func TestRenderStorageFormatWithAuthor(t *testing.T) {
	report := render.Report{
		Version:    "1.0.0",
		ShowAuthor: true,
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items:   []render.Item{{Description: "add thing", Author: "alice"}},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "by @alice") {
		t.Error("should contain author attribution")
	}
}

func TestRenderStorageFormatSpacers(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Breaking: []render.Item{
			{Description: "removed old API"},
		},
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{{Description: "add thing"}}},
			{Heading: "Bug Fixes", Type: "fix", Items: []render.Item{{Description: "fix bug"}}},
		},
	}
	out := RenderStorageFormat(report)
	spacer := "<p>&nbsp;</p>"
	count := strings.Count(out, spacer)
	if count < 3 {
		t.Errorf("expected at least 3 spacers (after title, after breaking, between sections), got %d", count)
	}
	idxTitle := strings.Index(out, "</h1>")
	idxFirstSpacer := strings.Index(out, spacer)
	if idxFirstSpacer < idxTitle {
		t.Error("first spacer should come after the h1 title")
	}
}

func TestRenderStorageFormatCompareURL(t *testing.T) {
	report := render.Report{
		Version:    "1.0.0",
		CompareURL: "https://github.com/org/repo/compare/v0.9...v1.0",
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "Full Changelog") {
		t.Error("should contain changelog link")
	}
	if !strings.Contains(html, "https://github.com/org/repo/compare/v0.9...v1.0") {
		t.Error("should contain compare URL")
	}
}

func TestRenderSummaryPanel(t *testing.T) {
	summary := "This release adds new features and fixes bugs."
	panel := RenderSummaryPanel(summary)
	if !strings.Contains(panel, "ac:structured-macro") {
		t.Error("should contain structured-macro for info panel")
	}
	if !strings.Contains(panel, `ac:name="info"`) {
		t.Error("should use info panel macro")
	}
	if !strings.Contains(panel, "Аналитическая сводка") {
		t.Error("should contain summary title")
	}
	if !strings.Contains(panel, summary) {
		t.Error("should contain the summary text")
	}
	if !strings.Contains(panel, "font-size: 15px") {
		t.Error("should use enlarged font size")
	}
	if !strings.Contains(panel, "line-height: 1.75") {
		t.Error("should use comfortable line-height for readability")
	}
	if !strings.Contains(panel, "border-left") {
		t.Error("should contain accent border for visual structure")
	}
	if !strings.Contains(panel, "padding: 10px 0 10px 16px") {
		t.Error("should have breathing room around text")
	}
}

func TestRenderSummaryPanelEmpty(t *testing.T) {
	panel := RenderSummaryPanel("")
	if !strings.Contains(panel, "ac:structured-macro") {
		t.Error("should still render panel even with empty summary")
	}
}

func TestRenderCommandFooter(t *testing.T) {
	cmd := "patchlog --from kexp/0.32.0 --to kexp/0.33.0 --filter services/kexp --confluence --ai-enhance"
	footer := RenderCommandFooter(cmd)
	if !strings.Contains(footer, cmd) {
		t.Error("should contain the command string")
	}
	if !strings.Contains(footer, "font-size: 11px") {
		t.Error("should use small font size")
	}
	if !strings.Contains(footer, "color: #999") {
		t.Error("should use grey color")
	}
	if !strings.Contains(footer, "monospace") {
		t.Error("should use monospace font")
	}
}

func TestRenderCommandFooterEmpty(t *testing.T) {
	footer := RenderCommandFooter("")
	if footer != "" {
		t.Error("should return empty string for empty command")
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := map[string]string{
		`<script>`: "&lt;script&gt;",
		`"quoted"`: "&#34;quoted&#34;",
		`a&b`:      "a&amp;b",
		`it's`:     "it&#39;s",
		`plain`:    "plain",
	}
	for input, want := range tests {
		got := html.EscapeString(input)
		if got != want {
			t.Errorf("EscapeString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestClientConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{"fully configured", Config{BaseURL: "https://example.com", APIToken: "tok", SpaceKey: "ENG"}, true},
		{"missing space", Config{BaseURL: "https://example.com", APIToken: "tok"}, false},
		{"missing base URL", Config{APIToken: "tok", SpaceKey: "ENG"}, false},
		{"missing token", Config{BaseURL: "https://example.com", SpaceKey: "ENG"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			if client.Configured() != tt.want {
				t.Errorf("Configured() = %v, want %v", client.Configured(), tt.want)
			}
		})
	}
}

func TestJiraPriorityColors(t *testing.T) {
	tests := map[string]string{
		"Highest":  "red",
		"Critical": "red",
		"High":     "orange",
		"Medium":   "yellow",
		"Low":      "green",
		"Lowest":   "green",
		"Unknown":  "grey",
	}
	for priority, expectedColor := range tests {
		report := render.Report{
			Version: "1.0.0",
			Sections: []render.Section{
				{
					Heading: "Features",
					Type:    "feat",
					Items: []render.Item{
						{
							Description: "test",
							JiraIssues: []*render.JiraInfo{
								{Key: "PROJ-1", Priority: priority},
							},
						},
					},
				},
			},
		}
		html := RenderStorageFormat(report)
		if !strings.Contains(html, expectedColor) {
			t.Errorf("priority %q should produce color %q in HTML", priority, expectedColor)
		}
	}
}

func newConfluenceMock(t *testing.T) *confluenceMock {
	t.Helper()
	m := &confluenceMock{t: t, pages: make(map[string]*mockPage), version: 1}
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/content", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if r.URL.Query().Get("cql") != "" {
				if m.handleSearch != nil {
					m.handleSearch(w, r)
				} else {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
				}
				return
			}
			m.handleFind(w, r)
			return
		}
		if r.Method == "POST" {
			m.handleCreate(w, r)
			return
		}
	})
	mux.HandleFunc("/rest/api/content/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/rest/api/content/"):]
		query := r.URL.Query()
		if r.Method == "GET" {
			if strings.HasPrefix(path, "search") {
				if m.handleSearch != nil {
					m.handleSearch(w, r)
				} else {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
				}
				return
			}
			if query.Get("expand") == "version" {
				if m.handleGetVersion != nil {
					m.handleGetVersion(w, r, path)
					return
				}
				m.handleGetVersionDefault(w, r, path)
				return
			}
			if strings.Contains(query.Get("expand"), "body.storage") {
				m.handleGetBody(w, r, path)
				return
			}
			m.handleFind(w, r)
			return
		}
		if r.Method == "PUT" {
			if strings.HasSuffix(path, "/restriction") {
				pageID := strings.TrimSuffix(path, "/restriction")
				if m.handleRestrictions != nil {
					m.handleRestrictions(w, r, pageID)
				} else {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{})
				}
				return
			}
			m.handleUpdate(w, r, path)
			return
		}
		if r.Method == "POST" {
			if strings.HasSuffix(path, "/label") {
				pageID := strings.TrimSuffix(path, "/label")
				if m.handleLabels != nil {
					m.handleLabels(w, r, pageID)
				} else {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{})
				}
				return
			}
		}
	})
	m.server = httptest.NewServer(mux)
	return m
}

type mockPage struct {
	ID      string
	Title   string
	Version int
	Body    string
}

type confluenceMock struct {
	t                  *testing.T
	server             *httptest.Server
	pages              map[string]*mockPage
	version            int
	calls              atomic.Int32
	handleLabels       func(w http.ResponseWriter, r *http.Request, path string)
	handleRestrictions func(w http.ResponseWriter, r *http.Request, path string)
	handleSearch       func(w http.ResponseWriter, r *http.Request)
	handleGetVersion   func(w http.ResponseWriter, r *http.Request, path string)
}

func (m *confluenceMock) Close()      { m.server.Close() }
func (m *confluenceMock) URL() string { return m.server.URL }

func (m *confluenceMock) handleFind(w http.ResponseWriter, r *http.Request) {
	m.calls.Add(1)
	title := r.URL.Query().Get("title")
	if p, ok := m.pages[title]; ok {
		result := map[string]any{
			"results": []map[string]any{
				{
					"id":    p.ID,
					"title": p.Title,
					"type":  "page",
					"_links": map[string]string{
						"webui": "/spaces/ENG/pages/" + p.ID,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
}

func (m *confluenceMock) handleCreate(w http.ResponseWriter, r *http.Request) {
	m.calls.Add(1)
	var req struct {
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	m.version++
	p := &mockPage{
		ID:      fmt.Sprintf("page-%d", m.version),
		Title:   req.Title,
		Version: m.version,
		Body:    req.Body.Storage.Value,
	}
	m.pages[req.Title] = p

	result := map[string]any{
		"id":    p.ID,
		"title": p.Title,
		"_links": map[string]string{
			"webui": "/spaces/ENG/pages/" + p.ID,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (m *confluenceMock) handleUpdate(w http.ResponseWriter, r *http.Request, path string) {
	m.calls.Add(1)
	var req struct {
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if p, ok := m.pages[req.Title]; ok {
		p.Version++
		p.Body = req.Body.Storage.Value
		result := map[string]any{
			"id":    p.ID,
			"title": p.Title,
			"_links": map[string]string{
				"webui": "/spaces/ENG/pages/" + p.ID,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}
	http.Error(w, "not found", 404)
}

func (m *confluenceMock) handleGetVersionDefault(w http.ResponseWriter, r *http.Request, path string) {
	m.calls.Add(1)
	id := strings.Split(path, "?")[0]
	for _, p := range m.pages {
		if p.ID == id {
			result := map[string]any{
				"version": map[string]int{"number": p.Version},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
	}
	http.Error(w, "not found", 404)
}

func (m *confluenceMock) handleGetBody(w http.ResponseWriter, r *http.Request, path string) {
	m.calls.Add(1)
	id := strings.Split(path, "?")[0]
	for _, p := range m.pages {
		if p.ID == id {
			result := map[string]any{
				"id":    p.ID,
				"title": p.Title,
				"body": map[string]any{
					"storage": map[string]any{
						"value":          p.Body,
						"representation": "storage",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
	}
	http.Error(w, "not found", 404)
}

func TestFindPageCacheHit(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, _, err := client.PublishOrUpdate(context.Background(), "My Page", "<p>content</p>")
	if err != nil {
		t.Fatalf("PublishOrUpdate: %v", err)
	}

	firstCalls := mock.calls.Load()

	_, err = client.FindPage(context.Background(), "My Page")
	if err != nil {
		t.Fatalf("FindPage (cached): %v", err)
	}

	if mock.calls.Load() != firstCalls {
		t.Errorf("expected no additional HTTP calls for cached FindPage, got %d total", mock.calls.Load())
	}
}

func TestFindPageCacheMiss(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	page, err := client.FindPage(context.Background(), "Nonexistent")
	if err != nil {
		t.Fatalf("FindPage: %v", err)
	}
	if page != nil {
		t.Error("expected nil for nonexistent page")
	}
}

func TestFindPageDisabledCache(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir, cache.WithEnabled(false))

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, _, err := client.PublishOrUpdate(context.Background(), "My Page", "<p>v1</p>")
	if err != nil {
		t.Fatalf("PublishOrUpdate: %v", err)
	}

	firstCalls := mock.calls.Load()

	_, err = client.FindPage(context.Background(), "My Page")
	if err != nil {
		t.Fatalf("FindPage: %v", err)
	}

	if mock.calls.Load() == firstCalls {
		t.Error("disabled cache should cause additional HTTP call for FindPage")
	}
}

func TestCreatePageCachesResult(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	page, err := client.CreatePage(context.Background(), "New Page", "<p>content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}

	var cached Page
	ok, _ := fc.Get("confluence", "New Page", &cached)
	if !ok {
		t.Fatal("expected CreatePage result to be cached")
	}
	if cached.ID != page.ID {
		t.Errorf("cached ID = %q, want %q", cached.ID, page.ID)
	}
}

func TestUpdatePageInvalidatesCache(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	created, err := client.CreatePage(context.Background(), "Update Me", "<p>v1</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	var cached Page
	ok, _ := fc.Get("confluence", "Update Me", &cached)
	if !ok {
		t.Fatal("expected page to be cached after create")
	}

	_, err = client.UpdatePage(context.Background(), created.ID, "Update Me", "<p>v2</p>")
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}

	ok, _ = fc.Get("confluence", "Update Me", &cached)
	if ok {
		t.Error("expected cache to be invalidated after UpdatePage")
	}
}

func TestPublishOrUpdateCreate(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	page, updated, err := client.PublishOrUpdate(context.Background(), "Fresh Page", "<p>new</p>")
	if err != nil {
		t.Fatalf("PublishOrUpdate: %v", err)
	}
	if updated {
		t.Error("expected created, not updated")
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}
}

func TestPublishOrUpdateUpdate(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, _, err := client.PublishOrUpdate(context.Background(), "Existing Page", "<p>v1</p>")
	if err != nil {
		t.Fatalf("first PublishOrUpdate: %v", err)
	}

	page, updated, err := client.PublishOrUpdate(context.Background(), "Existing Page", "<p>v2</p>")
	if err != nil {
		t.Fatalf("second PublishOrUpdate: %v", err)
	}
	if !updated {
		t.Error("expected updated=true on second call")
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}
}

func TestSetCache(t *testing.T) {
	client := NewClient(Config{})
	if client.fileCache != nil {
		t.Error("fileCache should be nil by default")
	}
	fc := cache.New(t.TempDir())
	client.SetCache(fc)
	if client.fileCache == nil {
		t.Error("SetCache should set fileCache")
	}
}

func TestFindPageCrossClientPersistence(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client1 := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client1.SetCache(fc)

	created, _, err := client1.PublishOrUpdate(context.Background(), "My Page", "<p>v1</p>")
	if err != nil {
		t.Fatalf("client1 PublishOrUpdate: %v", err)
	}

	firstCalls := mock.calls.Load()

	client2 := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client2.SetCache(fc)

	page2, err := client2.FindPage(context.Background(), "My Page")
	if err != nil {
		t.Fatalf("client2 FindPage: %v", err)
	}

	if mock.calls.Load() != firstCalls {
		t.Errorf("expected no additional HTTP calls, got %d total", mock.calls.Load())
	}

	if page2.ID != created.ID {
		t.Errorf("page ID mismatch: client2=%q, client1=%q", page2.ID, created.ID)
	}
}

func TestGetPageBody(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, err := client.CreatePage(context.Background(), "Changelog", "<p>existing content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	body, page, err := client.GetPageBody(context.Background(), "Changelog")
	if err != nil {
		t.Fatalf("GetPageBody: %v", err)
	}
	if page == nil {
		t.Fatal("expected page to be non-nil")
	}
	if body != "<p>existing content</p>" {
		t.Errorf("body: got %q", body)
	}
}

func TestGetPageBodyMissing(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	body, page, err := client.GetPageBody(context.Background(), "Nonexistent")
	if err != nil {
		t.Fatalf("GetPageBody: %v", err)
	}
	if page != nil {
		t.Error("expected nil page for nonexistent title")
	}
	if body != "" {
		t.Error("expected empty body for nonexistent page")
	}
}

func TestAccumulatePageCreate(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	page, updated, err := client.AccumulatePage(context.Background(), "Changelog", "<p>release 1</p>")
	if err != nil {
		t.Fatalf("AccumulatePage: %v", err)
	}
	if updated {
		t.Error("expected created, not updated")
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}
}

func TestAccumulatePageUpdate(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, err := client.CreatePage(context.Background(), "Changelog", "<p>old content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	page, updated, err := client.AccumulatePage(context.Background(), "Changelog", "<p>new release</p>")
	if err != nil {
		t.Fatalf("AccumulatePage: %v", err)
	}
	if !updated {
		t.Error("expected updated=true on accumulate with existing page")
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}

	body, _, err := client.GetPageBody(context.Background(), "Changelog")
	if err != nil {
		t.Fatalf("GetPageBody: %v", err)
	}
	if !strings.Contains(body, "new release") {
		t.Error("accumulated body should contain new release content")
	}
	if !strings.Contains(body, "old content") {
		t.Error("accumulated body should preserve old content")
	}
	if !strings.Contains(body, "<hr/>") {
		t.Error("accumulated body should contain separator")
	}
}

func TestAccumulatePageEmptyBody(t *testing.T) {
	mock := newConfluenceMock(t)
	defer mock.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})
	client.SetCache(fc)

	_, err := client.CreatePage(context.Background(), "Changelog", "")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	page, updated, err := client.AccumulatePage(context.Background(), "Changelog", "<p>new release</p>")
	if err != nil {
		t.Fatalf("AccumulatePage: %v", err)
	}
	if !updated {
		t.Error("expected updated=true when accumulating onto existing page with empty body")
	}
	if page.ID == "" {
		t.Error("expected page ID")
	}

	body, _, err := client.GetPageBody(context.Background(), "Changelog")
	if err != nil {
		t.Fatalf("GetPageBody: %v", err)
	}
	if !strings.Contains(body, "new release") {
		t.Error("accumulated body should contain new release content")
	}
}

func TestRenderStorageFormatTOC(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Date:    "2024-01-15",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{{Description: "add feature"}}},
			{Heading: "Bug Fixes", Type: "fix", Items: []render.Item{{Description: "fix bug"}}},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, `ac:name="toc"`) {
		t.Error("should contain TOC macro")
	}
	if !strings.Contains(html, `maxLevel`) {
		t.Error("TOC should have maxLevel parameter")
	}
}

func TestRenderStorageFormatExpandForLongSections(t *testing.T) {
	items := make([]render.Item, 10)
	for i := range items {
		items[i] = render.Item{Description: fmt.Sprintf("item %d", i)}
	}
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: items},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, `ac:name="expand"`) {
		t.Error("should contain expand macro for sections with more than 8 items")
	}
	if !strings.Contains(html, "Features (10)") {
		t.Error("expand title should include item count")
	}
}

func TestRenderStorageFormatNoExpandForShortSections(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{{Description: "item 1"}, {Description: "item 2"}}},
		},
	}
	html := RenderStorageFormat(report)
	if strings.Contains(html, `ac:name="expand"`) {
		t.Error("should not contain expand macro for short sections")
	}
}

func TestRenderStorageFormatCommitHyperlinks(t *testing.T) {
	report := render.Report{
		Version:           "1.0.0",
		CommitURLTemplate: "https://git.example.com/%s/commit/%s",
		IssueURLTemplate:  "https://git.example.com/%s/issues/%s",
		Repo:              "org/repo",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{Description: "add feature", Ref: "#42", Hash: "abc1234def5678"},
				},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, `href="https://git.example.com/org/repo/issues/42"`) {
		t.Error("should contain issue hyperlink for Ref")
	}
	if !strings.Contains(html, `href="https://git.example.com/org/repo/commit/abc1234def5678"`) {
		t.Error("should contain commit hyperlink for Hash")
	}
	if !strings.Contains(html, ">abc1234<") {
		t.Error("should display shortened commit hash")
	}
}

func TestRenderStorageFormatRefWithoutTemplate(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items:   []render.Item{{Description: "add feature", Ref: "#42"}},
			},
		},
	}
	html := RenderStorageFormat(report)
	if !strings.Contains(html, "(#42)") {
		t.Error("should render Ref as plain text when no template configured")
	}
}

func TestRenderPageProperties(t *testing.T) {
	report := render.Report{
		Version: "1.3.0",
		Date:    "2024-06-26",
	}
	sm := AnalyticsData{
		TotalCommits:     47,
		TotalAuthors:     5,
		BreakingChanges:  2,
		ReleaseRiskScore: 42,
	}
	html := RenderPageProperties(report, sm)
	if !strings.Contains(html, `ac:name="details"`) {
		t.Error("should contain details macro for page properties")
	}
	if !strings.Contains(html, "1.3.0") {
		t.Error("should contain version")
	}
	if !strings.Contains(html, "2024-06-26") {
		t.Error("should contain date")
	}
	if !strings.Contains(html, "47") {
		t.Error("should contain commit count")
	}
	if !strings.Contains(html, "Medium") {
		t.Error("should contain risk level")
	}
}

func TestRenderEpicsPanel(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{
				Heading: "Features",
				Type:    "feat",
				Items: []render.Item{
					{
						Description: "add feature A",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-1", EpicKey: "EPIC-100", URL: "https://jira.example.com/browse/PROJ-1"},
						},
					},
					{
						Description: "add feature B",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-2", EpicKey: "EPIC-100", URL: "https://jira.example.com/browse/PROJ-2"},
						},
					},
					{
						Description: "add feature C",
						JiraIssues: []*render.JiraInfo{
							{Key: "PROJ-3", EpicKey: "EPIC-200", URL: "https://jira.example.com/browse/PROJ-3"},
						},
					},
				},
			},
		},
	}
	html := RenderEpicsPanel(report)
	if !strings.Contains(html, `ac:name="info"`) {
		t.Error("should contain info macro")
	}
	if !strings.Contains(html, activeLabels.EpicsInRelease) {
		t.Error("should contain epics panel title")
	}
	if !strings.Contains(html, "EPIC-100") {
		t.Error("should contain first epic key")
	}
	if !strings.Contains(html, "EPIC-200") {
		t.Error("should contain second epic key")
	}
}

func TestRenderEpicsPanelEmpty(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{{Description: "no jira"}}},
		},
	}
	html := RenderEpicsPanel(report)
	if html != "" {
		t.Error("should return empty string when no epics")
	}
}

func TestRenderPrevNextNavBoth(t *testing.T) {
	prev := &SiblingPage{Title: "Release Notes — 1.2.0 (2024-06-20)", URL: "https://conf.example.com/prev"}
	next := &SiblingPage{Title: "Release Notes — 1.4.0 (2024-06-28)", URL: "https://conf.example.com/next"}
	html := RenderPrevNextNav(prev, next)
	if !strings.Contains(html, "prev") {
		t.Error("should contain prev link")
	}
	if !strings.Contains(html, "next") {
		t.Error("should contain next link")
	}
	if !strings.Contains(html, "1.2.0") {
		t.Error("should contain prev title")
	}
	if !strings.Contains(html, "1.4.0") {
		t.Error("should contain next title")
	}
}

func TestRenderPrevNextNavOnlyPrev(t *testing.T) {
	prev := &SiblingPage{Title: "Prev Release", URL: "https://conf.example.com/prev"}
	html := RenderPrevNextNav(prev, nil)
	if !strings.Contains(html, "Prev Release") {
		t.Error("should contain prev title")
	}
}

func TestRenderPrevNextNavNil(t *testing.T) {
	html := RenderPrevNextNav(nil, nil)
	if html != "" {
		t.Error("should return empty string when both nil")
	}
}

func TestRenderProgressBar(t *testing.T) {
	bar := renderProgressBar(75, colorYellow)
	if !strings.Contains(bar, `width: 75%`) {
		t.Error("progress bar should have correct width")
	}
	if !strings.Contains(bar, colorYellow) {
		t.Error("progress bar should use specified color")
	}
}

func TestRenderStackedBar(t *testing.T) {
	segments := []struct {
		pct   float64
		color string
	}{
		{30, colorRed},
		{50, colorYellow},
		{20, colorGreen},
	}
	bar := renderStackedBar(segments)
	if !strings.Contains(bar, `width: 30%`) {
		t.Error("should contain first segment width")
	}
	if !strings.Contains(bar, `width: 50%`) {
		t.Error("should contain second segment width")
	}
	if !strings.Contains(bar, `width: 20%`) {
		t.Error("should contain third segment width")
	}
}

func TestRenderAnalyticsPanelVisualRiskGauge(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:     10,
		ReleaseRiskScore: 65.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, `background: #eee`) {
		t.Error("should contain progress bar background")
	}
	if !strings.Contains(panel, `width: 65%`) {
		t.Error("progress bar should reflect risk score percentage")
	}
}

func TestRenderAnalyticsPanelStackedBar(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:       10,
		SignificanceCounts: map[string]int{"major": 3, "minor": 5, "patch": 2},
	}
	panel := RenderAnalyticsPanel(ad)
	if !strings.Contains(panel, `display: flex`) {
		t.Error("should contain stacked bar container")
	}
}

func TestTypoFixCrossCutting(t *testing.T) {
	ad := AnalyticsData{
		TotalCommits:    10,
		CrossCuttingPct: 55.0,
	}
	panel := RenderAnalyticsPanel(ad)
	if strings.Contains(panel, "кросс-катting") {
		t.Error("should not contain old typo")
	}
	if !strings.Contains(panel, "сквозных") {
		t.Error("should contain corrected text")
	}
}

func TestRenderPageWithTemplateDefault(t *testing.T) {
	sections := PageSections{
		PageProperties: "<p>props</p>",
		AISummary:      "<p>summary</p>",
		Analytics:      "<p>analytics</p>",
		ReleaseNotes:   "<p>notes</p>",
		EpicsPanel:     "<p>epics</p>",
		PrevNextNav:    "<p>nav</p>",
		CommandFooter:  "<p>footer</p>",
	}
	result, err := RenderPageWithTemplate(sections, "")
	if err != nil {
		t.Fatalf("RenderPageWithTemplate: %v", err)
	}
	idxNav := strings.Index(result, "nav")
	idxProps := strings.Index(result, "props")
	idxSummary := strings.Index(result, "summary")
	idxAnalytics := strings.Index(result, "analytics")
	idxNotes := strings.Index(result, "notes")
	idxEpics := strings.Index(result, "epics")
	idxFooter := strings.Index(result, "footer")

	if idxNav >= idxProps || idxProps >= idxSummary || idxSummary >= idxAnalytics ||
		idxAnalytics >= idxNotes || idxNotes >= idxEpics || idxEpics >= idxFooter {
		t.Error("default template should order: nav, props, summary, analytics, notes, epics, footer")
	}
}

func TestRenderPageWithTemplateCustom(t *testing.T) {
	tmplFile := t.TempDir() + "/custom.tmpl"
	customTmpl := "{{.ReleaseNotes}}{{.CommandFooter}}"
	if err := os.WriteFile(tmplFile, []byte(customTmpl), 0644); err != nil {
		t.Fatal(err)
	}
	sections := PageSections{
		ReleaseNotes:  "<p>notes</p>",
		CommandFooter: "<p>footer</p>",
	}
	result, err := RenderPageWithTemplate(sections, tmplFile)
	if err != nil {
		t.Fatalf("RenderPageWithTemplate: %v", err)
	}
	if !strings.Contains(result, "notes") {
		t.Error("should contain release notes")
	}
	if !strings.Contains(result, "footer") {
		t.Error("should contain command footer")
	}
	if strings.Contains(result, "analytics") {
		t.Error("custom template should not contain analytics (not included in template)")
	}
}

func TestRenderPageWithTemplateInvalidPath(t *testing.T) {
	_, err := RenderPageWithTemplate(PageSections{}, "/nonexistent/template.tmpl")
	if err == nil {
		t.Error("should return error for nonexistent template path")
	}
}

func TestAddLabels(t *testing.T) {
	mock := newConfluenceMock(t)
	mock.handleLabels = func(w http.ResponseWriter, r *http.Request, path string) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{})
	}
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	page, err := client.CreatePage(context.Background(), "Test", "<p>content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	err = client.AddLabels(context.Background(), page.ID, []string{"release-notes", "v1.0"})
	if err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
}

func TestAddLabelsEmpty(t *testing.T) {
	client := NewClient(Config{BaseURL: "https://example.com", APIToken: "tok", SpaceKey: "ENG"})
	err := client.AddLabels(context.Background(), "123", nil)
	if err != nil {
		t.Error("AddLabels with empty list should return nil")
	}
}

func TestSetRestrictions(t *testing.T) {
	mock := newConfluenceMock(t)
	mock.handleRestrictions = func(w http.ResponseWriter, r *http.Request, path string) {
		if r.Method != "PUT" {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{})
	}
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	page, err := client.CreatePage(context.Background(), "Test", "<p>content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	err = client.SetRestrictions(context.Background(), page.ID, []string{"user1"}, []string{"user2"})
	if err != nil {
		t.Fatalf("SetRestrictions: %v", err)
	}
}

func TestSetRestrictionsEmpty(t *testing.T) {
	client := NewClient(Config{BaseURL: "https://example.com", APIToken: "tok", SpaceKey: "ENG"})
	err := client.SetRestrictions(context.Background(), "123", nil, nil)
	if err != nil {
		t.Error("SetRestrictions with empty lists should return nil")
	}
}

func TestFindSiblingPages(t *testing.T) {
	mock := newConfluenceMock(t)
	mock.handleSearch = func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"results": []map[string]any{
				{"id": "1", "title": "Release Notes — 1.0.0 (2024-06-01)", "type": "page", "_links": map[string]string{"webui": "/spaces/ENG/pages/1"}},
				{"id": "2", "title": "Release Notes — 1.1.0 (2024-06-15)", "type": "page", "_links": map[string]string{"webui": "/spaces/ENG/pages/2"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	siblings, err := client.FindSiblingPages(context.Background(), "Release Notes")
	if err != nil {
		t.Fatalf("FindSiblingPages: %v", err)
	}
	if len(siblings) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(siblings))
	}
	if siblings[0].Title != "Release Notes — 1.0.0 (2024-06-01)" {
		t.Errorf("unexpected first sibling: %s", siblings[0].Title)
	}
}

func TestGetPageVersionErrorHandling(t *testing.T) {
	mock := newConfluenceMock(t)
	mock.handleGetVersion = func(w http.ResponseWriter, r *http.Request, path string) {
		http.Error(w, `{"error": "unauthorized"}`, 401)
	}
	defer mock.Close()

	client := NewClient(Config{
		BaseURL:  mock.URL(),
		Email:    "test@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	page, err := client.CreatePage(context.Background(), "Test", "<p>content</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}

	_, err = client.UpdatePage(context.Background(), page.ID, "Test", "<p>updated</p>")
	if err == nil {
		t.Error("UpdatePage should fail when getPageVersion returns non-200")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status 401, got: %v", err)
	}
}
