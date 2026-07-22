package metrics

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/render"
)

func TestComputeReportMetricsBasic(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{
				{Description: "add login", Author: "alice"},
			}},
		},
	}
	commits := []commit.Commit{
		{Hash: "abc", Author: "alice", Type: "feat", Header: "add login", Timestamp: time.Now()},
		{Hash: "def", Author: "bob", Type: "fix", Header: "fix bug", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.TotalCommits != 2 {
		t.Errorf("expected 2 commits, got %d", m.TotalCommits)
	}
	if m.TotalAuthors != 2 {
		t.Errorf("expected 2 authors, got %d", m.TotalAuthors)
	}
	if m.ConventionalRatio != 1.0 {
		t.Errorf("expected conventional ratio 1.0, got %f", m.ConventionalRatio)
	}
}

func TestComputeReportMetricsEmpty(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	m := ComputeReportMetrics(report, nil)
	if m.TotalCommits != 0 {
		t.Errorf("expected 0 commits, got %d", m.TotalCommits)
	}
	if m.TotalAuthors != 0 {
		t.Errorf("expected 0 authors, got %d", m.TotalAuthors)
	}
}

func TestComputeReportMetricsOtherType(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "alice", Type: "other", Header: "random commit", Timestamp: time.Now()},
		{Author: "bob", Type: "feat", Header: "add thing", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.ConventionalRatio != 0.5 {
		t.Errorf("expected conventional ratio 0.5, got %f", m.ConventionalRatio)
	}
}

func TestComputeReportMetricsAuthorsSorted(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "bob", Type: "feat", Timestamp: time.Now()},
		{Author: "bob", Type: "feat", Timestamp: time.Now()},
		{Author: "charlie", Type: "feat", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if len(m.Authors) != 3 {
		t.Fatalf("expected 3 authors, got %d", len(m.Authors))
	}
	if m.Authors[0].Name != "bob" || m.Authors[0].Commits != 2 {
		t.Errorf("expected bob first with 2 commits, got %s with %d", m.Authors[0].Name, m.Authors[0].Commits)
	}
}

func TestComputeReportMetricsReleaseContributionConcentration(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "bob", Type: "feat", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.ReleaseContributionConcentration < 1 {
		t.Errorf("expected release contribution concentration >= 1, got %d", m.ReleaseContributionConcentration)
	}
}

func TestComputeReportMetricsSingleAuthor(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.OwnershipEntropy != 0 {
		t.Errorf("expected entropy 0 for single author, got %f", m.OwnershipEntropy)
	}
	if m.OwnershipConcentration != 100 {
		t.Errorf("expected concentration 100 for single author, got %f", m.OwnershipConcentration)
	}
}

func TestComputeReportMetricsBreaking(t *testing.T) {
	report := render.Report{
		Version:  "2.0.0",
		Breaking: []render.Item{{Description: "removed API"}},
	}
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Breaking: true, Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.BreakingChanges != 1 {
		t.Errorf("expected 1 breaking change, got %d", m.BreakingChanges)
	}
	if m.CommitQuality.BreakingCount != 1 {
		t.Errorf("expected 1 breaking in quality, got %d", m.CommitQuality.BreakingCount)
	}
}

func TestComputeReportMetricsRevertRate(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "bob", Type: "revert", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.RevertRate != 50 {
		t.Errorf("expected revert rate 50%%, got %f", m.RevertRate)
	}
	if m.CommitQuality.RevertCount != 1 {
		t.Errorf("expected 1 revert, got %d", m.CommitQuality.RevertCount)
	}
}

func TestComputeReportMetricsTypeCounts(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "a", Type: "feat", Timestamp: time.Now()},
		{Author: "b", Type: "fix", Timestamp: time.Now()},
		{Author: "c", Type: "fix", Timestamp: time.Now()},
		{Author: "d", Type: "other", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.TypeCounts["feat"] != 1 {
		t.Errorf("expected 1 feat, got %d", m.TypeCounts["feat"])
	}
	if m.TypeCounts["fix"] != 2 {
		t.Errorf("expected 2 fix, got %d", m.TypeCounts["fix"])
	}
	if m.TypeCounts["other"] != 1 {
		t.Errorf("expected 1 other, got %d", m.TypeCounts["other"])
	}
}

func TestComputeReportMetricsScopeUsage(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	commits := []commit.Commit{
		{Author: "a", Type: "feat", Scope: "api", Timestamp: time.Now()},
		{Author: "b", Type: "fix", Scope: "api", Timestamp: time.Now()},
		{Author: "c", Type: "feat", Scope: "ui", Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.ScopeUsage["api"] != 2 {
		t.Errorf("expected 2 api scope, got %d", m.ScopeUsage["api"])
	}
	if m.ScopeUsage["ui"] != 1 {
		t.Errorf("expected 1 ui scope, got %d", m.ScopeUsage["ui"])
	}
}

func TestComputeReportMetricsDateRange(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	commits := []commit.Commit{
		{Author: "a", Type: "feat", Timestamp: t1},
		{Author: "b", Type: "feat", Timestamp: t2},
	}
	m := ComputeReportMetrics(report, commits)
	if !strings.Contains(m.DateRange, "2024-01-01") {
		t.Errorf("expected date range to contain 2024-01-01, got %s", m.DateRange)
	}
	if !strings.Contains(m.DateRange, "2024-01-15") {
		t.Errorf("expected date range to contain 2024-01-15, got %s", m.DateRange)
	}
}

func TestComputeReportMetricsVelocity(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC)
	commits := []commit.Commit{
		{Author: "a", Type: "feat", Timestamp: t1},
		{Author: "b", Type: "feat", Timestamp: t2},
	}
	m := ComputeReportMetrics(report, commits)
	if m.Velocity.ReleaseCommitSpanHours <= 0 {
		t.Errorf("expected positive release commit span, got %f", m.Velocity.ReleaseCommitSpanHours)
	}
	if m.Velocity.CommitsPerDay <= 0 {
		t.Errorf("expected positive commits per day, got %f", m.Velocity.CommitsPerDay)
	}
}

func TestComputeReportMetricsWeekendRatio(t *testing.T) {
	report := render.Report{Version: "1.0.0"}
	saturday := time.Date(2024, 1, 6, 10, 0, 0, 0, time.UTC)
	monday := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC)
	commits := []commit.Commit{
		{Author: "a", Type: "feat", Timestamp: saturday},
		{Author: "b", Type: "feat", Timestamp: monday},
	}
	m := ComputeReportMetrics(report, commits)
	if m.Velocity.WeekendRatio != 0.5 {
		t.Errorf("expected weekend ratio 0.5, got %f", m.Velocity.WeekendRatio)
	}
}

func TestComputeReportMetricsJiraTickets(t *testing.T) {
	report := render.Report{
		Version: "1.0.0",
		Sections: []render.Section{
			{Heading: "Features", Type: "feat", Items: []render.Item{
				{Description: "add thing", JiraIssues: []*render.JiraInfo{
					{Key: "PROJ-123"},
					{Key: "PROJ-456"},
				}},
			}},
		},
	}
	commits := []commit.Commit{
		{Author: "a", Type: "feat", JiraKeys: []string{"PROJ-123"}, Timestamp: time.Now()},
	}
	m := ComputeReportMetrics(report, commits)
	if m.JiraTicketsLinked != 2 {
		t.Errorf("expected 2 jira tickets linked, got %d", m.JiraTicketsLinked)
	}
	if m.CommitsWithJira != 1 {
		t.Errorf("expected 1 commit with jira, got %d", m.CommitsWithJira)
	}
}

func TestFileExtension(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"main.go", ".go"},
		{"src/app.py", ".py"},
		{"README", "other"},
		{".hidden", ".hidden"},
		{"path/to/file.ts", ".ts"},
	}
	for _, tc := range tests {
		got := fileExtension(tc.path)
		if got != tc.want {
			t.Errorf("fileExtension(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"foo_test.go", true},
		{"app_test.py", true},
		{"component.test.js", true},
		{"component.test.tsx", true},
		{"service.spec.js", true},
		{"src/test/main.go", true},
		{"tests/integration_test.go", true},
		{"src/__tests__/app.js", true},
		{"main.go", false},
		{"app.py", false},
		{"README.md", false},
	}
	for _, tc := range tests {
		got := isTestFile(tc.path)
		if got != tc.want {
			t.Errorf("isTestFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsAPIFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"openapi.yaml", true},
		{"openapi.json", true},
		{"swagger.json", true},
		{"proto/service.proto", true},
		{"schema.graphql", true},
		{"src/api/v1/handler.go", true},
		{"src/api/v2/users.go", true},
		{"src/public/api/doc.go", true},
		{"main.go", false},
		{"README.md", false},
	}
	for _, tc := range tests {
		got := isAPIFile(tc.path)
		if got != tc.want {
			t.Errorf("isAPIFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFileCategory(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"foo_test.go", "test"},
		{"src/client/app.js", "client"},
		{"src/frontend/main.js", "client"},
		{"src/server/handler.go", "server"},
		{"src/backend/api.py", "server"},
		{"src/worker/job.go", "worker"},
		{"migrations/001.sql", "database"},
		{"schema.sql", "database"},
		{"docs/README.md", "docs"},
		{"guide.md", "docs"},
		{"main.go", "other"},
	}
	for _, tc := range tests {
		got := fileCategory(tc.path)
		if got != tc.want {
			t.Errorf("fileCategory(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestComputeHotspots(t *testing.T) {
	fileChurn := map[string]int{
		"a.go": 5,
		"b.go": 3,
		"c.go": 1,
	}
	fileLines := map[string]int{
		"a.go": 100,
		"b.go": 50,
		"c.go": 10,
	}
	hotspots := computeHotspots(fileChurn, fileLines, 2)
	if len(hotspots) != 2 {
		t.Fatalf("expected 2 hotspots, got %d", len(hotspots))
	}
	if hotspots[0].Path != "a.go" || hotspots[0].Changes != 5 {
		t.Errorf("expected a.go first with 5 changes, got %s with %d", hotspots[0].Path, hotspots[0].Changes)
	}
}

func TestFormatMetricsMarkdown(t *testing.T) {
	m := ReportMetrics{
		TotalCommits:       10,
		TotalAuthors:       3,
		BreakingChanges:    1,
		ConventionalRatio:  0.8,
		TypeCounts:         map[string]int{"feat": 5, "fix": 3},
		SignificanceCounts: map[string]int{"major": 1, "minor": 2},
		Authors: []AuthorStat{
			{Name: "alice", Commits: 5},
			{Name: "bob", Commits: 3},
		},
		CommitQuality: CommitQuality{
			AvgHeaderLen:     35,
			CommitsWithBody:  6,
			CommitsWithScope: 4,
			BodyRatio:        0.6,
			ScopeRatio:       0.4,
			BreakingCount:    1,
			RevertCount:      0,
		},
		Velocity: VelocityStats{
			CommitsPerDay:      2.5,
			AvgHoursBetween:    12,
			MostActiveDay:      "Mon",
			MostActiveDayCount: 4,
			WeekendRatio:       0.1,
		},
	}
	cs := CodeStats{
		TotalFiles:     15,
		TotalAdditions: 500,
		TotalDeletions: 100,
		NetLines:       400,
		FilesByType:    map[string]int{".go": 10, ".md": 5},
		Hotspots: []FileHotspot{
			{Path: "main.go", Changes: 5, Lines: 200},
		},
	}
	out := FormatMetricsMarkdown(m, cs)
	if !strings.Contains(out, "Release Metrics") {
		t.Error("should contain metrics heading")
	}
	if !strings.Contains(out, "Total commits") {
		t.Error("should contain total commits")
	}
	if !strings.Contains(out, "alice") {
		t.Error("should contain contributor name")
	}
	if !strings.Contains(out, "main.go") {
		t.Error("should contain hotspot file")
	}
}

func TestComputeCodeStatsWithMockFetcher(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runGit("init")
	runGit("config", "user.name", "test")
	runGit("config", "user.email", "test@test.com")
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "feat: initial commit")

	fetcher := &gitlog.Fetcher{RepoPath: dir}
	commits := []commit.Commit{
		{Hash: "HEAD", Author: "test", Type: "feat", Timestamp: time.Now()},
	}
	cs := ComputeCodeStats(context.Background(), fetcher, commits)
	if cs.TotalFiles == 0 {
		t.Error("expected at least 1 file")
	}
	if cs.TotalAdditions == 0 {
		t.Error("expected some insertions")
	}
}
