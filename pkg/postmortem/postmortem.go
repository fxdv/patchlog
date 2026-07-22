// Package postmortem analyzes post-release stability by detecting rollbacks, hotfixes, and regressions.
package postmortem

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/gitlog"
)

type Finding struct {
	Hash      string
	Author    string
	Message   string
	Type      string
	Timestamp time.Time
	DaysAfter float64
	JiraKeys  []string
}

type Report struct {
	Tag              string
	ReleaseDate      time.Time
	WindowDays       int
	Rollbacks        []Finding
	Hotfixes         []Finding
	Regressions      []Finding
	EmergencyDeploys []Finding
	StabilityScore   int
}

var rollbackKeywords = []string{"revert", "rollback", "revert:", "откат"}
var hotfixKeywords = []string{"hotfix", "urgent", "critical", "prod", "срочно", "критич"}
var deployKeywords = []string{"deploy", "release", "rollout", "cd", "publish"}

func Analyze(ctx context.Context, fetcher *gitlog.Fetcher, tag string, windowDays int) (*Report, error) {
	if windowDays <= 0 {
		windowDays = 7
	}

	releaseDate, err := fetcher.TagDate(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting tag date for %s: %w", tag, err)
	}

	windowEnd := releaseDate.AddDate(0, 0, windowDays)

	commits, err := fetcher.FetchLog(ctx, tag, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("fetching post-release commits: %w", err)
	}

	var r Report
	r.Tag = tag
	r.ReleaseDate = releaseDate
	r.WindowDays = windowDays

	for _, rc := range commits {
		if rc.Timestamp.Before(releaseDate) || rc.Timestamp.After(windowEnd) {
			continue
		}
		if rc.Timestamp.Equal(releaseDate) {
			continue
		}

		daysAfter := rc.Timestamp.Sub(releaseDate).Hours() / 24
		c := commit.Parse(rc)
		msg := strings.ToLower(c.RawHeader)

		finding := Finding{
			Hash:      c.Hash,
			Author:    c.Author,
			Message:   c.RawHeader,
			Type:      c.Type,
			Timestamp: rc.Timestamp,
			DaysAfter: daysAfter,
			JiraKeys:  c.JiraKeys,
		}

		if containsAny(msg, rollbackKeywords) || c.Type == "revert" {
			r.Rollbacks = append(r.Rollbacks, finding)
		}
		if c.Type == "fix" && (containsAny(msg, hotfixKeywords) || hasUrgentJira(c)) {
			r.Hotfixes = append(r.Hotfixes, finding)
		}
		if c.Type == "fix" {
			r.Regressions = append(r.Regressions, finding)
		}
		if c.Type == "chore" || c.Type == "ci" {
			if containsAny(msg, deployKeywords) {
				r.EmergencyDeploys = append(r.EmergencyDeploys, finding)
			}
		}
	}

	r.StabilityScore = computeStability(&r)
	return &r, nil
}

func hasUrgentJira(c commit.Commit) bool {
	return len(c.JiraKeys) > 0
}

func computeStability(r *Report) int {
	score := 100
	score -= len(r.Rollbacks) * 15
	score -= len(r.Hotfixes) * 10
	score -= len(r.Regressions) * 5
	score -= len(r.EmergencyDeploys) * 5
	if score < 0 {
		score = 0
	}
	return score
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func FormatTerminal(r *Report) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("┌─ Release Postmortem ────────────────────────────────────┐\n")
	fmt.Fprintf(&sb, "│  Release: %s\n", r.Tag)
	fmt.Fprintf(&sb, "│  Window:  %d days (%s → %s)\n", r.WindowDays,
		r.ReleaseDate.Format("2006-01-02"),
		r.ReleaseDate.AddDate(0, 0, r.WindowDays).Format("2006-01-02"))
	scoreLabel := "✓"
	if r.StabilityScore < 70 {
		scoreLabel = "⚠️"
	}
	fmt.Fprintf(&sb, "│  Stability Score: %d/100 %s\n", r.StabilityScore, scoreLabel)
	sb.WriteString("│\n")
	fmt.Fprintf(&sb, "│  Rollbacks:         %d\n", len(r.Rollbacks))
	fmt.Fprintf(&sb, "│  Hotfixes:          %d\n", len(r.Hotfixes))
	fmt.Fprintf(&sb, "│  Regressions:       %d\n", len(r.Regressions))
	fmt.Fprintf(&sb, "│  Emergency deploys: %d\n", len(r.EmergencyDeploys))
	sb.WriteString("└──────────────────────────────────────────────────────────┘\n")
	return sb.String()
}

func FormatMarkdown(r *Report) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Release Postmortem\n\n")
	fmt.Fprintf(&sb, "**Release:** %s  \n", r.Tag)
	fmt.Fprintf(&sb, "**Window:** %d days  \n", r.WindowDays)
	fmt.Fprintf(&sb, "**Stability Score:** %d/100\n\n", r.StabilityScore)

	if len(r.Rollbacks) > 0 {
		sb.WriteString(fmt.Sprintf("### Rollbacks (%d)\n\n", len(r.Rollbacks)))
		for _, f := range r.Rollbacks {
			fmt.Fprintf(&sb, "- %s (%.0f days after) by @%s\n", f.Message, f.DaysAfter, f.Author)
		}
		sb.WriteString("\n")
	}
	if len(r.Hotfixes) > 0 {
		sb.WriteString(fmt.Sprintf("### Hotfixes (%d)\n\n", len(r.Hotfixes)))
		for _, f := range r.Hotfixes {
			fmt.Fprintf(&sb, "- %s (%.0f days after) by @%s\n", f.Message, f.DaysAfter, f.Author)
		}
		sb.WriteString("\n")
	}
	if len(r.Regressions) > 0 {
		sb.WriteString(fmt.Sprintf("### Regressions (%d)\n\n", len(r.Regressions)))
		for _, f := range r.Regressions {
			fmt.Fprintf(&sb, "- %s (%.0f days after) by @%s\n", f.Message, f.DaysAfter, f.Author)
		}
		sb.WriteString("\n")
	}
	if len(r.Rollbacks) == 0 && len(r.Hotfixes) == 0 && len(r.Regressions) == 0 {
		sb.WriteString("No rollbacks, hotfixes, or regressions detected. Release looks stable.\n")
	}
	return sb.String()
}
