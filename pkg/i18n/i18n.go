// Package i18n provides section heading translations and AI prompt localization for en, ru, and zh.
package i18n

import (
	"fmt"
	"strings"
)

type Lang string

const (
	LangEN Lang = "en"
	LangRU Lang = "ru"
	LangZH Lang = "zh"
)

func ParseLang(s string) (Lang, error) {
	switch s {
	case "en", "":
		return LangEN, nil
	case "ru":
		return LangRU, nil
	case "zh":
		return LangZH, nil
	default:
		return LangEN, fmt.Errorf("unknown language %q, use en, ru, or zh", s)
	}
}

func ParseLangs(s string) ([]Lang, error) {
	if s == "" {
		return []Lang{LangEN}, nil
	}
	parts := strings.Split(s, ",")
	var langs []Lang
	for _, p := range parts {
		p = strings.TrimSpace(p)
		l, err := ParseLang(p)
		if err != nil {
			return nil, err
		}
		langs = append(langs, l)
	}
	if len(langs) == 0 {
		return []Lang{LangEN}, nil
	}
	return langs, nil
}

func BilingualHeading(langs []Lang, typeKey string) string {
	if len(langs) == 0 {
		return Heading(LangEN, typeKey)
	}
	if len(langs) == 1 {
		return Heading(langs[0], typeKey)
	}
	var parts []string
	seen := make(map[string]bool)
	for _, l := range langs {
		h := Heading(l, typeKey)
		if !seen[h] {
			seen[h] = true
			parts = append(parts, h)
		}
	}
	return strings.Join(parts, " / ")
}

var sectionHeadings = map[Lang]map[string]string{
	LangEN: {
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"perf":     "Performance Improvements",
		"refactor": "Code Refactoring",
		"docs":     "Documentation",
		"test":     "Tests",
		"style":    "Style / Formatting",
		"ci":       "CI / Build",
		"chore":    "Chores",
		"other":    "Uncategorised",
	},
	LangRU: {
		"feat":     "Возможности",
		"fix":      "Исправления",
		"perf":     "Производительность",
		"refactor": "Рефакторинг",
		"docs":     "Документация",
		"test":     "Тесты",
		"style":    "Стиль / Форматирование",
		"ci":       "CI / Сборка",
		"chore":    "Прочее",
		"other":    "Без категории",
	},
	LangZH: {
		"feat":     "新功能",
		"fix":      "问题修复",
		"perf":     "性能优化",
		"refactor": "代码重构",
		"docs":     "文档",
		"test":     "测试",
		"style":    "样式",
		"ci":       "持续集成",
		"chore":    "杂项",
		"other":    "未分类",
	},
}

var breakingHeadings = map[Lang]string{
	LangEN: "Breaking Changes",
	LangRU: "Критические изменения",
	LangZH: "破坏性变更",
}

var promptPrefixes = map[Lang]string{
	LangEN: "You are a technical writer. Respond in English.",
	LangRU: "Ты — технический писатель. Отвечай на русском.",
	LangZH: "你是一名技术文档工程师。请用中文回答。",
}

var summaryPromptInstructions = map[Lang]string{
	LangEN: "Generate a detailed analytical release summary for the changelog in English.\nInclude analytical insights about metrics, highlight top contributors and key achievements.\nFormat: 1 detailed paragraph (3-5 sentences). No heading.",
	LangRU: "Сгенерируй развёрнутую аналитическую сводку релиза для changelog на русском языке.\nВключи аналитические инсайты о метриках, выдели лучших контрибьюторов и ключевые достижения.\nФормат: 1 развёрнутый абзац (3-5 предложений). Без заголовка.",
	LangZH: "为变更日志生成一份详细的英文分析摘要。\n包含关于指标的分析洞察，突出顶级贡献者和关键成就。\n格式：1个详细段落（3-5句）。无标题。",
}

func Heading(lang Lang, typeKey string) string {
	if headings, ok := sectionHeadings[lang]; ok {
		if h, ok := headings[typeKey]; ok {
			return h
		}
	}
	if headings, ok := sectionHeadings[LangEN]; ok {
		if h, ok := headings[typeKey]; ok {
			return h
		}
	}
	return typeKey
}

func BreakingHeading(lang Lang) string {
	if h, ok := breakingHeadings[lang]; ok {
		return h
	}
	return breakingHeadings[LangEN]
}

func PromptPrefix(lang Lang) string {
	if p, ok := promptPrefixes[lang]; ok {
		return p
	}
	return promptPrefixes[LangEN]
}

func SummaryInstructions(lang Lang) string {
	if s, ok := summaryPromptInstructions[lang]; ok {
		return s
	}
	return summaryPromptInstructions[LangEN]
}

func LocalizeSections(lang Lang, sections map[string]string) map[string]string {
	if lang == LangEN || lang == "" {
		return sections
	}
	out := make(map[string]string, len(sections))
	for typ, heading := range sections {
		if localized, ok := sectionHeadings[lang][typ]; ok {
			out[typ] = localized
		} else {
			out[typ] = heading
		}
	}
	return out
}

type ConfluenceLabels struct {
	AnalyticsTitle     string
	CodeMetrics        string
	Metric             string
	Value              string
	Interpretation     string
	TopContributors    string
	Author             string
	CommitsCol         string
	Share              string
	ReleaseOverview    string
	TotalCommits       string
	UniqueAuthors      string
	DevPeriod          string
	CommitsPerDay      string
	MostActiveDay      string
	FilesChanged       string
	LinesAdded         string
	LinesDeleted       string
	NetGrowth          string
	BreakingChanges    string
	JiraTicketsLinked  string
	ImpactDistribution string
	SignificanceLevel  string
	Count              string
	CommitType         string
	SummaryTitle       string
	EpicsInRelease     string
	Dependencies       string
	Package            string
	Change             string
	Ecosystem          string
	Upstream           string
}

var confluenceLabels = map[Lang]ConfluenceLabels{
	LangEN: {
		AnalyticsTitle:     "📊 Release Metrics & Analytics",
		CodeMetrics:        "🔬 Code Metrics",
		Metric:             "Metric",
		Value:              "Value",
		Interpretation:     "Interpretation",
		TopContributors:    "🏆 Top Contributors",
		Author:             "Author",
		CommitsCol:         "Commits",
		Share:              "Share",
		ReleaseOverview:    "📋 Release Overview",
		TotalCommits:       "Total commits",
		UniqueAuthors:      "Unique authors",
		DevPeriod:          "Development period",
		CommitsPerDay:      "Commits per day",
		MostActiveDay:      "Most active day",
		FilesChanged:       "Files changed",
		LinesAdded:         "Lines added",
		LinesDeleted:       "Lines deleted",
		NetGrowth:          "Net growth",
		BreakingChanges:    "Breaking changes",
		JiraTicketsLinked:  "Jira tickets linked",
		ImpactDistribution: "🎯 Impact Distribution",
		SignificanceLevel:  "Significance level",
		Count:              "Count",
		CommitType:         "Commit type",
		SummaryTitle:       "📝 Analytical Summary",
		EpicsInRelease:     "🎯 Epics in this release",
		Dependencies:       "📦 Dependencies",
		Package:            "Package",
		Change:             "Change",
		Ecosystem:          "Ecosystem",
		Upstream:           "Upstream",
	},
	LangRU: {
		AnalyticsTitle:     "📊 Метрики и аналитика релиза",
		CodeMetrics:        "🔬 Метрики кода",
		Metric:             "Метрика",
		Value:              "Значение",
		Interpretation:     "Интерпретация",
		TopContributors:    "🏆 Топ контрибьюторов",
		Author:             "Автор",
		CommitsCol:         "Коммитов",
		Share:              "Доля",
		ReleaseOverview:    "📋 Обзор релиза",
		TotalCommits:       "Всего коммитов",
		UniqueAuthors:      "Уникальных авторов",
		DevPeriod:          "Период разработки",
		CommitsPerDay:      "Коммитов в день",
		MostActiveDay:      "Самый активный день",
		FilesChanged:       "Файлов изменено",
		LinesAdded:         "Строк добавлено",
		LinesDeleted:       "Строк удалено",
		NetGrowth:          "Чистый прирост",
		BreakingChanges:    "Breaking changes",
		JiraTicketsLinked:  "Привязано Jira-тикетов",
		ImpactDistribution: "🎯 Распределение изменений",
		SignificanceLevel:  "Уровень значимости",
		Count:              "Количество",
		CommitType:         "Тип коммита",
		SummaryTitle:       "📝 Аналитическая сводка",
		EpicsInRelease:     "🎯 Epics в этом релизе",
		Dependencies:       "📦 Зависимости",
		Package:            "Пакет",
		Change:             "Изменение",
		Ecosystem:          "Экосистема",
		Upstream:           "Upstream",
	},
	LangZH: {
		AnalyticsTitle:     "📊 发布指标与分析",
		CodeMetrics:        "🔬 代码指标",
		Metric:             "指标",
		Value:              "值",
		Interpretation:     "解读",
		TopContributors:    "🏆 顶级贡献者",
		Author:             "作者",
		CommitsCol:         "提交数",
		Share:              "占比",
		ReleaseOverview:    "📋 发布概览",
		TotalCommits:       "总提交数",
		UniqueAuthors:      "独立作者数",
		DevPeriod:          "开发周期",
		CommitsPerDay:      "每日提交数",
		MostActiveDay:      "最活跃日期",
		FilesChanged:       "变更文件数",
		LinesAdded:         "新增行数",
		LinesDeleted:       "删除行数",
		NetGrowth:          "净增长",
		BreakingChanges:    "破坏性变更",
		JiraTicketsLinked:  "关联 Jira 工单数",
		ImpactDistribution: "🎯 影响分布",
		SignificanceLevel:  "重要性级别",
		Count:              "数量",
		CommitType:         "提交类型",
		SummaryTitle:       "📝 分析摘要",
		EpicsInRelease:     "🎯 本次发布的 Epic",
		Dependencies:       "📦 依赖项",
		Package:            "包",
		Change:             "变更",
		Ecosystem:          "生态系统",
		Upstream:           "上游",
	},
}

func ConfluenceLabelsFor(lang Lang) ConfluenceLabels {
	if labels, ok := confluenceLabels[lang]; ok {
		return labels
	}
	return confluenceLabels[LangEN]
}
