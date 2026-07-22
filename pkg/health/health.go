// Package health analyzes team health signals: workload balance, after-hours work, release contribution concentration, burnout indicators.
package health

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/trends"
)

type Signal struct {
	Name     string
	Status   string
	Value    string
	Detail   string
	Severity string
}

type Report struct {
	Signals        []Signal
	OverallScore   int
	OverallStatus  string
	TrendDirection string
}

func Analyze(commits []commit.Commit, rm metrics.ReportMetrics, snapshots []trends.Snapshot) Report {
	var r Report
	score := 100

	r.Signals = append(r.Signals, analyzeWorkloadBalance(rm, &score))
	r.Signals = append(r.Signals, analyzeAfterHours(rm, commits, &score))
	r.Signals = append(r.Signals, analyzeWeekendWork(rm, &score))
	r.Signals = append(r.Signals, analyzeReleaseContributionConcentration(rm, &score))
	r.Signals = append(r.Signals, analyzeContributorTurnover(snapshots, rm, &score))
	r.Signals = append(r.Signals, analyzeBatchPattern(rm, &score))
	r.Signals = append(r.Signals, analyzeRevertRate(rm, &score))

	if len(snapshots) >= 2 {
		r.Signals = append(r.Signals, analyzeTrendDirection(snapshots, &score))
	}

	r.OverallScore = score
	switch {
	case score >= 80:
		r.OverallStatus = "Healthy"
	case score >= 60:
		r.OverallStatus = "Watch"
	case score >= 40:
		r.OverallStatus = "Concerning"
	default:
		r.OverallStatus = "Critical"
	}

	return r
}

func analyzeWorkloadBalance(rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Workload Balance"}
	if len(rm.Authors) == 0 {
		s.Status = "N/A"
		s.Severity = "neutral"
		return s
	}
	maxC := rm.Authors[0].Commits
	totalC := 0
	for _, a := range rm.Authors {
		totalC += a.Commits
	}
	share := float64(maxC) / float64(totalC) * 100
	s.Value = fmt.Sprintf("%.0f%% by top contributor", share)
	if share > 60 {
		s.Status = "Imbalanced"
		s.Detail = fmt.Sprintf("%s owns %.0f%% of all commits — knowledge silo risk", rm.Authors[0].Name, share)
		s.Severity = "bad"
		*score -= 12
	} else if share > 40 {
		s.Status = "Moderate"
		s.Detail = "Concentration is moderate but watch for silos"
		s.Severity = "warn"
		*score -= 5
	} else {
		s.Status = "Balanced"
		s.Detail = "Workload is well distributed across the team"
		s.Severity = "good"
	}
	return s
}

func analyzeAfterHours(rm metrics.ReportMetrics, commits []commit.Commit, score *int) Signal {
	s := Signal{Name: "After-Hours Activity"}
	if len(commits) == 0 {
		s.Status = "N/A"
		s.Severity = "neutral"
		return s
	}
	afterHours := 0
	for _, c := range commits {
		h := c.Timestamp.Hour()
		if h >= 22 || h < 7 {
			afterHours++
		}
	}
	pct := float64(afterHours) / float64(len(commits)) * 100
	s.Value = fmt.Sprintf("%.0f%% commits after 22:00 or before 07:00", pct)
	if pct > 30 {
		s.Status = "High"
		s.Detail = "Significant after-hours work — possible burnout risk"
		s.Severity = "bad"
		*score -= 10
	} else if pct > 15 {
		s.Status = "Elevated"
		s.Detail = "Some after-hours work detected"
		s.Severity = "warn"
		*score -= 4
	} else {
		s.Status = "Normal"
		s.Detail = "Healthy working hours pattern"
		s.Severity = "good"
	}
	return s
}

func analyzeWeekendWork(rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Weekend Work"}
	wr := rm.Velocity.WeekendRatio * 100
	s.Value = fmt.Sprintf("%.0f%% weekend commits", wr)
	if wr > 20 {
		s.Status = "High"
		s.Detail = "Team is working weekends regularly — burnout indicator"
		s.Severity = "bad"
		*score -= 8
	} else if wr > 5 {
		s.Status = "Some"
		s.Detail = "Occasional weekend work"
		s.Severity = "warn"
		*score -= 3
	} else {
		s.Status = "Minimal"
		s.Detail = "Weekends are respected"
		s.Severity = "good"
	}
	return s
}

func analyzeReleaseContributionConcentration(rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Release Contribution Concentration"}
	s.Value = fmt.Sprintf("%d", rm.ReleaseContributionConcentration)
	if rm.ReleaseContributionConcentration <= 1 {
		s.Status = "Critical"
		s.Detail = "Single point of failure — only 1 person knows the codebase"
		s.Severity = "bad"
		*score -= 15
	} else if rm.ReleaseContributionConcentration <= 2 {
		s.Status = "Low"
		s.Detail = "Knowledge concentrated in 2 people"
		s.Severity = "warn"
		*score -= 6
	} else {
		s.Status = "Healthy"
		s.Detail = fmt.Sprintf("%d+ people can maintain the codebase", rm.ReleaseContributionConcentration)
		s.Severity = "good"
	}
	return s
}

func analyzeContributorTurnover(snapshots []trends.Snapshot, rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Contributor Stability"}
	if len(snapshots) == 0 {
		s.Status = "N/A"
		s.Value = "No historical data"
		s.Severity = "neutral"
		return s
	}
	prevContribs := make(map[string]bool)
	if len(snapshots) > 0 {
		for _, tc := range snapshots[len(snapshots)-1].TopContributors {
			prevContribs[tc.Name] = true
		}
	}
	newContribs := 0
	returningCount := 0
	for _, a := range rm.Authors {
		if prevContribs[a.Name] {
			returningCount++
		} else {
			newContribs++
		}
	}
	s.Value = fmt.Sprintf("%d returning, %d new", returningCount, newContribs)
	if newContribs > returningCount && returningCount > 0 {
		s.Status = "High turnover"
		s.Detail = "More new contributors than returning — team churn risk"
		s.Severity = "warn"
		*score -= 5
	} else if returningCount == 0 && len(rm.Authors) > 0 {
		s.Status = "All new team"
		s.Detail = "No returning contributors from previous release"
		s.Severity = "warn"
		*score -= 3
	} else {
		s.Status = "Stable"
		s.Detail = "Core team is retained across releases"
		s.Severity = "good"
	}
	return s
}

func analyzeBatchPattern(rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Commit Cadence"}
	bf := rm.Velocity.BatchFactor
	s.Value = fmt.Sprintf("Batch factor %.2f", bf)
	if bf > 2.0 {
		s.Status = "Erratic"
		s.Detail = "Commits come in bursts — irregular workflow"
		s.Severity = "warn"
		*score -= 4
	} else if bf > 1.0 {
		s.Status = "Moderate"
		s.Detail = "Some batching but generally steady"
		s.Severity = "neutral"
	} else {
		s.Status = "Steady"
		s.Detail = "Consistent commit cadence"
		s.Severity = "good"
	}
	return s
}

func analyzeRevertRate(rm metrics.ReportMetrics, score *int) Signal {
	s := Signal{Name: "Stability"}
	rr := rm.RevertRate
	s.Value = fmt.Sprintf("%.1f%% revert rate", rr)
	if rr > 10 {
		s.Status = "Unstable"
		s.Detail = "High revert rate — quality issues in commits"
		s.Severity = "bad"
		*score -= 10
	} else if rr > 3 {
		s.Status = "Watch"
		s.Detail = "Some reverts — review commit quality"
		s.Severity = "warn"
		*score -= 3
	} else {
		s.Status = "Stable"
		s.Detail = "Low revert rate — commits are landing cleanly"
		s.Severity = "good"
	}
	return s
}

func analyzeTrendDirection(snapshots []trends.Snapshot, score *int) Signal {
	s := Signal{Name: "Health Trend"}
	if len(snapshots) < 2 {
		s.Status = "N/A"
		s.Severity = "neutral"
		return s
	}
	prev := snapshots[len(snapshots)-2]
	curr := snapshots[len(snapshots)-1]
	riskDelta := curr.ReleaseRiskScore - prev.ReleaseRiskScore
	if riskDelta > 5 {
		s.Status = "Declining"
		s.Value = fmt.Sprintf("Risk +%.0f", riskDelta)
		s.Detail = "Release risk is trending up"
		s.Severity = "bad"
		*score -= 5
	} else if riskDelta < -5 {
		s.Status = "Improving"
		s.Value = fmt.Sprintf("Risk %.0f", riskDelta)
		s.Detail = "Release risk is trending down"
		s.Severity = "good"
	} else {
		s.Status = "Stable"
		s.Value = fmt.Sprintf("Risk %+.0f", riskDelta)
		s.Detail = "Risk level is stable"
		s.Severity = "neutral"
	}
	return s
}

func FormatHTML(r Report) string {
	if len(r.Signals) == 0 {
		return ""
	}
	var sb strings.Builder

	scoreCls := "var(--green)"
	if r.OverallScore < 60 {
		scoreCls = "var(--red)"
	} else if r.OverallScore < 80 {
		scoreCls = "var(--accent)"
	}

	sb.WriteString(`<div style="display: flex; align-items: center; gap: 20px; margin-bottom: 16px;">`)
	fmt.Fprintf(&sb, `<div style="font-family: var(--mono); font-size: 36px; font-weight: 700; color: %s;">%d</div>`, scoreCls, r.OverallScore)
	fmt.Fprintf(&sb, `<div><div style="font-size: 10px; color: var(--text-dim); text-transform: uppercase; letter-spacing: 1.5px;">Team Health Score</div><div style="font-size: 14px; color: var(--text-bright); font-weight: 600;">%s</div></div>`, safehtml.Text(r.OverallStatus))
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="table-wrap"><table><thead><tr><th>Signal</th><th>Status</th><th>Value</th><th>Detail</th></tr></thead><tbody>`)
	for _, sig := range r.Signals {
		cls := ""
		switch sig.Severity {
		case "good":
			cls = ` style="color: var(--green);"`
		case "bad":
			cls = ` style="color: var(--red);"`
		case "warn":
			cls = ` style="color: var(--accent);"`
		}
		fmt.Fprintf(&sb, `<tr><td>%s</td><td%s>%s</td><td>%s</td><td style="font-size: 11px; color: var(--text-dim);">%s</td></tr>`,
			safehtml.Text(sig.Name), cls, safehtml.Text(sig.Status), safehtml.Text(sig.Value), safehtml.Text(sig.Detail))
	}
	sb.WriteString(`</tbody></table></div>`)
	return sb.String()
}
