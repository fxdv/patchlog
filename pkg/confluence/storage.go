package confluence

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/safehtml"
	"github.com/fxdv/patchlog/pkg/theme"
)

func RenderStorageFormat(report render.Report) string {
	var buf bytes.Buffer
	useEmoji := report.Emojis

	fmt.Fprintf(&buf, "<h1>%s", safehtml.Text(report.Version))
	if report.Date != "" {
		fmt.Fprintf(&buf, " <span style=\"color: %s;\">(%s)</span>", colorGrey, safehtml.Text(report.Date))
	} else {
		fmt.Fprintf(&buf, " <span style=\"color: %s;\">(%s)</span>", colorGrey, time.Now().Format("2006-01-02"))
	}
	buf.WriteString("</h1>")

	if report.CompareURL != "" {
		fmt.Fprintf(&buf, "<p><a href=\"%s\">Full Changelog</a></p>", safehtml.Text(report.CompareURL))
	}

	buf.WriteString(Spacer())

	buf.WriteString("<ac:structured-macro ac:name=\"toc\">")
	buf.WriteString("<ac:parameter ac:name=\"maxLevel\">2</ac:parameter>")
	buf.WriteString("<ac:parameter ac:name=\"minLevel\">2</ac:parameter>")
	buf.WriteString("</ac:structured-macro>")
	buf.WriteString(Spacer())

	if len(report.Breaking) > 0 {
		buf.WriteString("<ac:structured-macro ac:name=\"info\">")
		buf.WriteString("<ac:parameter ac:name=\"title\">⚠️ Breaking Changes</ac:parameter>")
		buf.WriteString("<ac:rich-text-body>")
		renderItemTable(&buf, report.Breaking, report, true)
		buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
		buf.WriteString(Spacer())
	}

	firstSection := true
	for _, section := range report.Sections {
		if len(section.Items) == 0 && len(section.Scopes) == 0 {
			continue
		}
		if !firstSection {
			buf.WriteString(Spacer())
		}
		firstSection = false
		heading := section.Heading
		if useEmoji {
			heading = render.PrettyHeading(section.Heading, section.Type, true)
		}

		totalItems := len(section.Items)
		for _, sg := range section.Scopes {
			totalItems += len(sg.Items)
		}
		useExpand := totalItems > maxItemsBeforeExpand

		if useExpand {
			buf.WriteString("<ac:structured-macro ac:name=\"expand\">")
			fmt.Fprintf(&buf, "<ac:parameter ac:name=\"title\">%s (%d)</ac:parameter>", safehtml.Text(heading), totalItems)
			buf.WriteString("<ac:rich-text-body>")
		} else {
			fmt.Fprintf(&buf, "<h2>%s</h2>", safehtml.Text(heading))
		}

		renderSectionTable(&buf, section, report)

		if useExpand {
			buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
		}
	}

	if len(report.Dependencies) > 0 {
		buf.WriteString(Spacer())
		buf.WriteString(RenderDependencies(report.Dependencies))
	}

	return buf.String()
}

func RenderDependencies(deps []render.DependencyChange) string {
	if len(deps) == 0 {
		return ""
	}
	var buf bytes.Buffer

	buf.WriteString("<ac:structured-macro ac:name=\"expand\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">📦 Dependencies</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")

	buf.WriteString(`<table style="width: 100%; border-collapse: collapse;">`)
	buf.WriteString("<tbody>")
	buf.WriteString(`<tr style="background: #f4f5f7; font-size: 12px; color: #666;">`)
	buf.WriteString(`<th style="padding: 6px 10px; text-align: left; border-bottom: 2px solid #ddd;">Package</th>`)
	buf.WriteString(`<th style="padding: 6px 10px; text-align: left; border-bottom: 2px solid #ddd;">Change</th>`)
	buf.WriteString(`<th style="padding: 6px 10px; text-align: left; border-bottom: 2px solid #ddd;">Ecosystem</th>`)
	buf.WriteString(`<th style="padding: 6px 10px; text-align: left; border-bottom: 2px solid #ddd;">Upstream</th>`)
	buf.WriteString("</tr>")

	for _, dep := range deps {
		buf.WriteString(`<tr style="border-bottom: 1px solid #eee;">`)
		fmt.Fprintf(&buf, `<td style="padding: 6px 10px; vertical-align: top;"><strong>%s</strong></td>`, safehtml.Text(dep.Name))

		buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top; font-family: monospace; font-size: 12px;">`)
		if dep.OldVersion != "" {
			fmt.Fprintf(&buf, `<span style="color: %s;">%s</span> → <span style="color: %s;"><strong>%s</strong></span>`, colorMuted, safehtml.Text(dep.OldVersion), colorGreen, safehtml.Text(dep.NewVersion))
		} else {
			fmt.Fprintf(&buf, `<span style="color: %s;"><strong>%s</strong> (new)</span>`, colorGreen, safehtml.Text(dep.NewVersion))
		}
		buf.WriteString("</td>")

		fmt.Fprintf(&buf, `<td style="padding: 6px 10px; vertical-align: top; font-size: 12px;">%s</td>`, safehtml.Text(dep.Ecosystem))

		buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top; font-size: 12px;">`)
		if dep.ChangelogURL != "" {
			fmt.Fprintf(&buf, `<a href="%s">link</a>`, safehtml.Text(dep.ChangelogURL))
		}
		buf.WriteString("</td>")
		buf.WriteString("</tr>")
	}

	buf.WriteString("</tbody></table>")

	hasChangelog := false
	for _, dep := range deps {
		if dep.Changelog != "" {
			hasChangelog = true
			break
		}
	}

	if hasChangelog {
		buf.WriteString(Spacer())
		for _, dep := range deps {
			if dep.Changelog == "" {
				continue
			}
			buf.WriteString("<ac:structured-macro ac:name=\"expand\">")
			fmt.Fprintf(&buf, "<ac:parameter ac:name=\"title\">%s</ac:parameter>", safehtml.Text(dep.Name))
			buf.WriteString("<ac:rich-text-body>")
			fmt.Fprintf(&buf, `<p style="font-size: 13px; line-height: 1.6; color: %s;">%s</p>`, colorDarkText, safehtml.Text(dep.Changelog))
			buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
		}
	}

	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

func RenderThemedStorageFormat(tr theme.ThemedReport) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "<h1>%s", safehtml.Text(tr.Version))
	if tr.Date != "" {
		fmt.Fprintf(&buf, " <span style=\"color: %s;\">(%s)</span>", colorGrey, safehtml.Text(tr.Date))
	} else {
		fmt.Fprintf(&buf, " <span style=\"color: %s;\">(%s)</span>", colorGrey, time.Now().Format("2006-01-02"))
	}
	buf.WriteString("</h1>")

	if tr.CompareURL != "" {
		fmt.Fprintf(&buf, "<p><a href=\"%s\">Full Changelog</a></p>", safehtml.Text(tr.CompareURL))
	}

	buf.WriteString(Spacer())

	buf.WriteString("<ac:structured-macro ac:name=\"toc\">")
	buf.WriteString("<ac:parameter ac:name=\"maxLevel\">2</ac:parameter>")
	buf.WriteString("<ac:parameter ac:name=\"minLevel\">2</ac:parameter>")
	buf.WriteString("</ac:structured-macro>")
	buf.WriteString(Spacer())

	if len(tr.Breaking) > 0 {
		buf.WriteString("<ac:structured-macro ac:name=\"info\">")
		buf.WriteString("<ac:parameter ac:name=\"title\">⚠️ Breaking Changes</ac:parameter>")
		buf.WriteString("<ac:rich-text-body>")
		fakeReport := render.Report{
			ShowAuthor:        tr.ShowAuthor,
			CommitURLTemplate: tr.CommitURLTemplate,
			IssueURLTemplate:  tr.IssueURLTemplate,
			Repo:              tr.Repo,
		}
		renderItemTable(&buf, tr.Breaking, fakeReport, true)
		buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
		buf.WriteString(Spacer())
	}

	fakeReport := render.Report{
		ShowAuthor:        tr.ShowAuthor,
		CommitURLTemplate: tr.CommitURLTemplate,
		IssueURLTemplate:  tr.IssueURLTemplate,
		Repo:              tr.Repo,
	}

	firstTheme := true
	for _, tg := range tr.Themes {
		if len(tg.Items) == 0 {
			continue
		}
		if !firstTheme {
			buf.WriteString(Spacer())
		}
		firstTheme = false

		useExpand := len(tg.Items) > maxItemsBeforeExpand

		if useExpand {
			buf.WriteString("<ac:structured-macro ac:name=\"expand\">")
			fmt.Fprintf(&buf, "<ac:parameter ac:name=\"title\">%s (%d)</ac:parameter>", safehtml.Text(tg.Title), len(tg.Items))
			buf.WriteString("<ac:rich-text-body>")
		} else {
			fmt.Fprintf(&buf, "<h2>%s</h2>", safehtml.Text(tg.Title))
		}

		if tg.Narrative != "" {
			fmt.Fprintf(&buf, "<p><em>%s</em></p>", safehtml.Text(tg.Narrative))
		}

		renderItemTable(&buf, tg.Items, fakeReport, false)

		if useExpand {
			buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
		}
	}

	return buf.String()
}

func renderSectionTable(buf *bytes.Buffer, section render.Section, report render.Report) {
	var rows []itemRow
	for _, item := range section.Items {
		rows = append(rows, itemRow{item: item})
	}
	for _, sg := range section.Scopes {
		for _, item := range sg.Items {
			rows = append(rows, itemRow{scope: sg.Name, item: item})
		}
	}
	if len(rows) == 0 {
		return
	}
	renderItemTableRows(buf, rows, report, false)
}

func renderItemTable(buf *bytes.Buffer, items []render.Item, report render.Report, isBreaking bool) {
	rows := make([]itemRow, len(items))
	for i, item := range items {
		rows[i] = itemRow{item: item}
	}
	renderItemTableRows(buf, rows, report, isBreaking)
}

type itemRow struct {
	scope string
	item  render.Item
}

func renderItemTableRows(buf *bytes.Buffer, rows []itemRow, report render.Report, isBreaking bool) {
	hasScope := false
	hasTicket := false
	hasImpact := false
	hasCommit := false
	hasAuthor := false
	for _, r := range rows {
		if r.scope != "" {
			hasScope = true
		}
		if len(r.item.JiraIssues) > 0 || r.item.Ref != "" {
			hasTicket = true
		}
		if r.item.Significance != "" && r.item.Significance != "skip" {
			hasImpact = true
		}
		if r.item.Hash != "" {
			hasCommit = true
		}
		if r.item.Author != "" && report.ShowAuthor {
			hasAuthor = true
		}
	}

	buf.WriteString(`<table style="width: 100%; border-collapse: collapse;">`)
	buf.WriteString("<tbody>")

	buf.WriteString(`<tr style="background: #f4f5f7; font-size: 12px; color: #666;">`)
	if hasScope {
		buf.WriteString(`<th style="padding: 6px 10px; text-align: left; width: 10%; border-bottom: 2px solid #ddd;">Scope</th>`)
	}
	descHeader := "Change"
	if isBreaking {
		descHeader = "⚠️ Change"
	}
	fmt.Fprintf(buf, `<th style="padding: 6px 10px; text-align: left; border-bottom: 2px solid #ddd;">%s</th>`, descHeader)
	if hasTicket {
		buf.WriteString(`<th style="padding: 6px 10px; text-align: left; width: 15%; border-bottom: 2px solid #ddd;">Ticket</th>`)
	}
	if hasImpact {
		buf.WriteString(`<th style="padding: 6px 10px; text-align: center; width: 8%; border-bottom: 2px solid #ddd;">Impact</th>`)
	}
	if hasCommit {
		buf.WriteString(`<th style="padding: 6px 10px; text-align: left; width: 10%; border-bottom: 2px solid #ddd;">Commit</th>`)
	}
	if hasAuthor {
		buf.WriteString(`<th style="padding: 6px 10px; text-align: left; width: 12%; border-bottom: 2px solid #ddd;">Author</th>`)
	}
	buf.WriteString("</tr>")

	for _, r := range rows {
		buf.WriteString(`<tr style="border-bottom: 1px solid #eee;">`)
		if hasScope {
			buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top;">`)
			if r.scope != "" {
				fmt.Fprintf(buf, `<strong style="font-size: 11px; color: %s;">%s</strong>`, colorMuted, safehtml.Text(r.scope))
			}
			buf.WriteString("</td>")
		}
		buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top;">`)
		if isBreaking {
			fmt.Fprintf(buf, "<strong>%s</strong>", safehtml.Text(r.item.Description))
		} else {
			buf.WriteString(safehtml.Text(r.item.Description))
		}
		buf.WriteString("</td>")
		if hasTicket {
			buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top; font-size: 12px;">`)
			if r.item.Ref != "" {
				renderItemRef(buf, r.item, report)
			}
			renderJiraLinks(buf, r.item)
			buf.WriteString("</td>")
		}
		if hasImpact {
			buf.WriteString(`<td style="padding: 6px 10px; text-align: center; vertical-align: top;">`)
			renderSignificanceBadge(buf, r.item.Significance, report.Emojis)
			buf.WriteString("</td>")
		}
		if hasCommit {
			buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top;">`)
			renderItemHash(buf, r.item, report)
			buf.WriteString("</td>")
		}
		if hasAuthor {
			buf.WriteString(`<td style="padding: 6px 10px; vertical-align: top; font-size: 12px;">`)
			if r.item.Author != "" && report.ShowAuthor {
				fmt.Fprintf(buf, `<em>by @%s</em>`, safehtml.Text(r.item.Author))
			}
			buf.WriteString("</td>")
		}
		buf.WriteString("</tr>")
	}

	buf.WriteString("</tbody></table>")
}

func renderSignificanceBadge(buf *bytes.Buffer, significance string, useEmoji bool) {
	if significance == "" || significance == "skip" {
		return
	}
	colour := "Grey"
	switch significance {
	case "major":
		colour = "Red"
	case "minor":
		colour = "Yellow"
	case "patch":
		colour = "Green"
	}
	buf.WriteString(" <ac:structured-macro ac:name=\"status\">")
	fmt.Fprintf(buf, "<ac:parameter ac:name=\"colour\">%s</ac:parameter>", colour)
	fmt.Fprintf(buf, "<ac:parameter ac:name=\"title\">%s</ac:parameter>", safehtml.Text(significance))
	buf.WriteString("</ac:structured-macro>")
}

func RenderSection(report render.Report) string {
	body := RenderStorageFormat(report)
	return body
}

func renderJiraLinks(buf *bytes.Buffer, item render.Item) {
	for _, j := range item.JiraIssues {
		if j.URL != "" {
			fmt.Fprintf(buf, " <a href=\"%s\">%s</a>", safehtml.Text(j.URL), safehtml.Text(j.Key))
		} else {
			fmt.Fprintf(buf, " %s", safehtml.Text(j.Key))
		}
		if j.Summary != "" {
			fmt.Fprintf(buf, " %s", safehtml.Text(j.Summary))
		}
		if j.Status != "" {
			statusColor := "Grey"
			switch strings.ToLower(j.Status) {
			case "done", "closed", "resolved":
				statusColor = "Green"
			case "in progress", "in review":
				statusColor = "Yellow"
			case "to do", "open", "backlog":
				statusColor = "Grey"
			default:
				statusColor = "Blue"
			}
			fmt.Fprintf(buf, " <ac:structured-macro ac:name=\"status\"><ac:parameter ac:name=\"colour\">%s</ac:parameter><ac:parameter ac:name=\"title\">%s</ac:parameter></ac:structured-macro>", statusColor, safehtml.Text(j.Status))
		}
		if j.Priority != "" {
			priorityColor := "grey"
			switch strings.ToLower(j.Priority) {
			case "highest", "critical":
				priorityColor = "red"
			case "high":
				priorityColor = "orange"
			case "medium":
				priorityColor = "yellow"
			case "low", "lowest":
				priorityColor = "green"
			}
			fmt.Fprintf(buf, " <ac:structured-macro ac:name=\"status\"><ac:parameter ac:name=\"colour\">%s</ac:parameter><ac:parameter ac:name=\"title\">%s</ac:parameter></ac:structured-macro>", priorityColor, safehtml.Text(j.Priority))
		}
	}
}

func renderItemRef(buf *bytes.Buffer, item render.Item, report render.Report) {
	if item.Ref == "" {
		return
	}
	issueURL := report.IssueURL(item.Ref)
	if issueURL != "" {
		fmt.Fprintf(buf, " (<a href=\"%s\">%s</a>)", safehtml.Text(issueURL), safehtml.Text(item.Ref))
	} else {
		fmt.Fprintf(buf, " (%s)", safehtml.Text(item.Ref))
	}
}

func renderItemHash(buf *bytes.Buffer, item render.Item, report render.Report) {
	if item.Hash == "" {
		return
	}
	shortHash := item.Hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	commitURL := report.CommitURL(item.Hash)
	if commitURL != "" {
		fmt.Fprintf(buf, " <a href=\"%s\" style=\"font-family: monospace; font-size: 11px; color: %s;\">%s</a>",
			safehtml.Text(commitURL), colorMuted, safehtml.Text(shortHash))
	}
}

func RenderPageProperties(report render.Report, sm AnalyticsData) string {
	var buf bytes.Buffer
	buf.WriteString("<ac:structured-macro ac:name=\"details\">")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString("<table><tbody>")

	dateStr := report.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	fmt.Fprintf(&buf, "<tr><th>Version</th><td>%s</td></tr>", safehtml.Text(report.Version))
	fmt.Fprintf(&buf, "<tr><th>Date</th><td>%s</td></tr>", safehtml.Text(dateStr))

	if sm.ReleaseRiskScore > 0 {
		riskLabel := "Low"
		if sm.ReleaseRiskScore >= 70 {
			riskLabel = "High"
		} else if sm.ReleaseRiskScore >= 40 {
			riskLabel = "Medium"
		}
		fmt.Fprintf(&buf, "<tr><th>Risk Score</th><td>%.0f/100 (%s)</td></tr>", sm.ReleaseRiskScore, riskLabel)
	}
	if sm.BreakingChanges > 0 {
		fmt.Fprintf(&buf, "<tr><th>Breaking Changes</th><td>%d</td></tr>", sm.BreakingChanges)
	}
	if sm.TotalCommits > 0 {
		fmt.Fprintf(&buf, "<tr><th>Commits</th><td>%d</td></tr>", sm.TotalCommits)
	}
	if sm.TotalAuthors > 0 {
		fmt.Fprintf(&buf, "<tr><th>Contributors</th><td>%d</td></tr>", sm.TotalAuthors)
	}

	buf.WriteString("</tbody></table>")
	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

func RenderEpicsPanel(report render.Report) string {
	epicMap := make(map[string][]render.Item)
	report.ForEachItem(func(item *render.Item) {
		for _, j := range item.JiraIssues {
			if j.EpicKey != "" {
				epicMap[j.EpicKey] = append(epicMap[j.EpicKey], *item)
			}
		}
	})

	if len(epicMap) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("<ac:structured-macro ac:name=\"info\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">" + safehtml.Text(activeLabels.EpicsInRelease) + "</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString(Spacer())
	buf.WriteString("<table><tbody>")
	buf.WriteString("<tr><th>Epic</th><th>Issues</th><th>Count</th></tr>")

	for epicKey, items := range epicMap {
		var epicURL string
		var epicSummary string
		for _, item := range items {
			for _, j := range item.JiraIssues {
				if j.EpicKey == epicKey {
					if epicURL == "" && j.URL != "" {
						epicURL = strings.Replace(j.URL, j.Key, epicKey, 1)
					}
					if epicSummary == "" && j.Summary != "" {
						epicSummary = epicKey
					}
					break
				}
			}
		}
		if epicSummary == "" {
			epicSummary = epicKey
		}
		fmt.Fprintf(&buf, "<tr><td>")
		if epicURL != "" {
			fmt.Fprintf(&buf, "<a href=\"%s\">%s</a>", safehtml.Text(epicURL), safehtml.Text(epicKey))
		} else {
			buf.WriteString(safehtml.Text(epicKey))
		}
		fmt.Fprintf(&buf, "</td><td>")
		for i, item := range items {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(safehtml.Text(item.Description))
		}
		fmt.Fprintf(&buf, "</td><td>%d</td></tr>", len(items))
	}

	buf.WriteString("</tbody></table>")
	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

func RenderPrevNextNav(prevPage, nextPage *SiblingPage) string {
	if prevPage == nil && nextPage == nil {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString(`<table style="width: 100%; border: none;"><tbody><tr>`)

	if prevPage != nil {
		fmt.Fprintf(&buf, `<td style="border: none; text-align: left;">← <a href="%s">%s</a></td>`,
			safehtml.Text(prevPage.URL), safehtml.Text(prevPage.Title))
	} else {
		buf.WriteString(`<td style="border: none;"></td>`)
	}

	buf.WriteString(`<td style="border: none; text-align: center;"><strong>Release Notes</strong></td>`)

	if nextPage != nil {
		fmt.Fprintf(&buf, `<td style="border: none; text-align: right;"><a href="%s">%s</a> →</td>`,
			safehtml.Text(nextPage.URL), safehtml.Text(nextPage.Title))
	} else {
		buf.WriteString(`<td style="border: none;"></td>`)
	}

	buf.WriteString("</tr></tbody></table>")
	buf.WriteString(Spacer())
	return buf.String()
}

type PageSections struct {
	PageProperties string
	AISummary      string
	Analytics      string
	ReleaseNotes   string
	EpicsPanel     string
	PrevNextNav    string
	CommandFooter  string
}

const defaultPageTemplate = `{{.PrevNextNav}}{{.PageProperties}}{{if .AISummary}}{{.AISummary}}
{{end}}{{.Analytics}}{{.ReleaseNotes}}{{.EpicsPanel}}{{.CommandFooter}}`

func RenderPageWithTemplate(sections PageSections, templatePath string) (string, error) {
	tmplStr := defaultPageTemplate
	if templatePath != "" {
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("confluence template read: %w", err)
		}
		tmplStr = string(data)
	}

	tmpl, err := template.New("page").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("confluence template parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sections); err != nil {
		return "", fmt.Errorf("confluence template execute: %w", err)
	}

	return buf.String(), nil
}
