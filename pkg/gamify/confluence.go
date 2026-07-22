package gamify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/safehtml"
)

var rarityColors = map[string]string{
	"legendary": "#ff8c00",
	"epic":      "#9b59b6",
	"rare":      "#3498db",
	"common":    "#95a5a6",
}

var rarityLabels = map[string]string{
	"legendary": "LEGENDARY",
	"epic":      "EPIC",
	"rare":      "RARE",
	"common":    "COMMON",
}

var levelColors = []string{
	"#95a5a6",
	"#2ecc71",
	"#3498db",
	"#9b59b6",
	"#e74c3c",
	"#ffd700",
}

func FormatConfluence(achievements []ContributorAchievements) string {
	if len(achievements) == 0 {
		return ""
	}

	sort.Slice(achievements, func(i, j int) bool {
		if achievements[i].Commits != achievements[j].Commits {
			return achievements[i].Commits > achievements[j].Commits
		}
		return achievements[i].Name < achievements[j].Name
	})

	var buf strings.Builder

	buf.WriteString("<ac:structured-macro ac:name=\"info\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">🎮 Hall of Fame</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString("<p style=\"color: #666; font-size: 12px;\">AI-generated achievements based on contributor activity across releases.</p>")
	buf.WriteString("<p>&nbsp;</p>")

	for _, ca := range achievements {
		renderContributorCard(&buf, ca)
	}

	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

func renderContributorCard(buf *strings.Builder, ca ContributorAchievements) {
	levelColor := levelColors[0]
	if ca.Level > 0 && ca.Level <= len(levelColors) {
		levelColor = levelColors[ca.Level-1]
	}

	buf.WriteString(`<div style="border: 1px solid #e1e4e8; border-radius: 8px; padding: 16px; margin-bottom: 12px; background: #fafbfc;">`)

	buf.WriteString(`<div style="display: flex; align-items: center; margin-bottom: 12px;">`)

	fmt.Fprintf(buf, `<div style="width: 48px; height: 48px; border-radius: 50%%; background: %s; color: white; display: flex; align-items: center; justify-content: center; font-weight: bold; font-size: 20px; margin-right: 12px;">%s</div>`,
		levelColor, safehtml.Text(initials(ca.Name)))

	fmt.Fprintf(buf, `<div><span style="font-size: 16px; font-weight: bold; color: #2c3e50;">%s</span>`, safehtml.Text(ca.Name))
	fmt.Fprintf(buf, `<br/><span style="font-size: 12px; color: #666;">%d commits · %s (L%d)`, ca.Commits, safehtml.Text(ca.LevelName), ca.Level)
	if ca.TotalCommits > 0 {
		fmt.Fprintf(buf, " · %d total", ca.TotalCommits)
	}
	if ca.Streak > 1 {
		fmt.Fprintf(buf, " · %d release streak", ca.Streak)
	}
	buf.WriteString(`</span></div>`)

	buf.WriteString(`<div style="margin-left: auto; text-align: right;">`)
	for _, b := range ca.Badges {
		fmt.Fprintf(buf, `<span style="font-size: 20px; margin-left: 4px;" title="%s">%s</span>`, safehtml.Text(b.Reason), safehtml.Text(b.Emoji))
	}
	buf.WriteString(`</div>`)

	buf.WriteString(`</div>`)

	if len(ca.Achievements) > 0 {
		buf.WriteString(`<div style="margin-top: 8px;">`)
		for _, ach := range ca.Achievements {
			renderAchievementBadge(buf, ach)
		}
		buf.WriteString(`</div>`)
	}

	buf.WriteString(`</div>`)
}

func renderAchievementBadge(buf *strings.Builder, ach Achievement) {
	color := rarityColors["common"]
	label := rarityLabels["common"]
	if c, ok := rarityColors[ach.Rarity]; ok {
		color = c
	}
	if l, ok := rarityLabels[ach.Rarity]; ok {
		label = l
	}

	fmt.Fprintf(buf, `<div style="display: inline-block; border: 1px solid %s; border-radius: 6px; padding: 8px 12px; margin: 4px 6px 4px 0; background: white; max-width: 100%%;">`, color)
	fmt.Fprintf(buf, `<div style="display: flex; align-items: center;">`)
	fmt.Fprintf(buf, `<span style="font-size: 22px; margin-right: 8px;">%s</span>`, safehtml.Text(ach.Emoji))
	fmt.Fprintf(buf, `<div>`)
	fmt.Fprintf(buf, `<div style="font-weight: bold; font-size: 13px; color: %s;">%s</div>`, color, safehtml.Text(ach.Title))
	fmt.Fprintf(buf, `<div style="font-size: 11px; color: #666; margin-top: 2px;">%s</div>`, safehtml.Text(ach.Description))
	fmt.Fprintf(buf, `<div style="font-size: 9px; font-weight: bold; color: %s; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px;">%s</div>`, color, safehtml.Text(label))
	fmt.Fprintf(buf, `</div></div></div>`)
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		if len(r) >= 2 {
			return string(r[:2])
		}
		return string(r)
	}
	first := []rune(parts[0])
	last := []rune(parts[len(parts)-1])
	if len(first) > 0 && len(last) > 0 {
		return string(first[0:1]) + string(last[0:1])
	}
	return "?"
}
