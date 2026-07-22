// Package timeline renders a release history table with key metrics for cross-release comparison.
package timeline

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/trends"
)

func FormatHTML(snapshots []trends.Snapshot) string {
	if len(snapshots) == 0 {
		return ""
	}
	var sb strings.Builder

	sb.WriteString(`<div class="table-wrap" style="overflow-x: auto;"><table><thead><tr>`)
	sb.WriteString(`<th>Release</th><th>Date</th><th>Commits</th><th>Authors</th><th>Release Contribution Concentration</th><th>Risk</th><th>Tech Debt</th><th>Net Lines</th><th>Top Contributor</th>`)
	sb.WriteString(`</tr></thead><tbody>`)

	for _, s := range snapshots {
		topContrib := ""
		topCommits := 0
		for _, c := range s.TopContributors {
			if c.Commits > topCommits {
				topCommits = c.Commits
				topContrib = c.Name
			}
		}
		if topContrib == "" && len(s.TopContributors) > 0 {
			topContrib = s.TopContributors[0].Name
		}

		riskCls := ""
		if s.ReleaseRiskScore >= 40 {
			riskCls = ` style="color: var(--red);"`
		} else if s.ReleaseRiskScore >= 25 {
			riskCls = ` style="color: var(--accent);"`
		} else {
			riskCls = ` style="color: var(--green);"`
		}

		sb.WriteString(`<tr>`)
		fmt.Fprintf(&sb, `<td style="font-weight: 600; color: var(--accent);">%s</td>`, safehtml.Text(s.Version))
		fmt.Fprintf(&sb, `<td style="font-size: 11px;">%s</td>`, safehtml.Text(s.Date))
		fmt.Fprintf(&sb, `<td>%d</td>`, s.TotalCommits)
		fmt.Fprintf(&sb, `<td>%d</td>`, s.TotalAuthors)
		fmt.Fprintf(&sb, `<td>%d</td>`, s.ReleaseContributionConcentration)
		fmt.Fprintf(&sb, `<td%s>%.0f</td>`, riskCls, s.ReleaseRiskScore)
		fmt.Fprintf(&sb, `<td>$%.0f</td>`, s.TechDebtUSD)
		fmt.Fprintf(&sb, `<td>%+d</td>`, s.NetLines)
		shortName := topContrib
		if len(shortName) > 18 {
			shortName = shortName[:16] + "…"
		}
		fmt.Fprintf(&sb, `<td style="font-size: 11px;">%s (%d)</td>`, safehtml.Text(shortName), topCommits)
		sb.WriteString(`</tr>`)
	}

	sb.WriteString(`</tbody></table></div>`)

	return sb.String()
}
