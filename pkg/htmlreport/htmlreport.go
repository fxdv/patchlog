// Package htmlreport generates standalone HTML reports with metric tables, tooltips, DPI, health signals, and ownership heatmaps.
package htmlreport

import (
	"context"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/dpi"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/health"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/ownership"
	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/timeline"
	"github.com/fxdv/patchlog/pkg/trends"
)

type ReportData struct {
	Version   string
	Date      string
	Report    render.Report
	Metrics   metrics.ReportMetrics
	CodeStats metrics.CodeStats
	Summary   ai.SummaryMetrics
	Snapshots []trends.Snapshot
	AISummary string
	Commits   []commit.Commit
	Fetcher   *gitlog.Fetcher
	Ctx       context.Context
	Labs      bool
}

func Generate(data ReportData) string {
	var sb strings.Builder
	sb.WriteString(htmlHead(data.Version, data.Date))
	sb.WriteString(htmlBody(data))
	sb.WriteString(htmlTail())
	return sb.String()
}

func htmlHead(version, date string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Release Analytics — %s</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap');
:root {
  --bg: #fafbfc;
  --surface: #ffffff;
  --hover: #f4f6f8;
  --border: #e8eaed;
  --border-soft: #f0f1f3;
  --accent: #4f46e5;
  --accent-soft: #eef0fa;
  --green: #059669;
  --green-soft: #ecfdf5;
  --red: #dc2626;
  --red-soft: #fef2f2;
  --text: #374151;
  --text-dim: #9ca3af;
  --text-bright: #111827;
  --sans: 'Inter', -apple-system, sans-serif;
  --mono: 'JetBrains Mono', 'SF Mono', monospace;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: var(--sans); background: var(--bg); color: var(--text); -webkit-font-smoothing: antialiased; }
.container { max-width: 1280px; margin: 0 auto; padding: 48px 32px; }
.header { margin-bottom: 32px; }
.header h1 { font-size: 24px; font-weight: 400; color: var(--text-bright); letter-spacing: -0.5px; }
.header h1 .ver { color: var(--accent); font-weight: 600; }
.header h1 .tag { font-size: 10px; font-weight: 600; letter-spacing: 1px; background: var(--accent-soft); color: var(--accent); padding: 3px 10px; border-radius: 4px; margin-left: 12px; vertical-align: middle; text-transform: uppercase; }
.header .meta { font-size: 12px; color: var(--text-dim); margin-top: 6px; font-family: var(--mono); }
.summary { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 28px 36px; margin-bottom: 28px; }
.summary .label { font-size: 10px; color: var(--accent); letter-spacing: 2px; font-weight: 700; margin-bottom: 12px; }
.summary p { font-size: 14px; line-height: 1.8; color: var(--text); }
.table-wrap { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
table { width: 100%%; border-collapse: collapse; font-family: var(--mono); font-size: 12px; }
thead th { padding: 14px 20px; text-align: right; background: var(--surface); color: var(--text-dim); font-size: 10px; text-transform: uppercase; letter-spacing: 1px; font-weight: 600; border-bottom: 1px solid var(--border); white-space: nowrap; }
thead th:first-child { text-align: left; color: var(--text-bright); }
thead th:last-child { color: var(--accent); }
tbody td { padding: 10px 20px; text-align: right; border-bottom: 1px solid var(--border-soft); color: var(--text); white-space: nowrap; font-variant-numeric: tabular-nums; }
tbody td:first-child { text-align: left; color: var(--text-dim); font-family: var(--sans); font-size: 12px; font-weight: 500; }
tbody td:last-child { color: var(--text-bright); font-weight: 600; }
tbody tr { transition: background 0.1s; }
tbody tr:hover { background: var(--hover); }
tbody tr:hover td:first-child { color: var(--text-bright); }
.sec-row td { background: var(--accent-soft); color: var(--accent); font-size: 9px; text-transform: uppercase; letter-spacing: 2px; padding: 8px 20px; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border); font-weight: 700; font-family: var(--sans); }
.up-good { color: var(--green); font-weight: 700; }
.up-bad { color: var(--red); font-weight: 700; }
.dn-good { color: var(--green); font-weight: 700; }
.dn-bad { color: var(--red); font-weight: 700; }
.flat { color: var(--text-dim); }
.val-good { color: var(--green); }
.val-bad { color: var(--red); }
.val-warn { color: var(--accent); }
.legend { display: flex; gap: 24px; margin-top: 20px; padding: 0 4px; font-size: 11px; color: var(--text-dim); font-family: var(--sans); }
.legend .item { display: flex; align-items: center; gap: 6px; }
.legend .dot { width: 8px; height: 8px; border-radius: 50%%; }
.legend .dot.green { background: var(--green); }
.legend .dot.red { background: var(--red); }
.legend .dot.indigo { background: var(--accent); }
.section-spacer { height: 8px; }
.metric-name { position: relative; cursor: help; border-bottom: 1px dotted var(--text-dim); }
.metric-name:hover { border-bottom-color: var(--accent); }
.tooltip { display: none; position: absolute; left: 0; top: 100%%; z-index: 100; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; padding: 10px 14px; font-size: 11px; font-family: var(--sans); font-weight: 400; color: var(--text); line-height: 1.5; width: 320px; max-width: 90vw; box-shadow: 0 4px 16px rgba(0,0,0,0.08); margin-top: 6px; white-space: normal; text-align: left; }
.metric-name:hover .tooltip, .metric-name:focus .tooltip { display: block; }
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>Release Analytics <span class="ver">%s</span><span class="tag">Report</span></h1>
<div class="meta">%s</div>
</div>`, safehtml.Text(version), safehtml.Text(version), safehtml.Text(date))
}

func htmlBody(data ReportData) string {
	var sb strings.Builder

	if data.AISummary != "" {
		sb.WriteString(`<div class="summary"><div class="label">EXECUTIVE SUMMARY</div><p>`)
		sb.WriteString(safehtml.Text(data.AISummary))
		sb.WriteString(`</p></div>`)
	}

	if len(data.Snapshots) >= 2 {
		sb.WriteString(renderGrandTable(data.Snapshots))
	} else {
		sb.WriteString(renderSingleReleaseTable(data))
	}

	sb.WriteString(`<div class="legend">`)
	sb.WriteString(`<div class="item"><div class="dot green"></div>Positive trend</div>`)
	sb.WriteString(`<div class="item"><div class="dot red"></div>Negative trend</div>`)
	sb.WriteString(`<div class="item"><div class="dot indigo"></div>Section header</div>`)
	sb.WriteString(`<div class="item">→ arrows indicate direction vs previous release</div>`)
	sb.WriteString(`</div>`)

	if len(data.Snapshots) > 0 {
		sb.WriteString(`<div class="section-spacer"></div>`)
		sb.WriteString(renderSectionHeader("Release Timeline"))
		sb.WriteString(timeline.FormatHTML(data.Snapshots))
	}

	if data.Labs && len(data.Commits) > 0 && len(data.Metrics.Authors) > 0 {
		sb.WriteString(`<div class="section-spacer"></div>`)
		sb.WriteString(renderSectionHeader("Developer Productivity Index"))
		dpiResults := dpi.Compute(data.Commits, data.Metrics, data.Snapshots)
		sb.WriteString(dpi.FormatHTML(dpiResults))

		sb.WriteString(`<div class="section-spacer"></div>`)
		sb.WriteString(renderSectionHeader("Team Health Signals"))
		healthReport := health.Analyze(data.Commits, data.Metrics, data.Snapshots)
		sb.WriteString(health.FormatHTML(healthReport))
	}

	if data.Fetcher != nil && len(data.Commits) > 0 {
		sb.WriteString(`<div class="section-spacer"></div>`)
		sb.WriteString(renderSectionHeader("Code Ownership Heatmap"))
		fileOwn, dirOwn := ownership.Compute(data.Ctx, data.Fetcher, data.Commits)
		sb.WriteString(ownership.FormatHTML(fileOwn, dirOwn))
	}

	return sb.String()
}

func renderSectionHeader(title string) string {
	return fmt.Sprintf(`<div style="margin: 32px 0 16px;"><h2 style="font-size: 14px; font-weight: 600; color: var(--text-bright); letter-spacing: -0.2px;">%s</h2><div style="height: 2px; width: 32px; background: var(--accent); margin-top: 8px; border-radius: 1px;"></div></div>`, safehtml.Text(title))
}

type metricDef struct {
	label   string
	hint    string
	get     func(s trends.Snapshot) float64
	format  string
	inverse bool
}

type metricSection struct {
	title string
	rows  []metricDef
}

func renderGrandTable(snapshots []trends.Snapshot) string {
	var sb strings.Builder
	sb.WriteString(`<div class="table-wrap"><table><thead><tr><th>Metric</th>`)
	for _, s := range snapshots {
		short := s.Version
		if len(short) > 16 {
			short = short[:14] + "…"
		}
		fmt.Fprintf(&sb, `<th>%s</th>`, safehtml.Text(short))
	}
	sb.WriteString(`</tr></thead><tbody>`)

	sections := []metricSection{
		{"Overview", []metricDef{
			{"Commits", "Total number of commits in this release range", func(s trends.Snapshot) float64 { return float64(s.TotalCommits) }, "%.0f", false},
			{"Authors", "Unique developers who contributed commits", func(s trends.Snapshot) float64 { return float64(s.TotalAuthors) }, "%.0f", false},
			{"Release Contribution Concentration", "Number of contributors accounting for 80% of this release's commits. This is not a knowledge or maintainership measure.", func(s trends.Snapshot) float64 { return float64(s.ReleaseContributionConcentration) }, "%.0f", false},
			{"Breaking Changes", "Commits with breaking API or behavior changes. Should ideally be zero or well-documented.", func(s trends.Snapshot) float64 { return float64(s.BreakingChanges) }, "%.0f", true},
			{"Files Touched", "Unique files modified across all commits in this release", func(s trends.Snapshot) float64 { return float64(s.FilesTouched) }, "%.0f", false},
			{"Lines Added", "Total lines of code inserted across all commits", func(s trends.Snapshot) float64 { return float64(s.LinesAdded) }, "+%.0f", false},
			{"Lines Deleted", "Total lines of code removed. High values may indicate cleanup or refactoring.", func(s trends.Snapshot) float64 { return float64(s.LinesDeleted) }, "-%.0f", true},
			{"Net Lines", "Lines added minus lines deleted. Positive = codebase growing, negative = shrinking.", func(s trends.Snapshot) float64 { return float64(s.NetLines) }, "%+.0f", false},
			{"Jira Tickets", "Number of Jira tickets referenced in commit messages", func(s trends.Snapshot) float64 { return float64(s.JiraTickets) }, "%.0f", false},
		}},
		{"Velocity", []metricDef{
			{"Release Commit Span (h)", "Hours from first to last included commit. This is not a PR or deployment timestamp metric.", func(s trends.Snapshot) float64 { return s.ReleaseCommitSpanHours }, "%.0f", true},
			{"Release Age (h)", "Hours from first commit to current time. Measures how long ago work started.", func(s trends.Snapshot) float64 { return s.ReleaseAgeHours }, "%.0f", true},
			{"Commits / Day", "Average commits per day across the release cycle", func(s trends.Snapshot) float64 { return s.CommitsPerDay }, "%.1f", false},
			{"Batch Factor", "Standard deviation of gaps between commits / mean gap. High = irregular cadence (bursts). Low = steady flow.", func(s trends.Snapshot) float64 { return s.BatchFactor }, "%.2f", true},
			{"Conventional Ratio", "Percentage of commits following conventional commit format (type(scope): description)", func(s trends.Snapshot) float64 { return s.ConventionalRatio * 100 }, "%.0f%%", false},
			{"Rhythm Consistency", "How steady the commit cadence is (0-100%). High = regular intervals, low = erratic bursts.", func(s trends.Snapshot) float64 { return s.RhythmConsistency * 100 }, "%.0f%%", false},
		}},
		{"Code Metrics", []metricDef{
			{"Release Risk Score", "Composite score (0-100): breaking changes + hotspot density + ownership concentration. Higher = riskier release.", func(s trends.Snapshot) float64 { return s.ReleaseRiskScore }, "%.0f", true},
			{"Hotspot Score", "Score (0-100) measuring code churn concentration. High = a few files changed excessively.", func(s trends.Snapshot) float64 { return s.HotspotScore }, "%.0f", true},
			{"Technical Debt ($)", "Estimated cost of code maintenance: (churn × $0.50 + fix commits × $15 + refactor × $25) × complexity multiplier", func(s trends.Snapshot) float64 { return s.TechDebtUSD }, "%.0f", true},
			{"Change Complexity Proxy", "Heuristic derived from changed lines, files, and churn. It is not language-aware cognitive complexity.", func(s trends.Snapshot) float64 { return s.ChangeComplexityProxy }, "%.1f", true},
			{"Cross-Cutting Change Risk", "Heuristic based on cross-cutting commits, scope isolation, and touched API files. It is not a dependency graph.", func(s trends.Snapshot) float64 { return s.CrossCuttingChangeRisk }, "%.0f%%", true},
			{"Churn Factor", "Total churn (added + deleted) ÷ abs(net lines). High = lots of rework relative to net output.", func(s trends.Snapshot) float64 { return s.ChurnFactor }, "%.1fx", true},
			{"Complexity per Feature", "Average lines of churn per feature commit. High = features are heavyweight.", func(s trends.Snapshot) float64 { return s.ComplexityPerFeat }, "%.0f", true},
			{"File Volatility", "Average changes per file. High = files are being modified repeatedly.", func(s trends.Snapshot) float64 { return s.FileVolatility }, "%.1f", true},
			{"Hotspot Density", "Percentage of changes concentrated in top 5 files. High = changes are clustered.", func(s trends.Snapshot) float64 { return s.HotspotDensity }, "%.0f%%", true},
		}},
		{"Quality & Health", []metricDef{
			{"Ownership Concentration", "HHI index (0-100%) measuring how concentrated commit ownership is. High = one person dominates.", func(s trends.Snapshot) float64 { return s.OwnershipConc }, "%.0f%%", true},
			{"Ownership Entropy", "Shannon entropy (0-1) of commit distribution. High = knowledge is well shared. Low = silos.", func(s trends.Snapshot) float64 { return s.OwnershipEntropy }, "%.2f", false},
			{"Fix-to-Feature Ratio", "Bug fix commits per feature commit. High = more fixing than building (quality debt).", func(s trends.Snapshot) float64 { return s.FixToFeatureRatio }, "%.1f:1", true},
			{"Test-to-Source Ratio", "Touched test files divided by touched source files. Descriptive only; it is not code coverage.", func(s trends.Snapshot) float64 { return s.TestToSourceRatio }, "%.0f%%", false},
			{"Refactoring Ratio", "Percentage of commits that are refactors or fixes. High = paying down debt vs new features.", func(s trends.Snapshot) float64 { return s.RefactoringRatio }, "%.0f%%", false},
			{"Revert Rate", "Percentage of commits that are reverts. High = instability or quality issues.", func(s trends.Snapshot) float64 { return s.RevertRate }, "%.1f%%", true},
			{"Scope Isolation", "Percentage of commits with a scope tag. High = well-organized, focused commits.", func(s trends.Snapshot) float64 { return s.ScopeIsolation }, "%.0f%%", false},
			{"Cross-Cutting Concerns", "Percentage of commits touching multiple file categories (e.g. both API and UI). High = scattered changes.", func(s trends.Snapshot) float64 { return s.CrossCuttingPct }, "%.0f%%", true},
			{"API Surface Change", "Number of API contract files changed (openapi, proto, graphql). Non-zero = potential breaking changes.", func(s trends.Snapshot) float64 { return float64(s.APISurfaceChange) }, "%.0f", true},
			{"Touched Test File Ratio", "Touched test files divided by touched test and source files. It is not code coverage; use CI artifacts for coverage.", func(s trends.Snapshot) float64 { return s.TouchedTestFileRatio }, "%.0f%%", false},
		}},
		{"Commit Anatomy", []metricDef{
			{"Commit Size S/M/L/H", "Distribution of commit sizes: Small (<3 files), Medium (3-9), Large (10-29), Huge (30+). Healthy = mostly S/M.", func(s trends.Snapshot) float64 { return float64(s.CommitSizeLarge + s.CommitSizeHuge) }, "%d/%d/%d/%d", false},
			{"Message Quality", "Composite score (0-100): body presence + scope usage + conventional format + header length", func(s trends.Snapshot) float64 { return s.CommitMsgQuality }, "%.0f/100", false},
			{"Review Load", "Largest commit size ÷ average commit size. High = one mega-commit likely skipped proper review.", func(s trends.Snapshot) float64 { return s.ReviewLoad }, "%.1fx", true},
			{"Delete Ratio", "Deleted lines ÷ total churn (%). High = code cleanup. Low = pure accumulation.", func(s trends.Snapshot) float64 { return s.DeleteRatio * 100 }, "%.0f%%", false},
			{"Author Overlap", "Percentage of files touched by 2+ authors. High = shared ownership. Low = siloed work.", func(s trends.Snapshot) float64 { return s.AuthorOverlap }, "%.0f%%", false},
			{"Peak Hour σ", "Standard deviation of commit hours. High = flexible schedule. Low = everyone commits at the same time.", func(s trends.Snapshot) float64 { return s.PeakHourSpread }, "%.1f", false},
		}},
		{"Code Health", []metricDef{
			{"Directory Breadth", "Unique top-level directories touched ÷ total files (%). High = broad blast radius.", func(s trends.Snapshot) float64 { return s.DirectoryBreadth }, "%.0f%%", true},
			{"Config Churn Rate", "Config file changes (yaml, toml, json) ÷ total file changes (%). High = infrastructure instability.", func(s trends.Snapshot) float64 { return s.ConfigChurnRate }, "%.0f%%", true},
			{"Code Freshness (days)", "Days since the earliest commit in this release. Low = modifying recent code. High = revisiting old code.", func(s trends.Snapshot) float64 { return s.CodeFreshness }, "%.0f", false},
			{"Language Mix (Cyrillic)", "Percentage of commits with Cyrillic characters in the message. Indicates multi-language team.", func(s trends.Snapshot) float64 { return s.LanguageMix * 100 }, "%.0f%%", false},
		}},
	}

	for _, sec := range sections {
		fmt.Fprintf(&sb, `<tr class="sec-row"><td colspan="%d">%s</td></tr>`, len(snapshots)+1, safehtml.Text(sec.title))
		for _, row := range sec.rows {
			fmt.Fprintf(&sb, `<tr><td><span class="metric-name" tabindex="0">%s<span class="tooltip">%s</span></span></td>`, safehtml.Text(row.label), safehtml.Text(row.hint))
			for i, s := range snapshots {
				val := row.get(s)
				var display string
				if row.label == "Commit Size S/M/L/H" {
					display = fmt.Sprintf("%d/%d/%d/%d", s.CommitSizeSmall, s.CommitSizeMedium, s.CommitSizeLarge, s.CommitSizeHuge)
				} else {
					display = fmt.Sprintf(row.format, val)
				}
				cls := ""
				if i == len(snapshots)-1 {
					cls = ` class="last"`
				}
				arrow := ""
				arrowCls := "flat"
				if i > 0 {
					prev := row.get(snapshots[i-1])
					if val > prev {
						arrow = " ↑"
						if row.inverse {
							arrowCls = "up-bad"
						} else {
							arrowCls = "up-good"
						}
					} else if val < prev {
						arrow = " ↓"
						if row.inverse {
							arrowCls = "dn-good"
						} else {
							arrowCls = "dn-bad"
						}
					}
				}
				fmt.Fprintf(&sb, `<td%s>%s<span class="%s">%s</span></td>`, cls, display, arrowCls, arrow)
			}
			sb.WriteString(`</tr>`)
		}
	}

	sb.WriteString(`</tbody></table></div>`)
	return sb.String()
}

func renderSingleReleaseTable(data ReportData) string {
	var sb strings.Builder
	sb.WriteString(`<div class="table-wrap"><table><thead><tr><th>Metric</th><th>Value</th></tr></thead><tbody>`)
	m := data.Metrics
	cs := data.CodeStats
	sm := data.Summary

	type kv struct {
		k   string
		v   string
		cls string
	}
	rows := []struct {
		section string
		items   []kv
	}{
		{"Overview", []kv{
			{"Commits", fmt.Sprintf("%d", m.TotalCommits), ""},
			{"Authors", fmt.Sprintf("%d", m.TotalAuthors), ""},
			{"Release Contribution Concentration", fmt.Sprintf("%d", m.ReleaseContributionConcentration), ""},
			{"Breaking Changes", fmt.Sprintf("%d", m.BreakingChanges), ""},
			{"Files Touched", fmt.Sprintf("%d", cs.TotalFiles), ""},
			{"Lines Added", fmt.Sprintf("+%d", cs.TotalAdditions), "val-good"},
			{"Lines Deleted", fmt.Sprintf("-%d", cs.TotalDeletions), "val-bad"},
			{"Net Lines", fmt.Sprintf("%+d", cs.NetLines), ""},
			{"Jira Tickets", fmt.Sprintf("%d", m.JiraTicketsLinked), ""},
		}},
		{"Code Metrics", []kv{
			{"Risk Score", fmt.Sprintf("%.0f/100", sm.ReleaseRiskScore), ""},
			{"Hotspot Score", fmt.Sprintf("%.0f/100", sm.HotspotScore), ""},
			{"Technical Debt", fmt.Sprintf("$%.0f", sm.TechnicalDebtUSD), "val-bad"},
			{"Change Complexity Proxy", fmt.Sprintf("%.1f", sm.ChangeComplexityProxy), ""},
			{"Churn Factor", fmt.Sprintf("%.1fx", sm.ChurnFactor), ""},
		}},
		{"Quality", []kv{
			{"Ownership Concentration", fmt.Sprintf("%.0f%%", sm.OwnershipConc), ""},
			{"Fix/Feature Ratio", fmt.Sprintf("%.1f:1", sm.FixToFeatureRatio), ""},
			{"Touched Test File Ratio", fmt.Sprintf("%.0f%%", sm.TouchedTestFileRatio), ""},
			{"Review Load", fmt.Sprintf("%.1fx", m.ReviewLoad), ""},
			{"Message Quality", fmt.Sprintf("%.0f/100", m.CommitMsgQuality), ""},
		}},
	}

	for _, sec := range rows {
		fmt.Fprintf(&sb, `<tr class="sec-row"><td colspan="2">%s</td></tr>`, safehtml.Text(sec.section))
		for _, item := range sec.items {
			cls := ""
			if item.cls != "" {
				cls = fmt.Sprintf(` class="%s"`, item.cls)
			}
			fmt.Fprintf(&sb, `<tr><td>%s</td><td%s>%s</td></tr>`, safehtml.Text(item.k), cls, item.v)
		}
	}
	sb.WriteString(`</tbody></table></div>`)
	return sb.String()
}

func htmlTail() string {
	return `</div></body></html>`
}
