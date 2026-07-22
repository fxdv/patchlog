package confluence

import (
	"bytes"
	"fmt"

	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/trends"
)

type TrendsThresholds struct {
	ReleaseCommitSpanWarning            float64
	ReleaseCommitSpanCritical           float64
	TechDebtWarning                     float64
	TechDebtCritical                    float64
	ReleaseContributionConcentrationMin int
}

type trendMetricRow struct {
	label     string
	values    []float64
	displays  []string
	metricKey string
}

func RenderTrendsPanel(snapshots []trends.Snapshot, th TrendsThresholds) string {
	return RenderTrendsPanelWithGamification(snapshots, th, "")
}

func RenderTrendsPanelWithGamification(snapshots []trends.Snapshot, th TrendsThresholds, gamificationHTML string) string {
	if len(snapshots) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("<h1>Release Trends</h1>")
	buf.WriteString(Spacer())

	if len(snapshots) == 1 {
		renderTrendsSingle(&buf, snapshots[0])
	} else {
		renderTrendsMulti(&buf, snapshots, th)
	}

	if gamificationHTML != "" {
		buf.WriteString(Spacer())
		buf.WriteString(gamificationHTML)
	} else {
		buf.WriteString(Spacer())
		renderTopContributorsAcrossReleases(&buf, snapshots)
	}

	return buf.String()
}

func renderTrendsSingle(buf *bytes.Buffer, snap trends.Snapshot) {
	buf.WriteString("<ac:structured-macro ac:name=\"info\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">Release Snapshot</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString(Spacer())

	renderSnapshotFullTable(buf, snap)

	buf.WriteString(Spacer())
	buf.WriteString(`<p style="color: #999;">Need at least 2 releases to show cross-release trends.</p>`)
	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
}

func renderSnapshotFullTable(buf *bytes.Buffer, snap trends.Snapshot) {
	buf.WriteString(`<table><tbody>`)

	fmt.Fprintf(buf, "<tr><th colspan=\"2\" style=\"background: #f4f5f7;\">Overview</th></tr>")
	fmt.Fprintf(buf, "<tr><td><strong>Version</strong></td><td>%s</td></tr>", safehtml.Text(snap.Version))
	fmt.Fprintf(buf, "<tr><td><strong>Date</strong></td><td>%s</td></tr>", safehtml.Text(snap.Date))
	fmt.Fprintf(buf, "<tr><td><strong>Commits</strong></td><td>%d</td></tr>", snap.TotalCommits)
	fmt.Fprintf(buf, "<tr><td><strong>Authors</strong></td><td>%d</td></tr>", snap.TotalAuthors)
	fmt.Fprintf(buf, "<tr><td><strong>Release contribution concentration</strong></td><td>%d</td></tr>", snap.ReleaseContributionConcentration)
	fmt.Fprintf(buf, "<tr><td><strong>Breaking changes</strong></td><td>%d</td></tr>", snap.BreakingChanges)
	fmt.Fprintf(buf, "<tr><td><strong>Files touched</strong></td><td>%d</td></tr>", snap.FilesTouched)
	fmt.Fprintf(buf, "<tr><td><strong>Lines added</strong></td><td><span style=\"color: %s;\">+%d</span></td></tr>", colorGreen, snap.LinesAdded)
	fmt.Fprintf(buf, "<tr><td><strong>Lines deleted</strong></td><td><span style=\"color: %s;\">-%d</span></td></tr>", colorRed, snap.LinesDeleted)
	fmt.Fprintf(buf, "<tr><td><strong>Net lines</strong></td><td><strong>%+d</strong></td></tr>", snap.NetLines)
	if snap.JiraTickets > 0 {
		fmt.Fprintf(buf, "<tr><td><strong>Jira tickets</strong></td><td>%d</td></tr>", snap.JiraTickets)
	}

	fmt.Fprintf(buf, "<tr><th colspan=\"2\" style=\"background: #f4f5f7;\">Velocity</th></tr>")
	fmt.Fprintf(buf, "<tr><td><strong>Release commit span</strong></td><td>%s</td></tr>", safehtml.Text(formatDuration(snap.ReleaseCommitSpanHours)))
	fmt.Fprintf(buf, "<tr><td><strong>Release age</strong></td><td>%s</td></tr>", safehtml.Text(formatDuration(snap.ReleaseAgeHours)))
	fmt.Fprintf(buf, "<tr><td><strong>Commits/day</strong></td><td>%.1f</td></tr>", snap.CommitsPerDay)
	fmt.Fprintf(buf, "<tr><td><strong>Batch factor</strong></td><td>%.2f</td></tr>", snap.BatchFactor)
	fmt.Fprintf(buf, "<tr><td><strong>Conv. ratio</strong></td><td>%.0f%%</td></tr>", snap.ConventionalRatio*100)

	fmt.Fprintf(buf, "<tr><th colspan=\"2\" style=\"background: #f4f5f7;\">Code Metrics</th></tr>")
	fmt.Fprintf(buf, "<tr><td><strong>Release Risk Score</strong></td><td>%.0f/100</td></tr>", snap.ReleaseRiskScore)
	fmt.Fprintf(buf, "<tr><td><strong>Hotspot Score</strong></td><td>%.0f/100</td></tr>", snap.HotspotScore)
	fmt.Fprintf(buf, "<tr><td><strong>Tech debt</strong></td><td>$%.0f</td></tr>", snap.TechDebtUSD)
	fmt.Fprintf(buf, "<tr><td><strong>Change Complexity Proxy</strong></td><td>%.1f</td></tr>", snap.ChangeComplexityProxy)
	fmt.Fprintf(buf, "<tr><td><strong>Cross-Cutting Change Risk</strong></td><td>%.0f%%</td></tr>", snap.CrossCuttingChangeRisk)
	fmt.Fprintf(buf, "<tr><td><strong>Churn Factor</strong></td><td>%.1fx</td></tr>", snap.ChurnFactor)
	fmt.Fprintf(buf, "<tr><td><strong>Complexity per Feature</strong></td><td>%.0f</td></tr>", snap.ComplexityPerFeat)
	fmt.Fprintf(buf, "<tr><td><strong>File Volatility</strong></td><td>%.1f</td></tr>", snap.FileVolatility)
	fmt.Fprintf(buf, "<tr><td><strong>Hotspot Density</strong></td><td>%.0f%%</td></tr>", snap.HotspotDensity)

	fmt.Fprintf(buf, "<tr><th colspan=\"2\" style=\"background: #f4f5f7;\">Quality</th></tr>")
	fmt.Fprintf(buf, "<tr><td><strong>Ownership Concentration</strong></td><td>%.0f%%</td></tr>", snap.OwnershipConc)
	fmt.Fprintf(buf, "<tr><td><strong>Ownership Entropy</strong></td><td>%.2f</td></tr>", snap.OwnershipEntropy)
	fmt.Fprintf(buf, "<tr><td><strong>Fix-to-Feature Ratio</strong></td><td>%.1f:1</td></tr>", snap.FixToFeatureRatio)
	fmt.Fprintf(buf, "<tr><td><strong>Test-to-Source Ratio</strong></td><td>%.0f%%</td></tr>", snap.TestToSourceRatio)
	fmt.Fprintf(buf, "<tr><td><strong>Refactoring Ratio</strong></td><td>%.0f%%</td></tr>", snap.RefactoringRatio)
	fmt.Fprintf(buf, "<tr><td><strong>Revert Rate</strong></td><td>%.1f%%</td></tr>", snap.RevertRate)
	fmt.Fprintf(buf, "<tr><td><strong>Scope Isolation</strong></td><td>%.0f%%</td></tr>", snap.ScopeIsolation)
	fmt.Fprintf(buf, "<tr><td><strong>Cross-Cutting Concerns</strong></td><td>%.0f%%</td></tr>", snap.CrossCuttingPct)
	fmt.Fprintf(buf, "<tr><td><strong>API Surface Change</strong></td><td>%d files</td></tr>", snap.APISurfaceChange)
	fmt.Fprintf(buf, "<tr><td><strong>Touched Test File Ratio</strong></td><td>%.0f%%</td></tr>", snap.TouchedTestFileRatio)

	buf.WriteString("</tbody></table>")
}

func renderTrendsMulti(buf *bytes.Buffer, snapshots []trends.Snapshot, th TrendsThresholds) {
	n := len(snapshots)

	versions := make([]string, n)
	dates := make([]string, n)
	for i, s := range snapshots {
		versions[i] = s.Version
		dates[i] = s.Date
	}

	sections := [][]trendMetricRow{
		buildTrendSection(
			[]trendMetricDef{
				{"Commits", func(s trends.Snapshot) float64 { return float64(s.TotalCommits) }, "%.0f", ""},
				{"Authors", func(s trends.Snapshot) float64 { return float64(s.TotalAuthors) }, "%.0f", ""},
				{"Release contribution concentration", func(s trends.Snapshot) float64 { return float64(s.ReleaseContributionConcentration) }, "%.0f", "release_contribution_concentration"},
				{"Breaking", func(s trends.Snapshot) float64 { return float64(s.BreakingChanges) }, "%.0f", ""},
				{"Files touched", func(s trends.Snapshot) float64 { return float64(s.FilesTouched) }, "%.0f", ""},
				{"Lines added", func(s trends.Snapshot) float64 { return float64(s.LinesAdded) }, "%+.0f", ""},
				{"Lines deleted", func(s trends.Snapshot) float64 { return float64(s.LinesDeleted) }, "%-.0f", ""},
				{"Net lines", func(s trends.Snapshot) float64 { return float64(s.NetLines) }, "%+.0f", ""},
				{"Jira tickets", func(s trends.Snapshot) float64 { return float64(s.JiraTickets) }, "%.0f", ""},
			},
			snapshots,
		),
		buildTrendSection(
			[]trendMetricDef{
				{"Release commit span (h)", func(s trends.Snapshot) float64 { return s.ReleaseCommitSpanHours }, "%.0f", "release_commit_span"},
				{"Release age (h)", func(s trends.Snapshot) float64 { return s.ReleaseAgeHours }, "%.0f", ""},
				{"Commits/day", func(s trends.Snapshot) float64 { return s.CommitsPerDay }, "%.1f", ""},
				{"Batch factor", func(s trends.Snapshot) float64 { return s.BatchFactor }, "%.2f", ""},
				{"Conv. ratio", func(s trends.Snapshot) float64 { return s.ConventionalRatio * 100 }, "%.0f%%", ""},
			},
			snapshots,
		),
		buildTrendSection(
			[]trendMetricDef{
				{"Risk score", func(s trends.Snapshot) float64 { return s.ReleaseRiskScore }, "%.0f", ""},
				{"Hotspot score", func(s trends.Snapshot) float64 { return s.HotspotScore }, "%.0f", ""},
				{"Tech debt ($)", func(s trends.Snapshot) float64 { return s.TechDebtUSD }, "%.0f", "tech_debt"},
				{"Change complexity proxy", func(s trends.Snapshot) float64 { return s.ChangeComplexityProxy }, "%.1f", ""},
				{"Cross-cutting risk", func(s trends.Snapshot) float64 { return s.CrossCuttingChangeRisk }, "%.0f%%", ""},
				{"Churn factor", func(s trends.Snapshot) float64 { return s.ChurnFactor }, "%.1fx", ""},
				{"Complexity/feat", func(s trends.Snapshot) float64 { return s.ComplexityPerFeat }, "%.0f", ""},
				{"File volatility", func(s trends.Snapshot) float64 { return s.FileVolatility }, "%.1f", ""},
				{"Hotspot density", func(s trends.Snapshot) float64 { return s.HotspotDensity }, "%.0f%%", ""},
			},
			snapshots,
		),
		buildTrendSection(
			[]trendMetricDef{
				{"Ownership conc", func(s trends.Snapshot) float64 { return s.OwnershipConc }, "%.0f%%", ""},
				{"Ownership entropy", func(s trends.Snapshot) float64 { return s.OwnershipEntropy }, "%.2f", ""},
				{"Fix/feat ratio", func(s trends.Snapshot) float64 { return s.FixToFeatureRatio }, "%.1f:1", ""},
				{"Test/source", func(s trends.Snapshot) float64 { return s.TestToSourceRatio }, "%.0f%%", ""},
				{"Refactoring", func(s trends.Snapshot) float64 { return s.RefactoringRatio }, "%.0f%%", ""},
				{"Revert rate", func(s trends.Snapshot) float64 { return s.RevertRate }, "%.1f%%", ""},
				{"Scope isolation", func(s trends.Snapshot) float64 { return s.ScopeIsolation }, "%.0f%%", ""},
				{"Cross-cutting", func(s trends.Snapshot) float64 { return s.CrossCuttingPct }, "%.0f%%", ""},
				{"API surface", func(s trends.Snapshot) float64 { return float64(s.APISurfaceChange) }, "%.0f", ""},
				{"Touched test file ratio", func(s trends.Snapshot) float64 { return s.TouchedTestFileRatio }, "%.0f%%", ""},
			},
			snapshots,
		),
		buildTrendSection(
			[]trendMetricDef{
				{"Review load", func(s trends.Snapshot) float64 { return s.ReviewLoad }, "%.1fx", ""},
				{"Delete ratio", func(s trends.Snapshot) float64 { return s.DeleteRatio * 100 }, "%.0f%%", ""},
				{"Msg quality", func(s trends.Snapshot) float64 { return s.CommitMsgQuality }, "%.0f/100", ""},
				{"Author overlap", func(s trends.Snapshot) float64 { return s.AuthorOverlap }, "%.0f%%", ""},
				{"Peak hour σ", func(s trends.Snapshot) float64 { return s.PeakHourSpread }, "%.1f", ""},
			},
			snapshots,
		),
		buildTrendSection(
			[]trendMetricDef{
				{"Dir breadth", func(s trends.Snapshot) float64 { return s.DirectoryBreadth }, "%.0f%%", ""},
				{"Config churn", func(s trends.Snapshot) float64 { return s.ConfigChurnRate }, "%.0f%%", ""},
				{"Code freshness", func(s trends.Snapshot) float64 { return s.CodeFreshness }, "%.0fd", ""},
				{"Rhythm consist", func(s trends.Snapshot) float64 { return s.RhythmConsistency * 100 }, "%.0f%%", ""},
				{"Lang mix (cyr)", func(s trends.Snapshot) float64 { return s.LanguageMix * 100 }, "%.0f%%", ""},
			},
			snapshots,
		),
	}
	sectionTitles := []string{"Overview", "Velocity", "Code Metrics", "Quality", "Commit Anatomy", "Code Health & Rhythm"}

	buf.WriteString(fmt.Sprintf(`<p style="color: #666;">Showing %d releases. Arrows: `, n))
	buf.WriteString(`<span style="color: ` + colorRed + `;">▲ increasing</span>  `)
	buf.WriteString(`<span style="color: ` + colorGreen + `;">▼ decreasing</span>`)
	if hasThresholds(th) {
		buf.WriteString(`  |  Thresholds: `)
		buf.WriteString(`<span style="color: ` + colorGreen + `;">● good</span>  `)
		buf.WriteString(`<span style="color: ` + colorYellow + `;">● warning</span>  `)
		buf.WriteString(`<span style="color: ` + colorRed + `;">● critical</span>`)
	}
	buf.WriteString(`</p>`)
	buf.WriteString(Spacer())

	buf.WriteString(`<table style="width: 100%; border-collapse: collapse;"><tbody>`)

	headerCSS := "padding: 8px 10px; text-align: center; border-bottom: 2px solid #ddd; font-size: 12px;"
	buf.WriteString(`<tr style="background: #f4f5f7; color: #666;">`)
	fmt.Fprintf(buf, `<th style="padding: 8px 10px; text-align: left; width: 16%%; border-bottom: 2px solid #ddd; font-size: 12px;">Metric</th>`)
	for _, v := range versions {
		fmt.Fprintf(buf, `<th style="%s font-weight: bold;">%s</th>`, headerCSS, safehtml.Text(v))
	}
	buf.WriteString("</tr>")

	buf.WriteString(`<tr style="background: #fafbfc; color: #999; font-size: 11px;">`)
	fmt.Fprintf(buf, `<td style="padding: 4px 10px; border-bottom: 1px solid #eee;">Date</td>`)
	for _, d := range dates {
		fmt.Fprintf(buf, `<td style="padding: 4px 10px; text-align: center; border-bottom: 1px solid #eee;">%s</td>`, safehtml.Text(d))
	}
	buf.WriteString("</tr>")

	for si, sec := range sections {
		buf.WriteString(`<tr style="background: #fafbfc;">`)
		fmt.Fprintf(buf, `<td style="padding: 6px 10px; border-bottom: 1px solid #ddd; color: #999; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px;">%s</td>`, safehtml.Text(sectionTitles[si]))
		for i := 0; i < n; i++ {
			fmt.Fprintf(buf, `<td style="padding: 6px 10px; border-bottom: 1px solid #ddd;"></td>`)
		}
		buf.WriteString("</tr>")
		for _, r := range sec {
			renderGrandMetricRow(buf, r, th, n)
		}
	}

	buf.WriteString("</tbody></table>")
	buf.WriteString(Spacer())

	renderTrendsChart(buf, snapshots)
}

type trendMetricDef struct {
	label     string
	get       func(s trends.Snapshot) float64
	format    string
	metricKey string
}

func buildTrendSection(defs []trendMetricDef, snapshots []trends.Snapshot) []trendMetricRow {
	rows := make([]trendMetricRow, len(defs))
	for i, d := range defs {
		values := make([]float64, len(snapshots))
		displays := make([]string, len(snapshots))
		for j, s := range snapshots {
			values[j] = d.get(s)
			displays[j] = fmt.Sprintf(d.format, values[j])
		}
		rows[i] = trendMetricRow{
			label:     d.label,
			values:    values,
			displays:  displays,
			metricKey: d.metricKey,
		}
	}
	return rows
}

func renderGrandMetricRow(buf *bytes.Buffer, r trendMetricRow, th TrendsThresholds, n int) {
	buf.WriteString(`<tr style="border-bottom: 1px solid #eee;">`)
	fmt.Fprintf(buf, `<td style="padding: 8px 10px; vertical-align: middle;"><strong>%s</strong></td>`, safehtml.Text(r.label))

	for i, v := range r.displays {
		cellColor := colorDarkText
		display := v

		thColor := thresholdCellColor(r.metricKey, r.values[i], th)

		if i > 0 {
			delta := trends.ComputeDelta(r.values[i-1], r.values[i])
			if delta.Change != 0 {
				arrow := trends.TrendArrow(delta)
				display = fmt.Sprintf("%s %s", arrow, v)
			}
			if thColor != "" {
				cellColor = thColor
			} else if delta.Change > 0 {
				cellColor = colorRed
			} else if delta.Change < 0 {
				cellColor = colorGreen
			}
		} else if thColor != "" {
			cellColor = thColor
		}

		fontWeight := ""
		if i == n-1 {
			fontWeight = " font-weight: bold;"
		}
		fmt.Fprintf(buf, `<td style="padding: 8px 10px; text-align: center; color: %s;%s">%s</td>`, cellColor, fontWeight, safehtml.Text(display))
	}
	buf.WriteString("</tr>")
}

func thresholdCellColor(metricKey string, value float64, th TrendsThresholds) string {
	switch metricKey {
	case "release_commit_span":
		if th.ReleaseCommitSpanCritical > 0 && value >= th.ReleaseCommitSpanCritical {
			return colorRed
		}
		if th.ReleaseCommitSpanWarning > 0 && value >= th.ReleaseCommitSpanWarning {
			return colorYellow
		}
		if th.ReleaseCommitSpanWarning > 0 || th.ReleaseCommitSpanCritical > 0 {
			return colorGreen
		}
	case "tech_debt":
		if th.TechDebtCritical > 0 && value >= th.TechDebtCritical {
			return colorRed
		}
		if th.TechDebtWarning > 0 && value >= th.TechDebtWarning {
			return colorYellow
		}
		if th.TechDebtWarning > 0 || th.TechDebtCritical > 0 {
			return colorGreen
		}
	case "release_contribution_concentration":
		if th.ReleaseContributionConcentrationMin > 0 {
			if value < float64(th.ReleaseContributionConcentrationMin) {
				return colorRed
			}
			if value == float64(th.ReleaseContributionConcentrationMin) {
				return colorYellow
			}
			return colorGreen
		}
	}
	return ""
}

func hasThresholds(th TrendsThresholds) bool {
	return th.ReleaseCommitSpanWarning > 0 || th.ReleaseCommitSpanCritical > 0 ||
		th.TechDebtWarning > 0 || th.TechDebtCritical > 0 || th.ReleaseContributionConcentrationMin > 0
}

func renderTrendsChart(buf *bytes.Buffer, snapshots []trends.Snapshot) {
	if len(snapshots) < 2 {
		return
	}

	buf.WriteString("<h2>Trend Charts</h2>")

	buf.WriteString(`<ac:structured-macro ac:name="chart">`)
	buf.WriteString(`<ac:parameter ac:name="type">line</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="title">Release Metrics Overview</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="legend">true</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="width">700</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="height">350</ac:parameter>`)
	buf.WriteString(`<ac:rich-text-body>`)
	buf.WriteString(`<table><tbody>`)

	buf.WriteString(`<tr><th>Version</th><th>Commits</th><th>Release Contribution Concentration</th><th>Release Commit Span (h)</th><th>Risk Score</th><th>Hotspot Score</th><th>Tech Debt ($)</th><th>Change Complexity Proxy</th></tr>`)

	for _, s := range snapshots {
		fmt.Fprintf(buf, `<tr><td>%s</td><td>%d</td><td>%d</td><td>%.0f</td><td>%.0f</td><td>%.0f</td><td>%.0f</td><td>%.1f</td></tr>`,
			safehtml.Text(s.Version), s.TotalCommits, s.ReleaseContributionConcentration, s.ReleaseCommitSpanHours,
			s.ReleaseRiskScore, s.HotspotScore, s.TechDebtUSD, s.ChangeComplexityProxy)
	}

	buf.WriteString(`</tbody></table>`)
	buf.WriteString(`</ac:rich-text-body>`)
	buf.WriteString(`</ac:structured-macro>`)

	buf.WriteString(Spacer())

	buf.WriteString(`<ac:structured-macro ac:name="chart">`)
	buf.WriteString(`<ac:parameter ac:name="type">line</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="title">Quality Metrics</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="legend">true</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="width">700</ac:parameter>`)
	buf.WriteString(`<ac:parameter ac:name="height">300</ac:parameter>`)
	buf.WriteString(`<ac:rich-text-body>`)
	buf.WriteString(`<table><tbody>`)

	buf.WriteString(`<tr><th>Version</th><th>Ownership Conc (%)</th><th>Fix/Feat Ratio</th><th>Touched Test File Ratio (%)</th><th>Refactoring (%)</th><th>Scope Isolation (%)</th></tr>`)

	for _, s := range snapshots {
		fmt.Fprintf(buf, `<tr><td>%s</td><td>%.0f</td><td>%.1f</td><td>%.0f</td><td>%.0f</td><td>%.0f</td></tr>`,
			safehtml.Text(s.Version), s.OwnershipConc, s.FixToFeatureRatio,
			s.TouchedTestFileRatio, s.RefactoringRatio, s.ScopeIsolation)
	}

	buf.WriteString(`</tbody></table>`)
	buf.WriteString(`</ac:rich-text-body>`)
	buf.WriteString(`</ac:structured-macro>`)

	buf.WriteString(Spacer())
}

func renderTopContributorsAcrossReleases(buf *bytes.Buffer, snapshots []trends.Snapshot) {
	contributorCommits := make(map[string]int)
	for _, s := range snapshots {
		for _, c := range s.TopContributors {
			contributorCommits[c.Name] += c.Commits
		}
	}
	if len(contributorCommits) == 0 {
		return
	}

	type kv struct {
		Name    string
		Commits int
	}
	var sorted []kv
	for name, commits := range contributorCommits {
		sorted = append(sorted, kv{name, commits})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Commits > sorted[i].Commits {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}

	buf.WriteString("<h2>Top Contributors (All Releases)</h2>")
	buf.WriteString(`<table><tbody>`)
	buf.WriteString("<tr><th>#</th><th>Author</th><th>Commits</th></tr>")

	totalCommits := 0
	for _, c := range sorted {
		totalCommits += c.Commits
	}

	for i := 0; i < limit; i++ {
		c := sorted[i]
		medal := ""
		switch i {
		case 0:
			medal = "🥇 "
		case 1:
			medal = "🥈 "
		case 2:
			medal = "🥉 "
		}
		pct := 0.0
		if totalCommits > 0 {
			pct = float64(c.Commits) / float64(totalCommits) * 100
		}
		fmt.Fprintf(buf, "<tr><td>%d</td><td>%s%s</td><td>%d (%.0f%%)</td></tr>",
			i+1, medal, safehtml.Text(c.Name), c.Commits, pct)
	}
	buf.WriteString("</tbody></table>")
}

func RenderTrendsPage(snapshots []trends.Snapshot, th TrendsThresholds) string {
	return RenderTrendsPageWithGamification(snapshots, th, "")
}

func RenderTrendsPageWithGamification(snapshots []trends.Snapshot, th TrendsThresholds, gamificationHTML string) string {
	var buf bytes.Buffer
	buf.WriteString("<ac:structured-macro ac:name=\"toc\">")
	buf.WriteString("<ac:parameter ac:name=\"maxLevel\">2</ac:parameter>")
	buf.WriteString("<ac:parameter ac:name=\"minLevel\">2</ac:parameter>")
	buf.WriteString("</ac:structured-macro>")
	buf.WriteString(Spacer())

	panel := RenderTrendsPanelWithGamification(snapshots, th, gamificationHTML)
	buf.WriteString(panel)
	return buf.String()
}
