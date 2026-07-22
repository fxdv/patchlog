// Package dpi computes a Developer Productivity Index (0-100) per contributor based on velocity, quality, impact, and consistency.
package dpi

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/trends"
)

type DeveloperDPI struct {
	Name        string
	Commits     int
	Score       int
	Grade       string
	Velocity    float64
	Quality     float64
	Impact      float64
	Consistency float64
	Strengths   []string
	Weaknesses  []string
	Percentile  int
}

func Compute(commits []commit.Commit, reportMetrics metrics.ReportMetrics, snapshots []trends.Snapshot) []DeveloperDPI {
	if len(reportMetrics.Authors) == 0 {
		return nil
	}

	authorStats := make(map[string]*struct {
		commits      int
		featCount    int
		fixCount     int
		refactorCnt  int
		testCount    int
		docsCount    int
		withScope    int
		withBody     int
		withJira     int
		breaking     int
		hours        []int
		releases     int
		totalCommits int
	})

	for _, a := range reportMetrics.Authors {
		authorStats[a.Name] = &struct {
			commits      int
			featCount    int
			fixCount     int
			refactorCnt  int
			testCount    int
			docsCount    int
			withScope    int
			withBody     int
			withJira     int
			breaking     int
			hours        []int
			releases     int
			totalCommits int
		}{commits: a.Commits}
	}

	for _, c := range commits {
		s, ok := authorStats[c.Author]
		if !ok {
			continue
		}
		switch c.Type {
		case "feat":
			s.featCount++
		case "fix":
			s.fixCount++
		case "refactor":
			s.refactorCnt++
		case "test":
			s.testCount++
		case "docs":
			s.docsCount++
		}
		if c.Scope != "" {
			s.withScope++
		}
		if c.Body != "" {
			s.withBody++
		}
		if len(c.JiraKeys) > 0 {
			s.withJira++
		}
		if c.Breaking {
			s.breaking++
		}
		s.hours = append(s.hours, c.Timestamp.Hour())
	}

	for _, snap := range snapshots {
		for _, tc := range snap.TopContributors {
			if s, ok := authorStats[tc.Name]; ok {
				s.releases++
				s.totalCommits += tc.Commits
			}
		}
	}

	maxCommits := 0
	for _, a := range reportMetrics.Authors {
		if a.Commits > maxCommits {
			maxCommits = a.Commits
		}
	}

	var results []DeveloperDPI
	for _, a := range reportMetrics.Authors {
		s := authorStats[a.Name]
		d := DeveloperDPI{Name: a.Name, Commits: a.Commits}

		if maxCommits > 0 {
			d.Velocity = float64(a.Commits) / float64(maxCommits) * 100
		}

		scopeR := safeDiv(s.withScope, a.Commits)
		bodyR := safeDiv(s.withBody, a.Commits)
		jiraR := safeDiv(s.withJira, a.Commits)
		d.Quality = (scopeR*30 + bodyR*30 + jiraR*40)

		impactScore := 0.0
		impactScore += float64(s.featCount) * 20
		impactScore += float64(s.fixCount) * 10
		impactScore += float64(s.refactorCnt) * 15
		impactScore += float64(s.testCount) * 8
		impactScore -= float64(s.breaking) * 15
		d.Impact = math.Max(0, math.Min(100, impactScore))

		if s.releases >= 3 {
			d.Consistency = 100
		} else if s.releases == 2 {
			d.Consistency = 65
		} else {
			d.Consistency = 30
		}

		rawScore := d.Velocity*0.25 + d.Quality*0.30 + d.Impact*0.25 + d.Consistency*0.20
		d.Score = int(math.Round(rawScore))

		switch {
		case d.Score >= 85:
			d.Grade = "S"
		case d.Score >= 70:
			d.Grade = "A"
		case d.Score >= 55:
			d.Grade = "B"
		case d.Score >= 40:
			d.Grade = "C"
		default:
			d.Grade = "D"
		}

		if s.featCount >= 3 {
			d.Strengths = append(d.Strengths, fmt.Sprintf("Feature driver (%d feat)", s.featCount))
		}
		if s.withScope == a.Commits && a.Commits > 0 {
			d.Strengths = append(d.Strengths, "Perfect scope discipline")
		}
		if s.releases >= 3 {
			d.Strengths = append(d.Strengths, fmt.Sprintf("%d-release streak", s.releases))
		}
		if s.refactorCnt >= 2 {
			d.Strengths = append(d.Strengths, fmt.Sprintf("Code cleaner (%d refactor)", s.refactorCnt))
		}

		if s.withBody == 0 && a.Commits > 2 {
			d.Weaknesses = append(d.Weaknesses, "No commit bodies")
		}
		if s.breaking > 0 {
			d.Weaknesses = append(d.Weaknesses, fmt.Sprintf("%d breaking changes", s.breaking))
		}
		if scopeR < 0.5 {
			d.Weaknesses = append(d.Weaknesses, "Low scope usage")
		}

		results = append(results, d)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	for i := range results {
		results[i].Percentile = int(float64(len(results)-i) / float64(len(results)) * 100)
	}

	return results
}

func FormatHTML(results []DeveloperDPI) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<div class="table-wrap" style="margin-top: 16px;"><table><thead><tr><th>Developer</th><th>DPI</th><th>Grade</th><th>Velocity</th><th>Quality</th><th>Impact</th><th>Consistency</th><th>Percentile</th></tr></thead><tbody>`)
	for _, d := range results {
		gradeCls := ""
		switch d.Grade {
		case "S":
			gradeCls = ` style="color: var(--accent); font-weight: 700;"`
		case "A":
			gradeCls = ` style="color: var(--green); font-weight: 700;"`
		case "D":
			gradeCls = ` style="color: var(--red); font-weight: 700;"`
		}
		sb.WriteString(`<tr>`)
		fmt.Fprintf(&sb, `<td>%s</td>`, safehtml.Text(strings.TrimSpace(d.Name)))
		fmt.Fprintf(&sb, `<td style="font-weight: 700; color: var(--text-bright);">%d</td>`, d.Score)
		fmt.Fprintf(&sb, `<td%s>%s</td>`, gradeCls, safehtml.Text(d.Grade))
		fmt.Fprintf(&sb, `<td>%.0f</td>`, d.Velocity)
		fmt.Fprintf(&sb, `<td>%.0f</td>`, d.Quality)
		fmt.Fprintf(&sb, `<td>%.0f</td>`, d.Impact)
		fmt.Fprintf(&sb, `<td>%.0f</td>`, d.Consistency)
		fmt.Fprintf(&sb, `<td>%d%%</td>`, d.Percentile)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	sb.WriteString(`<div style="margin-top: 12px; display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">`)
	for _, d := range results {
		sb.WriteString(`<div style="background: var(--surface); border: 1px solid var(--border); border-radius: 6px; padding: 14px 18px;">`)
		fmt.Fprintf(&sb, `<div style="font-weight: 600; color: var(--text-bright); margin-bottom: 6px;">%s <span style="color: var(--accent); font-weight: 700;">%s</span></div>`, safehtml.Text(strings.TrimSpace(d.Name)), safehtml.Text(d.Grade))
		if len(d.Strengths) > 0 {
			sb.WriteString(`<div style="font-size: 11px; color: var(--green);">`)
			for _, s := range d.Strengths {
				fmt.Fprintf(&sb, `✓ %s &nbsp;`, safehtml.Text(s))
			}
			sb.WriteString(`</div>`)
		}
		if len(d.Weaknesses) > 0 {
			sb.WriteString(`<div style="font-size: 11px; color: var(--red); margin-top: 2px;">`)
			for _, w := range d.Weaknesses {
				fmt.Fprintf(&sb, `✗ %s &nbsp;`, safehtml.Text(w))
			}
			sb.WriteString(`</div>`)
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func safeDiv(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}
