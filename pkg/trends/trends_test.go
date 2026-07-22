package trends

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAndLoad(t *testing.T) {
	dir := t.TempDir()

	snap := Snapshot{
		Version:                          "v1.0.0",
		Date:                             "2026-06-26",
		TotalCommits:                     43,
		TotalAuthors:                     4,
		ReleaseContributionConcentration: 4,
		ReleaseCommitSpanHours:           576,
		ConventionalRatio:                0.28,
		TechDebtUSD:                      4100,
		ReleaseRiskScore:                 15,
		TypeCounts:                       map[string]int{"feat": 6, "fix": 6, "other": 31},
	}

	if err := Store(dir, snap); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	expectedPath := filepath.Join(dir, ".patchlog", "trends", "v1-0-0.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected file at %s", expectedPath)
	}

	loaded, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(loaded))
	}
	if loaded[0].Version != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", loaded[0].Version)
	}
	if loaded[0].TotalCommits != 43 {
		t.Errorf("expected 43 commits, got %d", loaded[0].TotalCommits)
	}
}

func TestLoadMultiple(t *testing.T) {
	dir := t.TempDir()

	snaps := []Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, ReleaseContributionConcentration: 3},
		{Version: "v1.1.0", Date: "2026-05-15", TotalCommits: 51, ReleaseContributionConcentration: 3},
		{Version: "v1.2.0", Date: "2026-06-01", TotalCommits: 44, ReleaseContributionConcentration: 3},
		{Version: "v1.3.0", Date: "2026-06-15", TotalCommits: 47, ReleaseContributionConcentration: 4},
		{Version: "v1.4.0", Date: "2026-06-26", TotalCommits: 43, ReleaseContributionConcentration: 4},
	}

	for _, s := range snaps {
		if err := Store(dir, s); err != nil {
			t.Fatalf("Store failed: %v", err)
		}
	}

	loaded, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(loaded) != 5 {
		t.Fatalf("expected 5 snapshots, got %d", len(loaded))
	}

	if loaded[0].Version != "v1.0.0" {
		t.Errorf("expected first to be v1.0.0, got %s", loaded[0].Version)
	}
	if loaded[4].Version != "v1.4.0" {
		t.Errorf("expected last to be v1.4.0, got %s", loaded[4].Version)
	}

	last3, err := Load(dir, 3)
	if err != nil {
		t.Fatalf("Load(3) failed: %v", err)
	}
	if len(last3) != 3 {
		t.Fatalf("expected 3, got %d", len(last3))
	}
	if last3[0].Version != "v1.2.0" {
		t.Errorf("expected first of last 3 to be v1.2.0, got %s", last3[0].Version)
	}
}

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	loaded, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll on empty dir failed: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil, got %v", loaded)
	}
}

func TestSparkline(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{"empty", []float64{}, ""},
		{"single", []float64{42}, "▄"},
		{"flat", []float64{5, 5, 5}, "▄▄▄"},
		{"ascending", []float64{1, 2, 3, 4, 5}, "▁▂▄▆█"},
		{"descending", []float64{5, 4, 3, 2, 1}, "█▆▄▂▁"},
		{"with zero", []float64{0, 5, 10}, "▁▄█"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sparkline(tt.values)
			if got != tt.want {
				t.Errorf("Sparkline(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestSparklineInt(t *testing.T) {
	got := SparklineInt([]int{10, 20, 30, 40, 50})
	if got != "▁▂▄▆█" {
		t.Errorf("SparklineInt = %q, want %q", got, "▁▂▄▆█")
	}
}

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		name       string
		prev, curr float64
		wantChange float64
		wantPct    float64
	}{
		{"increase", 100, 150, 50, 50},
		{"decrease", 200, 100, -100, -50},
		{"no change", 50, 50, 0, 0},
		{"from zero", 0, 100, 100, 0},
		{"to zero", 100, 0, -100, -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ComputeDelta(tt.prev, tt.curr)
			if d.Change != tt.wantChange {
				t.Errorf("Change = %v, want %v", d.Change, tt.wantChange)
			}
			if d.Percent != tt.wantPct {
				t.Errorf("Percent = %v, want %v", d.Percent, tt.wantPct)
			}
		})
	}
}

func TestTrendArrow(t *testing.T) {
	if TrendArrow(Delta{Change: 1}) != "▲" {
		t.Error("expected ▲ for positive change")
	}
	if TrendArrow(Delta{Change: -1}) != "▼" {
		t.Error("expected ▼ for negative change")
	}
	if TrendArrow(Delta{Change: 0}) != "▬" {
		t.Error("expected ▬ for zero change")
	}
}

func TestSanitizeVersion(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"v1.0.0", "v1-0-0"},
		{"kexp/0.33.0", "kexp_0-33-0"},
		{"release 2.0", "release_2-0"},
	}
	for _, tt := range tests {
		got := sanitizeVersion(tt.input)
		if got != tt.expect {
			t.Errorf("sanitizeVersion(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestRenderTerminalEmpty(t *testing.T) {
	out := RenderTerminal(nil)
	if out == "" {
		t.Fatal("expected non-empty output for nil snapshots")
	}
}

func TestRenderTerminalSingle(t *testing.T) {
	snap := Snapshot{
		Version:                          "v1.0.0",
		Date:                             "2026-06-26",
		TotalCommits:                     43,
		TotalAuthors:                     4,
		ReleaseContributionConcentration: 4,
		ReleaseRiskScore:                 15,
	}
	out := RenderTerminal([]Snapshot{snap})
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRenderTerminalMulti(t *testing.T) {
	snaps := []Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 432, TechDebtUSD: 2100, ReleaseRiskScore: 12, ConventionalRatio: 0.22},
		{Version: "v1.1.0", Date: "2026-05-15", TotalCommits: 51, ReleaseContributionConcentration: 3, ReleaseCommitSpanHours: 528, TechDebtUSD: 3400, ReleaseRiskScore: 18, ConventionalRatio: 0.31},
		{Version: "v1.2.0", Date: "2026-06-26", TotalCommits: 43, ReleaseContributionConcentration: 4, ReleaseCommitSpanHours: 576, TechDebtUSD: 4100, ReleaseRiskScore: 15, ConventionalRatio: 0.28},
	}
	out := RenderTerminal(snaps)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestRenderTerminalMultiAllMetrics(t *testing.T) {
	snaps := []Snapshot{
		{
			Version: "v1.0.0", Date: "2026-05-01",
			TotalCommits: 38, TotalAuthors: 3, ReleaseContributionConcentration: 3, BreakingChanges: 1,
			ReleaseCommitSpanHours: 432, ReleaseAgeHours: 100, CommitsPerDay: 1.5, ConventionalRatio: 0.22,
			OwnershipEntropy: 0.8, OwnershipConc: 45, TechDebtUSD: 2100, HotspotScore: 30,
			HotspotDensity: 25, ReleaseRiskScore: 12, NetLines: 500, LinesAdded: 800, LinesDeleted: 300,
			FilesTouched: 20, JiraTickets: 5, ChurnFactor: 5.2, ComplexityPerFeat: 200,
			FixToFeatureRatio: 1.5, TestToSourceRatio: 40, RefactoringRatio: 30, APISurfaceChange: 3,
			BatchFactor: 1.2, RevertRate: 2.5, ScopeIsolation: 60, CrossCuttingPct: 20,
			FileVolatility: 2.1, ChangeComplexityProxy: 4.5, CrossCuttingChangeRisk: 35, TouchedTestFileRatio: 45,
		},
		{
			Version: "v1.1.0", Date: "2026-05-15",
			TotalCommits: 51, TotalAuthors: 4, ReleaseContributionConcentration: 4, BreakingChanges: 0,
			ReleaseCommitSpanHours: 528, ReleaseAgeHours: 120, CommitsPerDay: 2.0, ConventionalRatio: 0.31,
			OwnershipEntropy: 0.9, OwnershipConc: 40, TechDebtUSD: 3400, HotspotScore: 35,
			HotspotDensity: 30, ReleaseRiskScore: 18, NetLines: 700, LinesAdded: 1000, LinesDeleted: 300,
			FilesTouched: 25, JiraTickets: 8, ChurnFactor: 4.8, ComplexityPerFeat: 250,
			FixToFeatureRatio: 2.0, TestToSourceRatio: 50, RefactoringRatio: 35, APISurfaceChange: 5,
			BatchFactor: 1.5, RevertRate: 1.5, ScopeIsolation: 70, CrossCuttingPct: 15,
			FileVolatility: 1.8, ChangeComplexityProxy: 6.0, CrossCuttingChangeRisk: 40, TouchedTestFileRatio: 55,
		},
	}
	out := RenderTerminal(snaps)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "Overview") {
		t.Error("should contain Overview section")
	}
	if !strings.Contains(out, "Velocity") {
		t.Error("should contain Velocity section")
	}
	if !strings.Contains(out, "Code Metrics") {
		t.Error("should contain Code Metrics section")
	}
	if !strings.Contains(out, "Quality") {
		t.Error("should contain Quality section")
	}
	if !strings.Contains(out, "Change complexity") {
		t.Error("should contain change complexity proxy metric")
	}
	if !strings.Contains(out, "Touched test file ratio") {
		t.Error("should contain touched test file ratio metric")
	}
	if !strings.Contains(out, "Cross-cutting risk") {
		t.Error("should contain cross-cutting risk metric")
	}
}

func TestRenderTerminalSingleAllMetrics(t *testing.T) {
	snap := Snapshot{
		Version: "v1.0.0", Date: "2026-06-26",
		TotalCommits: 43, TotalAuthors: 4, ReleaseContributionConcentration: 4, BreakingChanges: 2,
		ReleaseCommitSpanHours: 576, ReleaseAgeHours: 200, CommitsPerDay: 1.8, ConventionalRatio: 0.28,
		OwnershipEntropy: 0.85, OwnershipConc: 42, TechDebtUSD: 4100, HotspotScore: 35,
		HotspotDensity: 28, ReleaseRiskScore: 15, NetLines: 4304, LinesAdded: 5000, LinesDeleted: 696,
		FilesTouched: 30, JiraTickets: 6, ChurnFactor: 5.5, ComplexityPerFeat: 300,
		FixToFeatureRatio: 1.8, TestToSourceRatio: 45, RefactoringRatio: 25, APISurfaceChange: 4,
		BatchFactor: 1.3, RevertRate: 3.0, ScopeIsolation: 65, CrossCuttingPct: 18,
		FileVolatility: 2.0, ChangeComplexityProxy: 5.5, CrossCuttingChangeRisk: 38, TouchedTestFileRatio: 50,
	}
	out := RenderTerminal([]Snapshot{snap})
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "Overview") {
		t.Error("should contain Overview section")
	}
	if !strings.Contains(out, "Code Metrics") {
		t.Error("should contain Code Metrics section")
	}
	if !strings.Contains(out, "Quality") {
		t.Error("should contain Quality section")
	}
	if !strings.Contains(out, "Change complexity") {
		t.Error("should contain change complexity proxy")
	}
}

func TestRenderJSON(t *testing.T) {
	snaps := []Snapshot{
		{Version: "v1.0.0", Date: "2026-05-01", TotalCommits: 38, TotalAuthors: 3, ReleaseContributionConcentration: 3, ReleaseRiskScore: 12, ConventionalRatio: 0.22},
	}
	out, err := RenderJSON(snaps)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty JSON")
	}
}
