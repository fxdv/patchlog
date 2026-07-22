package gamify

import (
	"strings"
	"testing"
	"time"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/metrics"
)

func TestComputeBasic(t *testing.T) {
	commits := []commit.Commit{
		{Author: "alice", Type: "feat", Timestamp: time.Now()},
		{Author: "bob", Type: "fix", Timestamp: time.Now()},
	}
	rm := metrics.ReportMetrics{
		Authors: []metrics.AuthorStat{
			{Name: "alice", Commits: 10},
			{Name: "bob", Commits: 3},
		},
		TypeCounts: map[string]int{"feat": 5, "fix": 3},
	}
	results := Compute(commits, rm, nil, Options{})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "alice" {
		t.Error("alice should be first (most commits)")
	}
}

func TestComputeBadges(t *testing.T) {
	commits := []commit.Commit{}
	for i := 0; i < 6; i++ {
		commits = append(commits, commit.Commit{Author: "alice", Type: "feat", Timestamp: time.Now()})
	}
	for i := 0; i < 5; i++ {
		commits = append(commits, commit.Commit{Author: "alice", Type: "fix", Timestamp: time.Now()})
	}
	rm := metrics.ReportMetrics{
		Authors: []metrics.AuthorStat{
			{Name: "alice", Commits: 11},
		},
		TypeCounts: map[string]int{"feat": 6, "fix": 5},
	}
	results := Compute(commits, rm, nil, Options{})
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	badgeNames := make(map[string]bool)
	for _, b := range results[0].Badges {
		badgeNames[b.Name] = true
	}
	if !badgeNames["Release Hero"] {
		t.Error("alice should have Release Hero badge")
	}
	if !badgeNames["Feature Machine"] {
		t.Error("alice should have Feature Machine (6 feat)")
	}
	if !badgeNames["Bug Slayer"] {
		t.Error("alice should have Bug Slayer (5 fix)")
	}
	if !badgeNames["First Blood"] {
		t.Error("alice should have First Blood (no prev releases)")
	}
}

func TestComputeLevels(t *testing.T) {
	rm := metrics.ReportMetrics{
		Authors: []metrics.AuthorStat{
			{Name: "newbie", Commits: 1},
			{Name: "regular", Commits: 10},
		},
	}
	results := Compute(nil, rm, nil, Options{})
	if results[1].Level != 1 {
		t.Errorf("newbie should be L1, got L%d", results[1].Level)
	}
	if results[0].Level < 2 {
		t.Errorf("regular should be at least L2, got L%d", results[0].Level)
	}
}

func TestFormatMarkdownEmpty(t *testing.T) {
	if FormatMarkdown(nil) != "" {
		t.Error("nil should produce empty")
	}
}

func TestFormatMarkdownWithResults(t *testing.T) {
	results := []ContributorResult{
		{
			Name:      "alice",
			Commits:   10,
			Level:     3,
			LevelName: "Regular",
			Badges: []Badge{
				{Emoji: "🏆", Name: "Release Hero", Reason: "Most commits"},
			},
		},
	}
	out := FormatMarkdown(results)
	if !strings.Contains(out, "Contributor Achievements") {
		t.Error("should contain heading")
	}
	if !strings.Contains(out, "alice") {
		t.Error("should contain contributor name")
	}
	if !strings.Contains(out, "🏆") {
		t.Error("should contain badge emoji")
	}
	if !strings.Contains(out, "Regular") {
		t.Error("should contain level name")
	}
}
