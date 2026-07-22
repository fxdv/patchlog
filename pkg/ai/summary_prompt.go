package ai

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

func BuildSummaryPrompt(report render.Report, sm SummaryMetrics, lang i18n.Lang) string {
	var buf strings.Builder

	buf.WriteString(i18n.SummaryInstructions(lang))
	buf.WriteString("\n\n")

	fmt.Fprintf(&buf, "Version: %s\n", report.Version)
	if report.Date != "" {
		fmt.Fprintf(&buf, "Date: %s\n", report.Date)
	}

	switch lang {
	case i18n.LangRU:
		writeSummaryMetricsRU(&buf, sm)
	case i18n.LangZH:
		writeSummaryMetricsEN(&buf, sm)
	default:
		writeSummaryMetricsEN(&buf, sm)
	}

	if len(sm.TopContributors) > 0 {
		switch lang {
		case i18n.LangRU:
			buf.WriteString("\nТоп контрибьюторов:\n")
		default:
			buf.WriteString("\nTop contributors:\n")
		}
		limit := 5
		if len(sm.TopContributors) < limit {
			limit = len(sm.TopContributors)
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&buf, "- %s: %d\n", sm.TopContributors[i].Name, sm.TopContributors[i].Commits)
		}
	}

	switch lang {
	case i18n.LangRU:
		buf.WriteString("\nИзменения:\n")
	default:
		buf.WriteString("\nChanges:\n")
	}

	if len(report.Breaking) > 0 {
		switch lang {
		case i18n.LangRU:
			buf.WriteString("Breaking:\n")
		default:
			buf.WriteString("Breaking:\n")
		}
		for _, item := range report.Breaking {
			fmt.Fprintf(&buf, "- %s\n", item.Description)
		}
	}

	for _, s := range report.Sections {
		if len(s.Items) == 0 && len(s.Scopes) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "\n%s:\n", s.Heading)
		for _, item := range s.Items {
			fmt.Fprintf(&buf, "- %s\n", item.Description)
		}
		for _, sg := range s.Scopes {
			for _, item := range sg.Items {
				fmt.Fprintf(&buf, "- %s: %s\n", sg.Name, item.Description)
			}
		}
	}

	switch lang {
	case i18n.LangRU:
		buf.WriteString("\nНапиши развёрнутую аналитическую сводку на русском языке. ")
		buf.WriteString("Отрази ключевые изменения, проанализируй метрики (объём кода, активность, распределение), ")
		buf.WriteString("выдели лучших контрибьюторов и их вклад. ")
		buf.WriteString("Только текст сводки, без заголовка.\n")
	default:
		buf.WriteString("\nWrite a detailed analytical summary in English. ")
		buf.WriteString("Reflect key changes, analyze metrics (code volume, activity, distribution), ")
		buf.WriteString("highlight top contributors and their impact. ")
		buf.WriteString("Summary text only, no heading.\n")
	}
	return buf.String()
}

func writeSummaryMetricsRU(buf *strings.Builder, sm SummaryMetrics) {
	buf.WriteString("\nМетрики релиза:\n")
	fmt.Fprintf(buf, "- Всего коммитов: %d\n", sm.TotalCommits)
	fmt.Fprintf(buf, "- Уникальных авторов: %d\n", sm.TotalAuthors)
	fmt.Fprintf(buf, "- Breaking changes: %d\n", sm.BreakingChanges)
	if sm.DateRange != "" {
		fmt.Fprintf(buf, "- Период разработки: %s\n", sm.DateRange)
	}
	if sm.CommitsPerDay > 0 {
		fmt.Fprintf(buf, "- Коммитов в день: %.1f\n", sm.CommitsPerDay)
	}
	if sm.MostActiveDay != "" {
		fmt.Fprintf(buf, "- Самый активный день: %s (%d коммитов)\n", sm.MostActiveDay, sm.MostActiveDayCount)
	}
	fmt.Fprintf(buf, "- Файлов изменено: %d\n", sm.FilesTouched)
	fmt.Fprintf(buf, "- Строк добавлено: %d\n", sm.LinesAdded)
	fmt.Fprintf(buf, "- Строк удалено: %d\n", sm.LinesDeleted)
	fmt.Fprintf(buf, "- Чистый прирост строк: %+d\n", sm.NetLines)
	if sm.JiraTicketsLinked > 0 {
		fmt.Fprintf(buf, "- Привязано Jira-тикетов: %d\n", sm.JiraTicketsLinked)
	}
	if len(sm.SignificanceCounts) > 0 {
		buf.WriteString("- Распределение значимости:")
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := sm.SignificanceCounts[lvl]; ok && c > 0 {
				fmt.Fprintf(buf, " %s=%d", lvl, c)
			}
		}
		buf.WriteString("\n")
	}
	if len(sm.TypeCounts) > 0 {
		buf.WriteString("- Типы коммитов:")
		for _, t := range []string{"feat", "fix", "perf", "refactor", "docs", "test", "ci", "chore", "other"} {
			if c, ok := sm.TypeCounts[t]; ok && c > 0 {
				fmt.Fprintf(buf, " %s=%d", t, c)
			}
		}
		buf.WriteString("\n")
	}
}

func writeSummaryMetricsEN(buf *strings.Builder, sm SummaryMetrics) {
	buf.WriteString("\nRelease metrics:\n")
	fmt.Fprintf(buf, "- Total commits: %d\n", sm.TotalCommits)
	fmt.Fprintf(buf, "- Unique authors: %d\n", sm.TotalAuthors)
	fmt.Fprintf(buf, "- Breaking changes: %d\n", sm.BreakingChanges)
	if sm.DateRange != "" {
		fmt.Fprintf(buf, "- Development period: %s\n", sm.DateRange)
	}
	if sm.CommitsPerDay > 0 {
		fmt.Fprintf(buf, "- Commits per day: %.1f\n", sm.CommitsPerDay)
	}
	if sm.MostActiveDay != "" {
		fmt.Fprintf(buf, "- Most active day: %s (%d commits)\n", sm.MostActiveDay, sm.MostActiveDayCount)
	}
	fmt.Fprintf(buf, "- Files changed: %d\n", sm.FilesTouched)
	fmt.Fprintf(buf, "- Lines added: %d\n", sm.LinesAdded)
	fmt.Fprintf(buf, "- Lines deleted: %d\n", sm.LinesDeleted)
	fmt.Fprintf(buf, "- Net lines: %+d\n", sm.NetLines)
	if sm.JiraTicketsLinked > 0 {
		fmt.Fprintf(buf, "- Jira tickets linked: %d\n", sm.JiraTicketsLinked)
	}
	if len(sm.SignificanceCounts) > 0 {
		buf.WriteString("- Significance distribution:")
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := sm.SignificanceCounts[lvl]; ok && c > 0 {
				fmt.Fprintf(buf, " %s=%d", lvl, c)
			}
		}
		buf.WriteString("\n")
	}
	if len(sm.TypeCounts) > 0 {
		buf.WriteString("- Commit types:")
		for _, t := range []string{"feat", "fix", "perf", "refactor", "docs", "test", "ci", "chore", "other"} {
			if c, ok := sm.TypeCounts[t]; ok && c > 0 {
				fmt.Fprintf(buf, " %s=%d", t, c)
			}
		}
		buf.WriteString("\n")
	}
}
