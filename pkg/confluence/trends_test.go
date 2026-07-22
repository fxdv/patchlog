package confluence

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/trends"
)

func TestRenderTrendsPageMulti(t *testing.T) {
	snaps := []trends.Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, TotalAuthors: 3, ReleaseContributionConcentration: 3, BreakingChanges: 1, ReleaseCommitSpanHours: 432, ConventionalRatio: 0.22, TechDebtUSD: 2100, ReleaseRiskScore: 12, JiraTickets: 5, TopContributors: []trends.ContributorSnap{{Name: "alice", Commits: 20}}},
		{Version: "v1.1.0", Date: "2026-05-15", TotalCommits: 51, TotalAuthors: 4, ReleaseContributionConcentration: 4, BreakingChanges: 0, ReleaseCommitSpanHours: 528, ConventionalRatio: 0.31, TechDebtUSD: 3400, ReleaseRiskScore: 18, JiraTickets: 8, TopContributors: []trends.ContributorSnap{{Name: "alice", Commits: 25}, {Name: "bob", Commits: 15}}},
		{Version: "v1.2.0", Date: "2026-06-26", TotalCommits: 43, TotalAuthors: 4, ReleaseContributionConcentration: 4, BreakingChanges: 2, ReleaseCommitSpanHours: 576, ConventionalRatio: 0.28, TechDebtUSD: 4100, ReleaseRiskScore: 15, JiraTickets: 6, TopContributors: []trends.ContributorSnap{{Name: "alice", Commits: 18}, {Name: "bob", Commits: 12}}},
	}

	out := RenderTrendsPage(snaps, TrendsThresholds{})
	if out == "" {
		t.Fatal("expected non-empty trends page")
	}
	if !strings.Contains(out, "Release Trends") {
		t.Error("expected 'Release Trends' heading")
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v1.1.0") || !strings.Contains(out, "v1.2.0") {
		t.Error("expected all version names in output")
	}
	if !strings.Contains(out, "Top Contributors") {
		t.Error("expected Top Contributors section")
	}
	if !strings.Contains(out, "alice") {
		t.Error("expected contributor name 'alice'")
	}
	if !strings.Contains(out, "Trend Charts") {
		t.Error("expected Trend Charts section for multi-release")
	}
	if !strings.Contains(out, `ac:name="chart"`) {
		t.Error("expected chart macro in output")
	}
}

func TestRenderTrendsPageSingle(t *testing.T) {
	snap := trends.Snapshot{
		Version:                          "v1.0.0",
		Date:                             "2026-06-26",
		TotalCommits:                     43,
		TotalAuthors:                     4,
		ReleaseContributionConcentration: 4,
		BreakingChanges:                  2,
		ConventionalRatio:                0.28,
		ReleaseCommitSpanHours:           576,
		TechDebtUSD:                      4100,
		ReleaseRiskScore:                 15,
		JiraTickets:                      6,
	}

	out := RenderTrendsPage([]trends.Snapshot{snap}, TrendsThresholds{})
	if out == "" {
		t.Fatal("expected non-empty trends page")
	}
	if !strings.Contains(out, "Release Snapshot") {
		t.Error("expected 'Release Snapshot' info macro for single release")
	}
	if !strings.Contains(out, "Need at least 2 releases") {
		t.Error("expected hint about needing 2+ releases")
	}
	if strings.Contains(out, "Trend Charts") {
		t.Error("chart macro should not appear for single release")
	}
}

func TestRenderTrendsPageEmpty(t *testing.T) {
	out := RenderTrendsPage(nil, TrendsThresholds{})
	if out == "" {
		t.Fatal("expected non-empty output even with nil snapshots (TOC macro)")
	}
}

func TestRenderTrendsPanelEmpty(t *testing.T) {
	out := RenderTrendsPanel(nil, TrendsThresholds{})
	if out != "" {
		t.Errorf("expected empty string for nil snapshots, got %q", out)
	}
}

func TestRenderTrendsWithThresholds(t *testing.T) {
	snaps := []trends.Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, ReleaseContributionConcentration: 2, ReleaseCommitSpanHours: 400, TechDebtUSD: 2000, ReleaseRiskScore: 10},
		{Version: "v1.1.0", Date: "2026-05-15", TotalCommits: 51, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 550, TechDebtUSD: 3500, ReleaseRiskScore: 18},
		{Version: "v1.2.0", Date: "2026-06-26", TotalCommits: 43, ReleaseContributionConcentration: 5, ReleaseCommitSpanHours: 750, TechDebtUSD: 5200, ReleaseRiskScore: 25},
	}

	th := TrendsThresholds{
		ReleaseCommitSpanWarning:            500,
		ReleaseCommitSpanCritical:           700,
		TechDebtWarning:                     3000,
		TechDebtCritical:                    5000,
		ReleaseContributionConcentrationMin: 3,
	}

	out := RenderTrendsPage(snaps, th)
	if !strings.Contains(out, "Thresholds:") {
		t.Error("expected threshold legend when thresholds are configured")
	}
	if !strings.Contains(out, "good") || !strings.Contains(out, "warning") || !strings.Contains(out, "critical") {
		t.Error("expected threshold legend labels")
	}
}

func TestRenderTrendsWithoutThresholds(t *testing.T) {
	snaps := []trends.Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 400, TechDebtUSD: 2000, ReleaseRiskScore: 10},
		{Version: "v1.1.0", Date: "2026-05-15", TotalCommits: 51, ReleaseContributionConcentration: 4, ReleaseCommitSpanHours: 550, TechDebtUSD: 3500, ReleaseRiskScore: 18},
	}

	out := RenderTrendsPage(snaps, TrendsThresholds{})
	if strings.Contains(out, "Thresholds:") {
		t.Error("threshold legend should not appear when no thresholds configured")
	}
}

func TestThresholdCellColor(t *testing.T) {
	th := TrendsThresholds{
		ReleaseCommitSpanWarning:            500,
		ReleaseCommitSpanCritical:           700,
		TechDebtWarning:                     3000,
		TechDebtCritical:                    5000,
		ReleaseContributionConcentrationMin: 3,
	}

	tests := []struct {
		metricKey string
		value     float64
		want      string
	}{
		{"release_commit_span", 400, colorGreen},
		{"release_commit_span", 500, colorYellow},
		{"release_commit_span", 600, colorYellow},
		{"release_commit_span", 700, colorRed},
		{"release_commit_span", 800, colorRed},
		{"tech_debt", 2000, colorGreen},
		{"tech_debt", 3000, colorYellow},
		{"tech_debt", 4999, colorYellow},
		{"tech_debt", 5000, colorRed},
		{"release_contribution_concentration", 1, colorRed},
		{"release_contribution_concentration", 2, colorRed},
		{"release_contribution_concentration", 3, colorYellow},
		{"release_contribution_concentration", 5, colorGreen},
		{"", 100, ""},
		{"unknown", 100, ""},
	}

	for _, tt := range tests {
		got := thresholdCellColor(tt.metricKey, tt.value, th)
		if got != tt.want {
			t.Errorf("thresholdCellColor(%q, %v) = %q, want %q", tt.metricKey, tt.value, got, tt.want)
		}
	}
}

func TestThresholdCellColorNoThresholds(t *testing.T) {
	th := TrendsThresholds{}
	if c := thresholdCellColor("release_commit_span", 800, th); c != "" {
		t.Errorf("expected empty color with no thresholds, got %q", c)
	}
	if c := thresholdCellColor("tech_debt", 9999, th); c != "" {
		t.Errorf("expected empty color with no thresholds, got %q", c)
	}
	if c := thresholdCellColor("release_contribution_concentration", 1, th); c != "" {
		t.Errorf("expected empty color with no thresholds, got %q", c)
	}
}

func TestHasThresholds(t *testing.T) {
	if hasThresholds(TrendsThresholds{}) {
		t.Error("expected false for zero-value thresholds")
	}
	if !hasThresholds(TrendsThresholds{ReleaseCommitSpanWarning: 500}) {
		t.Error("expected true when ReleaseCommitSpanWarning is set")
	}
	if !hasThresholds(TrendsThresholds{ReleaseContributionConcentrationMin: 3}) {
		t.Error("expected true when ReleaseContributionConcentrationMin is set")
	}
}

func TestRenderTrendsChartSingleSnapshot(t *testing.T) {
	var buf bytes.Buffer
	renderTrendsChart(&buf, []trends.Snapshot{
		{Version: "v1.0.0", TotalCommits: 10, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 400, ReleaseRiskScore: 10},
	})
	if buf.Len() > 0 {
		t.Error("chart should not be rendered for single snapshot")
	}
}

func TestRenderTrendsChartMultiSnapshot(t *testing.T) {
	var buf bytes.Buffer
	renderTrendsChart(&buf, []trends.Snapshot{
		{Version: "v1.0.0", TotalCommits: 10, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 400, ReleaseRiskScore: 10},
		{Version: "v1.1.0", TotalCommits: 15, ReleaseContributionConcentration: 4, ReleaseCommitSpanHours: 500, ReleaseRiskScore: 15},
	})
	out := buf.String()
	if !strings.Contains(out, `ac:name="chart"`) {
		t.Error("expected chart macro")
	}
	if !strings.Contains(out, "line") {
		t.Error("expected line chart type")
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v1.1.0") {
		t.Error("expected version labels in chart data")
	}
}
