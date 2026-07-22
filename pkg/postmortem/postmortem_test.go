package postmortem

import (
	"strings"
	"testing"
	"time"
)

func TestComputeStability(t *testing.T) {
	r := &Report{StabilityScore: 100}
	r.Rollbacks = []Finding{{}}
	r.Hotfixes = []Finding{{}, {}}
	score := computeStability(r)
	if score != 100-15-20 {
		t.Errorf("expected %d, got %d", 100-15-20, score)
	}
}

func TestComputeStabilityMin(t *testing.T) {
	r := &Report{}
	r.Rollbacks = make([]Finding, 10)
	r.Hotfixes = make([]Finding, 10)
	score := computeStability(r)
	if score != 0 {
		t.Errorf("expected 0, got %d", score)
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("revert: something", rollbackKeywords) {
		t.Error("should match revert")
	}
	if !containsAny("hotfix prod", hotfixKeywords) {
		t.Error("should match hotfix")
	}
	if containsAny("feat: add feature", rollbackKeywords) {
		t.Error("should not match feat")
	}
}

func TestFormatMarkdownEmpty(t *testing.T) {
	if FormatMarkdown(nil) != "" {
		t.Error("nil should produce empty")
	}
}

func TestFormatMarkdownStable(t *testing.T) {
	r := &Report{
		Tag:            "v1.0.0",
		WindowDays:     7,
		StabilityScore: 100,
		ReleaseDate:    time.Now(),
	}
	out := FormatMarkdown(r)
	if !strings.Contains(out, "Release Postmortem") {
		t.Error("should contain heading")
	}
	if !strings.Contains(out, "stable") {
		t.Error("should contain stable message")
	}
}

func TestFormatMarkdownWithIssues(t *testing.T) {
	r := &Report{
		Tag:            "v1.0.0",
		WindowDays:     7,
		StabilityScore: 75,
		ReleaseDate:    time.Now(),
		Rollbacks: []Finding{
			{Message: "revert: bad commit", Author: "alice", DaysAfter: 2},
		},
		Hotfixes: []Finding{
			{Message: "fix: prod crash", Author: "bob", DaysAfter: 1},
		},
	}
	out := FormatMarkdown(r)
	if !strings.Contains(out, "Rollbacks") {
		t.Error("should contain rollbacks section")
	}
	if !strings.Contains(out, "revert: bad commit") {
		t.Error("should contain rollback message")
	}
	if !strings.Contains(out, "75/100") {
		t.Error("should contain stability score")
	}
}

func TestFormatTerminal(t *testing.T) {
	r := &Report{
		Tag:            "v1.0.0",
		WindowDays:     7,
		StabilityScore: 90,
		ReleaseDate:    time.Now(),
	}
	out := FormatTerminal(r)
	if !strings.Contains(out, "Release Postmortem") {
		t.Error("should contain title")
	}
	if !strings.Contains(out, "90/100") {
		t.Error("should contain score")
	}
}
