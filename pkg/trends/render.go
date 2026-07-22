package trends

import (
	"fmt"
	"strings"
)

type RenderOptions struct {
	Count int
}

func RenderTerminal(snapshots []Snapshot) string {
	if len(snapshots) == 0 {
		return "No trend data found. Run patchlog with a version (e.g. --bump auto) to start collecting snapshots.\n"
	}

	if len(snapshots) == 1 {
		return renderSingle(snapshots[0])
	}

	return renderMulti(snapshots)
}

func renderSingle(snap Snapshot) string {
	var sb strings.Builder
	sb.WriteString("┌─ Release Snapshot ───────────────────────────────────┐\n")
	sb.WriteString("│                                                      │\n")
	fmt.Fprintf(&sb, "│  Version:          %s\n", snap.Version)
	fmt.Fprintf(&sb, "│  Date:             %s\n", snap.Date)
	sb.WriteString("│                                                      │\n")
	sb.WriteString("│  ── Overview ──\n")
	fmt.Fprintf(&sb, "│  Commits:          %d\n", snap.TotalCommits)
	fmt.Fprintf(&sb, "│  Authors:          %d\n", snap.TotalAuthors)
	fmt.Fprintf(&sb, "│  Release contribution concentration:       %d\n", snap.ReleaseContributionConcentration)
	fmt.Fprintf(&sb, "│  Breaking changes: %d\n", snap.BreakingChanges)
	fmt.Fprintf(&sb, "│  Files touched:    %d\n", snap.FilesTouched)
	fmt.Fprintf(&sb, "│  Lines:            +%d / -%d (net %+d)\n", snap.LinesAdded, snap.LinesDeleted, snap.NetLines)
	if snap.JiraTickets > 0 {
		fmt.Fprintf(&sb, "│  Jira tickets:     %d\n", snap.JiraTickets)
	}
	sb.WriteString("│                                                      │\n")
	sb.WriteString("│  ── Velocity ──\n")
	fmt.Fprintf(&sb, "│  Release commit span:       %.0fh\n", snap.ReleaseCommitSpanHours)
	fmt.Fprintf(&sb, "│  Release age:        %.0fh\n", snap.ReleaseAgeHours)
	fmt.Fprintf(&sb, "│  Commits/day:      %.1f\n", snap.CommitsPerDay)
	fmt.Fprintf(&sb, "│  Batch factor:     %.2f\n", snap.BatchFactor)
	fmt.Fprintf(&sb, "│  Conv. ratio:      %.0f%%\n", snap.ConventionalRatio*100)
	sb.WriteString("│                                                      │\n")
	sb.WriteString("│  ── Code Metrics ──\n")
	fmt.Fprintf(&sb, "│  Risk score:       %.0f/100\n", snap.ReleaseRiskScore)
	fmt.Fprintf(&sb, "│  Hotspot score:    %.0f/100\n", snap.HotspotScore)
	fmt.Fprintf(&sb, "│  Tech debt:        $%.0f\n", snap.TechDebtUSD)
	fmt.Fprintf(&sb, "│  Change complexity: %.1f\n", snap.ChangeComplexityProxy)
	fmt.Fprintf(&sb, "│  Cross-cutting risk: %.0f%%\n", snap.CrossCuttingChangeRisk)
	fmt.Fprintf(&sb, "│  Churn factor:     %.1fx\n", snap.ChurnFactor)
	fmt.Fprintf(&sb, "│  Complexity/feat:  %.0f\n", snap.ComplexityPerFeat)
	fmt.Fprintf(&sb, "│  File volatility:  %.1f\n", snap.FileVolatility)
	fmt.Fprintf(&sb, "│  Hotspot density:  %.0f%%\n", snap.HotspotDensity)
	sb.WriteString("│                                                      │\n")
	sb.WriteString("│  ── Quality ──\n")
	fmt.Fprintf(&sb, "│  Ownership conc:   %.0f%%\n", snap.OwnershipConc)
	fmt.Fprintf(&sb, "│  Ownership entropy:%.2f\n", snap.OwnershipEntropy)
	fmt.Fprintf(&sb, "│  Fix/feat ratio:   %.1f:1\n", snap.FixToFeatureRatio)
	fmt.Fprintf(&sb, "│  Test/source:      %.0f%%\n", snap.TestToSourceRatio)
	fmt.Fprintf(&sb, "│  Refactoring:      %.0f%%\n", snap.RefactoringRatio)
	fmt.Fprintf(&sb, "│  Revert rate:      %.1f%%\n", snap.RevertRate)
	fmt.Fprintf(&sb, "│  Scope isolation:  %.0f%%\n", snap.ScopeIsolation)
	fmt.Fprintf(&sb, "│  Cross-cutting:    %.0f%%\n", snap.CrossCuttingPct)
	fmt.Fprintf(&sb, "│  API surface:      %d files\n", snap.APISurfaceChange)
	fmt.Fprintf(&sb, "│  Touched test file ratio:    %.0f%%\n", snap.TouchedTestFileRatio)
	sb.WriteString("│                                                      │\n")
	sb.WriteString("│  Need at least 2 releases to show trends.            │\n")
	sb.WriteString("└──────────────────────────────────────────────────────┘\n")
	return sb.String()
}

type metricRow struct {
	label  string
	values []string
	raw    []float64
}

func renderMulti(snapshots []Snapshot) string {
	var sb strings.Builder
	n := len(snapshots)

	versions := make([]string, n)
	dates := make([]string, n)
	for i, s := range snapshots {
		versions[i] = s.Version
		dates[i] = formatDate(s.Date)
	}

	sections := [][]metricRow{
		{
			{"Commits", intStrs(snapshots, func(s Snapshot) int { return s.TotalCommits }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.TotalCommits) })},
			{"Authors", intStrs(snapshots, func(s Snapshot) int { return s.TotalAuthors }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.TotalAuthors) })},
			{"Release contribution concentration", intStrs(snapshots, func(s Snapshot) int { return s.ReleaseContributionConcentration }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.ReleaseContributionConcentration) })},
			{"Breaking", intStrs(snapshots, func(s Snapshot) int { return s.BreakingChanges }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.BreakingChanges) })},
			{"Files touched", intStrs(snapshots, func(s Snapshot) int { return s.FilesTouched }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.FilesTouched) })},
			{"Lines +", intStrs(snapshots, func(s Snapshot) int { return s.LinesAdded }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.LinesAdded) })},
			{"Lines -", intStrs(snapshots, func(s Snapshot) int { return s.LinesDeleted }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.LinesDeleted) })},
			{"Net lines", intStrs(snapshots, func(s Snapshot) int { return s.NetLines }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.NetLines) })},
			{"Jira tickets", intStrs(snapshots, func(s Snapshot) int { return s.JiraTickets }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.JiraTickets) })},
		},
		{
			{"Release commit span (h)", floatStrs(snapshots, func(s Snapshot) float64 { return s.ReleaseCommitSpanHours }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.ReleaseCommitSpanHours })},
			{"Release age (h)", floatStrs(snapshots, func(s Snapshot) float64 { return s.ReleaseAgeHours }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.ReleaseAgeHours })},
			{"Commits/day", floatStrs(snapshots, func(s Snapshot) float64 { return s.CommitsPerDay }, "%.1f"), toFloats(snapshots, func(s Snapshot) float64 { return s.CommitsPerDay })},
			{"Batch factor", floatStrs(snapshots, func(s Snapshot) float64 { return s.BatchFactor }, "%.2f"), toFloats(snapshots, func(s Snapshot) float64 { return s.BatchFactor })},
			{"Conv. ratio", floatStrs(snapshots, func(s Snapshot) float64 { return s.ConventionalRatio * 100 }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.ConventionalRatio * 100 })},
		},
		{
			{"Risk score", floatStrs(snapshots, func(s Snapshot) float64 { return s.ReleaseRiskScore }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.ReleaseRiskScore })},
			{"Hotspot score", floatStrs(snapshots, func(s Snapshot) float64 { return s.HotspotScore }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.HotspotScore })},
			{"Tech debt ($)", floatStrs(snapshots, func(s Snapshot) float64 { return s.TechDebtUSD }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.TechDebtUSD })},
			{"Change complexity", floatStrs(snapshots, func(s Snapshot) float64 { return s.ChangeComplexityProxy }, "%.1f"), toFloats(snapshots, func(s Snapshot) float64 { return s.ChangeComplexityProxy })},
			{"Cross-cutting risk", floatStrs(snapshots, func(s Snapshot) float64 { return s.CrossCuttingChangeRisk }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.CrossCuttingChangeRisk })},
			{"Churn factor", floatStrs(snapshots, func(s Snapshot) float64 { return s.ChurnFactor }, "%.1fx"), toFloats(snapshots, func(s Snapshot) float64 { return s.ChurnFactor })},
			{"Complexity/feat", floatStrs(snapshots, func(s Snapshot) float64 { return s.ComplexityPerFeat }, "%.0f"), toFloats(snapshots, func(s Snapshot) float64 { return s.ComplexityPerFeat })},
			{"File volatility", floatStrs(snapshots, func(s Snapshot) float64 { return s.FileVolatility }, "%.1f"), toFloats(snapshots, func(s Snapshot) float64 { return s.FileVolatility })},
			{"Hotspot density", floatStrs(snapshots, func(s Snapshot) float64 { return s.HotspotDensity }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.HotspotDensity })},
		},
		{
			{"Ownership conc", floatStrs(snapshots, func(s Snapshot) float64 { return s.OwnershipConc }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.OwnershipConc })},
			{"Ownership entrop", floatStrs(snapshots, func(s Snapshot) float64 { return s.OwnershipEntropy }, "%.2f"), toFloats(snapshots, func(s Snapshot) float64 { return s.OwnershipEntropy })},
			{"Fix/feat ratio", floatStrs(snapshots, func(s Snapshot) float64 { return s.FixToFeatureRatio }, "%.1f:1"), toFloats(snapshots, func(s Snapshot) float64 { return s.FixToFeatureRatio })},
			{"Test/source", floatStrs(snapshots, func(s Snapshot) float64 { return s.TestToSourceRatio }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.TestToSourceRatio })},
			{"Refactoring", floatStrs(snapshots, func(s Snapshot) float64 { return s.RefactoringRatio }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.RefactoringRatio })},
			{"Revert rate", floatStrs(snapshots, func(s Snapshot) float64 { return s.RevertRate }, "%.1f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.RevertRate })},
			{"Scope isolation", floatStrs(snapshots, func(s Snapshot) float64 { return s.ScopeIsolation }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.ScopeIsolation })},
			{"Cross-cutting", floatStrs(snapshots, func(s Snapshot) float64 { return s.CrossCuttingPct }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.CrossCuttingPct })},
			{"API surface", intStrs(snapshots, func(s Snapshot) int { return s.APISurfaceChange }), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.APISurfaceChange) })},
			{"Touched test file ratio", floatStrs(snapshots, func(s Snapshot) float64 { return s.TouchedTestFileRatio }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.TouchedTestFileRatio })},
		},
		{
			{"Commit S/M/L/H", func() []string {
				out := make([]string, len(snapshots))
				for i, s := range snapshots {
					out[i] = fmt.Sprintf("%d/%d/%d/%d", s.CommitSizeSmall, s.CommitSizeMedium, s.CommitSizeLarge, s.CommitSizeHuge)
				}
				return out
			}(), toFloats(snapshots, func(s Snapshot) float64 { return float64(s.CommitSizeLarge + s.CommitSizeHuge) })},
			{"Review load", floatStrs(snapshots, func(s Snapshot) float64 { return s.ReviewLoad }, "%.1fx"), toFloats(snapshots, func(s Snapshot) float64 { return s.ReviewLoad })},
			{"Delete ratio", floatStrs(snapshots, func(s Snapshot) float64 { return s.DeleteRatio * 100 }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.DeleteRatio * 100 })},
			{"Msg quality", floatStrs(snapshots, func(s Snapshot) float64 { return s.CommitMsgQuality }, "%.0f/100"), toFloats(snapshots, func(s Snapshot) float64 { return s.CommitMsgQuality })},
			{"Author overlap", floatStrs(snapshots, func(s Snapshot) float64 { return s.AuthorOverlap }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.AuthorOverlap })},
			{"Peak hour σ", floatStrs(snapshots, func(s Snapshot) float64 { return s.PeakHourSpread }, "%.1f"), toFloats(snapshots, func(s Snapshot) float64 { return s.PeakHourSpread })},
		},
		{
			{"Dir breadth", floatStrs(snapshots, func(s Snapshot) float64 { return s.DirectoryBreadth }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.DirectoryBreadth })},
			{"Config churn", floatStrs(snapshots, func(s Snapshot) float64 { return s.ConfigChurnRate }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.ConfigChurnRate })},
			{"Code freshness", floatStrs(snapshots, func(s Snapshot) float64 { return s.CodeFreshness }, "%.0fd"), toFloats(snapshots, func(s Snapshot) float64 { return s.CodeFreshness })},
			{"Rhythm consist", floatStrs(snapshots, func(s Snapshot) float64 { return s.RhythmConsistency * 100 }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.RhythmConsistency * 100 })},
			{"Lang mix (cyr)", floatStrs(snapshots, func(s Snapshot) float64 { return s.LanguageMix * 100 }, "%.0f%%"), toFloats(snapshots, func(s Snapshot) float64 { return s.LanguageMix * 100 })},
		},
	}
	sectionTitles := []string{"Overview", "Velocity", "Code Metrics", "Quality", "Commit Anatomy", "Code Health & Rhythm"}

	labelW := 16
	for _, sec := range sections {
		for _, r := range sec {
			if len(r.label) > labelW {
				labelW = len(r.label)
			}
		}
	}

	colW := make([]int, n)
	for i := 0; i < n; i++ {
		colW[i] = len(versions[i])
		if len(dates[i]) > colW[i] {
			colW[i] = len(dates[i])
		}
		for _, sec := range sections {
			for _, r := range sec {
				if len(r.values[i]) > colW[i] {
					colW[i] = len(r.values[i])
				}
			}
		}
	}

	innerW := labelW + 2
	for _, w := range colW {
		innerW += w + 2
	}

	border := strings.Repeat("─", innerW+2)

	sb.WriteString("┌─ Cross-Release Trends")
	titleTail := fmt.Sprintf("(%d releases) ─┐", n)
	pad := innerW + 2 - 22 - len(titleTail)
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(titleTail + "\n")
	sb.WriteString("│" + strings.Repeat(" ", innerW+2) + "│\n")

	printRow(&sb, "Metric", versions, labelW, colW, innerW, true, false)
	printRow(&sb, "Date", dates, labelW, colW, innerW, false, true)

	sb.WriteString("│" + strings.Repeat("─", innerW+2) + "│\n")

	for si, sec := range sections {
		fmt.Fprintf(&sb, "│  %-*s  ", labelW, "── "+sectionTitles[si]+" ──")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, "%*s  ", colW[i], "")
		}
		sb.WriteString("│\n")
		for _, r := range sec {
			printTrendRow(&sb, r, labelW, colW, innerW)
		}
	}

	sb.WriteString("│" + strings.Repeat(" ", innerW+2) + "│\n")
	footer := "  Arrows: ▲ increasing   ▼ decreasing"
	footerPad := innerW + 2 - len(footer)
	if footerPad < 0 {
		footerPad = 0
	}
	sb.WriteString("│" + footer + strings.Repeat(" ", footerPad) + "│\n")
	sb.WriteString("└" + border + "┘\n")

	return sb.String()
}

func printRow(sb *strings.Builder, label string, values []string, labelW int, colW []int, innerW int, isHeader bool, isMuted bool) {
	fmt.Fprintf(sb, "│  %-*s  ", labelW, label)
	for i, v := range values {
		if isHeader {
			fmt.Fprintf(sb, "%-*s  ", colW[i], v)
		} else {
			fmt.Fprintf(sb, "%*s  ", colW[i], v)
		}
	}
	sb.WriteString("│\n")
}

func printTrendRow(sb *strings.Builder, r metricRow, labelW int, colW []int, innerW int) {
	fmt.Fprintf(sb, "│  %-*s  ", labelW, r.label)
	for i, v := range r.values {
		display := v
		if i > 0 && len(r.raw) > i {
			delta := ComputeDelta(r.raw[i-1], r.raw[i])
			if delta.Change != 0 {
				arrow := TrendArrow(delta)
				display = fmt.Sprintf("%s %s", arrow, v)
			}
		}
		fmt.Fprintf(sb, "%*s  ", colW[i], display)
	}
	sb.WriteString("│\n")
}

func intStrs(snaps []Snapshot, get func(Snapshot) int) []string {
	out := make([]string, len(snaps))
	for i, s := range snaps {
		out[i] = fmt.Sprintf("%d", get(s))
	}
	return out
}

func floatStrs(snaps []Snapshot, get func(Snapshot) float64, format string) []string {
	out := make([]string, len(snaps))
	for i, s := range snaps {
		out[i] = fmt.Sprintf(format, get(s))
	}
	return out
}

func toFloats(snaps []Snapshot, get func(Snapshot) float64) []float64 {
	out := make([]float64, len(snaps))
	for i, s := range snaps {
		out[i] = get(s)
	}
	return out
}

func formatDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func RenderJSON(snapshots []Snapshot) (string, error) {
	var sb strings.Builder
	sb.WriteString("[")
	for i, s := range snapshots {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"version":"%s","date":"%s","commits":%d,"authors":%d,"release_contribution_concentration":%d,"breaking":%d,"release_commit_span_h":%.0f,"tech_debt_usd":%.0f,"risk_score":%.0f,"conv_ratio":%.2f,"hotspot_score":%.0f,"hotspot_density":%.0f,"churn_factor":%.1f,"change_complexity_proxy":%.1f,"cross_cutting_change_risk":%.0f,"ownership_conc":%.0f,"ownership_entropy":%.2f,"fix_to_feature_ratio":%.1f,"test_to_source_ratio":%.0f,"refactoring_ratio":%.0f,"revert_rate":%.1f,"scope_isolation":%.0f,"cross_cutting_pct":%.0f,"file_volatility":%.1f,"complexity_per_feat":%.0f,"api_surface_change":%d,"batch_factor":%.2f,"touched_test_file_ratio":%.0f,"release_age_h":%.0f,"commits_per_day":%.1f,"net_lines":%d,"lines_added":%d,"lines_deleted":%d,"files_touched":%d,"jira_tickets":%d,"commit_size_small":%d,"commit_size_medium":%d,"commit_size_large":%d,"commit_size_huge":%d,"review_load":%.1f,"delete_ratio":%.2f,"commit_msg_quality":%.0f,"author_overlap":%.0f,"peak_hour_spread":%.1f,"directory_breadth":%.0f,"config_churn_rate":%.0f,"code_freshness":%.0f,"rhythm_consistency":%.2f,"language_mix":%.2f}`,
			s.Version, s.Date, s.TotalCommits, s.TotalAuthors, s.ReleaseContributionConcentration, s.BreakingChanges,
			s.ReleaseCommitSpanHours, s.TechDebtUSD, s.ReleaseRiskScore, s.ConventionalRatio,
			s.HotspotScore, s.HotspotDensity, s.ChurnFactor, s.ChangeComplexityProxy, s.CrossCuttingChangeRisk,
			s.OwnershipConc, s.OwnershipEntropy, s.FixToFeatureRatio, s.TestToSourceRatio,
			s.RefactoringRatio, s.RevertRate, s.ScopeIsolation, s.CrossCuttingPct,
			s.FileVolatility, s.ComplexityPerFeat, s.APISurfaceChange, s.BatchFactor,
			s.TouchedTestFileRatio, s.ReleaseAgeHours, s.CommitsPerDay,
			s.NetLines, s.LinesAdded, s.LinesDeleted, s.FilesTouched, s.JiraTickets,
			s.CommitSizeSmall, s.CommitSizeMedium, s.CommitSizeLarge, s.CommitSizeHuge,
			s.ReviewLoad, s.DeleteRatio, s.CommitMsgQuality, s.AuthorOverlap,
			s.PeakHourSpread, s.DirectoryBreadth, s.ConfigChurnRate, s.CodeFreshness,
			s.RhythmConsistency, s.LanguageMix))
	}
	sb.WriteString("]")
	return sb.String(), nil
}
