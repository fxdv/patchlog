package confluence

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/safehtml"
)

func RenderSummaryPanel(summary string) string {
	var buf bytes.Buffer
	buf.WriteString("<ac:structured-macro ac:name=\"info\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">" + safehtml.Text(activeLabels.SummaryTitle) + "</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString(Spacer())
	buf.WriteString(`<div style="border-left: 3px solid ` + colorAccent + `; padding: 10px 0 10px 16px; margin: 0;">`)
	fmt.Fprintf(&buf, `<p style="font-size: 15px; line-height: 1.75; color: %s; margin: 0;">%s</p>`, colorDarkText, safehtml.Text(summary))
	buf.WriteString("</div>")
	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

type analyticsMetrics interface{}

type AnalyticsData struct {
	TotalCommits                     int
	TotalAuthors                     int
	TopContributors                  []ContributorEntry
	BreakingChanges                  int
	SignificanceCounts               map[string]int
	TypeCounts                       map[string]int
	DateRange                        string
	CommitsPerDay                    float64
	MostActiveDay                    string
	MostActiveDayCount               int
	FilesTouched                     int
	LinesAdded                       int
	LinesDeleted                     int
	NetLines                         int
	JiraTicketsLinked                int
	HotspotDensity                   float64
	ChurnFactor                      float64
	ComplexityPerFeat                float64
	ReleaseCommitSpanHours           float64
	ReleaseAgeHours                  float64
	OwnershipConc                    float64
	ReleaseContributionConcentration int
	FixToFeatureRatio                float64
	TestToSourceRatio                float64
	RefactoringRatio                 float64
	APISurfaceChange                 int
	ReleaseRiskScore                 float64
	BatchFactor                      float64
	RevertRate                       float64
	ScopeIsolation                   float64
	CrossCuttingPct                  float64
	FileVolatility                   float64
	Interpretations                  map[string]string
	MetricsNarrative                 string
	ChangeComplexityProxy            float64
	CrossCuttingChangeRisk           float64
	TechnicalDebtUSD                 float64
	TouchedTestFileRatio             float64
	OwnershipEntropy                 float64
	HotspotScore                     float64
}

type ContributorEntry struct {
	Name    string
	Commits int
}

func RenderAnalyticsPanel(sm AnalyticsData) string {
	if sm.TotalCommits == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("<ac:structured-macro ac:name=\"info\">")
	buf.WriteString("<ac:parameter ac:name=\"title\">" + safehtml.Text(activeLabels.AnalyticsTitle) + "</ac:parameter>")
	buf.WriteString("<ac:rich-text-body>")
	buf.WriteString(Spacer())

	renderCodeMetricsTable(&buf, sm)
	renderMetricsNarrative(&buf, sm)
	renderContributorsTable(&buf, sm.TopContributors)
	renderOverviewTable(&buf, sm)
	renderImpactTable(&buf, sm.SignificanceCounts, sm.TypeCounts)

	buf.WriteString("</ac:rich-text-body></ac:structured-macro>")
	return buf.String()
}

func renderCodeMetricsTable(buf *bytes.Buffer, sm AnalyticsData) {
	hasAny := sm.ReleaseCommitSpanHours > 0 || sm.ReleaseAgeHours > 0 || sm.BatchFactor > 0 ||
		sm.ReleaseRiskScore > 0 || sm.ReleaseContributionConcentration > 0 || sm.OwnershipConc > 0 ||
		sm.OwnershipEntropy > 0 || sm.HotspotDensity > 0 || sm.HotspotScore > 0 ||
		sm.FileVolatility > 0 || sm.ChurnFactor > 0 || sm.ComplexityPerFeat > 0 ||
		sm.ChangeComplexityProxy > 0 || sm.CrossCuttingChangeRisk > 0 || sm.TechnicalDebtUSD > 0 ||
		sm.TouchedTestFileRatio > 0 || sm.FixToFeatureRatio > 0 || sm.RevertRate > 0 ||
		sm.RefactoringRatio > 0 || sm.TestToSourceRatio > 0 || sm.ScopeIsolation > 0 ||
		sm.CrossCuttingPct > 0 || sm.APISurfaceChange > 0
	if !hasAny {
		return
	}

	buf.WriteString("<h3>" + safehtml.Text(activeLabels.CodeMetrics) + "</h3>")
	buf.WriteString(`<table><tbody>`)
	buf.WriteString("<tr><th>" + safehtml.Text(activeLabels.Metric) + "</th><th>" + safehtml.Text(activeLabels.Value) + "</th><th>" + safehtml.Text(activeLabels.Interpretation) + "</th></tr>")

	renderMetricRow := func(key, label, value, fallback string) {
		interp := fallback
		if sm.Interpretations != nil {
			if ai, ok := sm.Interpretations[key]; ok && ai != "" {
				interp = safehtml.Text(ai)
			}
		}
		fmt.Fprintf(buf, "<tr><td><strong>%s</strong></td><td>%s</td><td>%s</td></tr>", label, value, interp)
	}

	if sm.ReleaseRiskScore > 0 {
		riskLabel := "Низкий"
		riskColor := colorGreen
		if sm.ReleaseRiskScore >= 70 {
			riskLabel = "Высокий"
			riskColor = colorRed
		} else if sm.ReleaseRiskScore >= 40 {
			riskLabel = "Средний"
			riskColor = colorYellow
		}
		gauge := renderProgressBar(sm.ReleaseRiskScore, riskColor)
		renderMetricRow("release_risk_score", "Release Risk Score",
			fmt.Sprintf(`%s<br/><span style="color: %s;"><strong>%.0f/100 (%s)</strong></span>`, gauge, riskColor, sm.ReleaseRiskScore, riskLabel),
			riskLabel+" риск релиза")
	}

	if sm.ReleaseCommitSpanHours > 0 {
		fallback := "Быстрый цикл"
		if sm.ReleaseCommitSpanHours >= 24*14 {
			fallback = "Длительный цикл"
		} else if sm.ReleaseCommitSpanHours >= 24*7 {
			fallback = "Средний цикл"
		}
		renderMetricRow("release_commit_span_hours", "Release Commit Span", formatDuration(sm.ReleaseCommitSpanHours), fallback)
	}

	if sm.ReleaseAgeHours > 0 {
		fallback := "Быстрая поставка"
		if sm.ReleaseAgeHours >= 24*30 {
			fallback = "Медленная поставка"
		} else if sm.ReleaseAgeHours >= 24*14 {
			fallback = "Средняя поставка"
		}
		renderMetricRow("release_age_hours", "Release Age for Changes", formatDuration(sm.ReleaseAgeHours), fallback)
	}

	if sm.BatchFactor > 0 {
		fallback := "Регулярный каденс"
		if sm.BatchFactor >= 2.0 {
			fallback = "Пачечные коммиты (batch)"
		} else if sm.BatchFactor >= 1.0 {
			fallback = "Умеренная нерегулярность"
		}
		renderMetricRow("batch_factor", "Batch Factor / Cadence Regularity",
			fmt.Sprintf("%.2f", sm.BatchFactor), fallback)
	}

	if sm.ReleaseContributionConcentration > 0 {
		fallback := "Несколько участников составляют 80% коммитов релиза"
		if sm.ReleaseContributionConcentration == 1 {
			fallback = "Один участник составил не менее 80% коммитов релиза"
		} else if sm.ReleaseContributionConcentration <= 2 {
			fallback = "80% коммитов релиза сосредоточены у двух участников"
		}
		renderMetricRow("release_contribution_concentration", "Release Contribution Concentration", fmt.Sprintf("%d", sm.ReleaseContributionConcentration), fallback)
	}

	if sm.OwnershipConc > 0 {
		fallback := "Распределённое владение"
		if sm.OwnershipConc >= 60 {
			fallback = "Высокая концентрация (release contribution concentration = 1)"
		} else if sm.OwnershipConc >= 40 {
			fallback = "Умеренная концентрация"
		}
		renderMetricRow("ownership_conc", "Code Ownership Concentration",
			fmt.Sprintf("%.0f%%", sm.OwnershipConc), fallback)
	}

	if sm.HotspotDensity > 0 {
		fallback := "Низкая концентрация"
		if sm.HotspotDensity >= 50 {
			fallback = "Высокая концентрация изменений"
		} else if sm.HotspotDensity >= 30 {
			fallback = "Средняя концентрация"
		}
		renderMetricRow("hotspot_density", "Hotspot Density",
			fmt.Sprintf("%.0f%%", sm.HotspotDensity), fallback)
	}

	if sm.FileVolatility > 0 {
		fallback := "Стабильная кодовая база"
		if sm.FileVolatility >= 3.0 {
			fallback = "Высокая волатильность файлов"
		} else if sm.FileVolatility >= 1.5 {
			fallback = "Умеренная волатильность"
		}
		renderMetricRow("file_volatility", "File Volatility",
			fmt.Sprintf("%.1f изменений/файл", sm.FileVolatility), fallback)
	}

	if sm.ChurnFactor > 0 {
		fallback := "Стабильные правки"
		if sm.ChurnFactor >= 20 {
			fallback = "Высокая турбулентность"
		} else if sm.ChurnFactor >= 10 {
			fallback = "Умеренная турбулентность"
		}
		renderMetricRow("churn_factor", "Churn Factor",
			fmt.Sprintf("%.1fx", sm.ChurnFactor), fallback)
	}

	if sm.ComplexityPerFeat > 0 {
		fallback := "Лёгкие фичи"
		if sm.ComplexityPerFeat >= 1000 {
			fallback = "Тяжёлые фичи"
		} else if sm.ComplexityPerFeat >= 300 {
			fallback = "Средние фичи"
		}
		renderMetricRow("complexity_per_feat", "Complexity per Feature",
			fmt.Sprintf("%.0f строк/фичу", sm.ComplexityPerFeat), fallback)
	}

	if sm.FixToFeatureRatio > 0 {
		fallback := "Здоровый баланс"
		if sm.FixToFeatureRatio >= 3.0 {
			fallback = "Дефекты доминируют над фичами"
		} else if sm.FixToFeatureRatio >= 1.5 {
			fallback = "Много багфиксов"
		}
		renderMetricRow("fix_to_feature_ratio", "Fix-to-Feature Ratio",
			fmt.Sprintf("%.1f:1", sm.FixToFeatureRatio), fallback)
	}

	if sm.RevertRate > 0 {
		fallback := "Стабильные коммиты"
		if sm.RevertRate >= 10 {
			fallback = "Высокий откат коммитов"
		} else if sm.RevertRate >= 5 {
			fallback = "Умеренный откат"
		}
		renderMetricRow("revert_rate", "Revert Rate",
			fmt.Sprintf("%.1f%%", sm.RevertRate), fallback)
	}

	if sm.RefactoringRatio > 0 {
		fallback := "Фокус на новых фичах"
		if sm.RefactoringRatio >= 50 {
			fallback = "Релиз техдолга"
		} else if sm.RefactoringRatio >= 30 {
			fallback = "Активная уборка кода"
		}
		renderMetricRow("refactoring_ratio", "Refactoring Ratio",
			fmt.Sprintf("%.0f%%", sm.RefactoringRatio), fallback)
	}

	if sm.TestToSourceRatio > 0 {
		fallback := "Мало затронутых тестовых файлов относительно исходных"
		if sm.TestToSourceRatio >= 50 {
			fallback = "Много затронутых тестовых файлов относительно исходных; это не покрытие"
		} else if sm.TestToSourceRatio >= 20 {
			fallback = "Умеренное отношение затронутых тестовых файлов к исходным"
		}
		renderMetricRow("test_to_source_ratio", "Test-to-Source Ratio",
			fmt.Sprintf("%.0f%%", sm.TestToSourceRatio), fallback)
	}

	if sm.ScopeIsolation > 0 {
		fallback := "Низкая изоляция скоупов"
		if sm.ScopeIsolation >= 70 {
			fallback = "Хорошая изоляция скоупов"
		} else if sm.ScopeIsolation >= 40 {
			fallback = "Умеренная изоляция"
		}
		renderMetricRow("scope_isolation", "Scope Isolation",
			fmt.Sprintf("%.0f%%", sm.ScopeIsolation), fallback)
	}

	if sm.CrossCuttingPct > 0 {
		fallback := "Изолированные изменения"
		if sm.CrossCuttingPct >= 50 {
			fallback = "Много сквозных изменений"
		} else if sm.CrossCuttingPct >= 30 {
			fallback = "Умеренные cross-cutting"
		}
		renderMetricRow("cross_cutting_pct", "Cross-Cutting Concerns",
			fmt.Sprintf("%.0f%%", sm.CrossCuttingPct), fallback)
	}

	if sm.APISurfaceChange > 0 {
		fallback := "Стабильный API"
		if sm.APISurfaceChange >= 10 {
			fallback = "Масштабное изменение API"
		} else if sm.APISurfaceChange >= 3 {
			fallback = "Частичное изменение API"
		}
		renderMetricRow("api_surface_change", "API Surface Change",
			fmt.Sprintf("%d файлов", sm.APISurfaceChange), fallback)
	}

	if sm.HotspotScore > 0 {
		scoreLabel := "Низкий"
		scoreColor := colorGreen
		if sm.HotspotScore >= 70 {
			scoreLabel = "Высокий"
			scoreColor = colorRed
		} else if sm.HotspotScore >= 40 {
			scoreLabel = "Средний"
			scoreColor = colorYellow
		}
		gauge := renderProgressBar(sm.HotspotScore, scoreColor)
		renderMetricRow("hotspot_score", "🔥 Hotspot Score",
			fmt.Sprintf(`%s<br/><span style="color: %s;"><strong>%.0f/100 (%s)</strong></span>`, gauge, scoreColor, sm.HotspotScore, scoreLabel),
			scoreLabel+" риск hotspots")
	}

	if sm.ChangeComplexityProxy > 0 {
		fallback := "Низкое значение эвристики изменений"
		if sm.ChangeComplexityProxy >= 10 {
			fallback = "Высокое значение эвристики изменений"
		} else if sm.ChangeComplexityProxy >= 5 {
			fallback = "Умеренная сложность"
		}
		renderMetricRow("change_complexity_proxy", "🧠 Change Complexity Proxy",
			fmt.Sprintf("%.1f", sm.ChangeComplexityProxy), fallback)
	}

	if sm.CrossCuttingChangeRisk > 0 {
		fallback := "Изменения преимущественно локализованы; это не граф зависимостей"
		if sm.CrossCuttingChangeRisk >= 60 {
			fallback = "Высокая доля сквозных изменений; требуется языковой граф зависимостей"
		} else if sm.CrossCuttingChangeRisk >= 35 {
			fallback = "Умеренная связность модулей"
		}
		riskColor := colorGreen
		if sm.CrossCuttingChangeRisk >= 60 {
			riskColor = colorRed
		} else if sm.CrossCuttingChangeRisk >= 35 {
			riskColor = colorYellow
		}
		gauge := renderProgressBar(sm.CrossCuttingChangeRisk, riskColor)
		renderMetricRow("cross_cutting_change_risk", "🔄 Cross-Cutting Change Risk",
			fmt.Sprintf(`%s<br/><span style="color: %s;"><strong>%.0f%%</strong></span>`, gauge, riskColor, sm.CrossCuttingChangeRisk),
			fallback)
	}

	if sm.TechnicalDebtUSD > 0 {
		fallback := "Низкий техдолг"
		if sm.TechnicalDebtUSD >= 10000 {
			fallback = "Критичный техдолг"
		} else if sm.TechnicalDebtUSD >= 3000 {
			fallback = "Заметный техдолг"
		}
		renderMetricRow("technical_debt_usd", "💰 Technical Debt",
			fmt.Sprintf("$%.0f", sm.TechnicalDebtUSD), fallback)
	}

	if sm.TouchedTestFileRatio > 0 {
		fallback := "Низкая доля затронутых тестовых файлов; это не покрытие кода"
		if sm.TouchedTestFileRatio >= 50 {
			fallback = "Высокая доля затронутых тестовых файлов; покрытие берите из CI"
		} else if sm.TouchedTestFileRatio >= 25 {
			fallback = "Умеренная доля затронутых тестовых файлов; это не покрытие"
		}
		coverColor := colorRed
		if sm.TouchedTestFileRatio >= 50 {
			coverColor = colorGreen
		} else if sm.TouchedTestFileRatio >= 25 {
			coverColor = colorYellow
		}
		gauge := renderProgressBar(sm.TouchedTestFileRatio, coverColor)
		renderMetricRow("touched_test_file_ratio", "🧪 Touched Test File Ratio",
			fmt.Sprintf(`%s<br/><span style="color: %s;"><strong>%.0f%%</strong></span>`, gauge, coverColor, sm.TouchedTestFileRatio),
			fallback)
	}

	if sm.OwnershipEntropy > 0 {
		fallback := "Низкая энтропия владения"
		if sm.OwnershipEntropy >= 0.8 {
			fallback = "Здоровое распределение знаний"
		} else if sm.OwnershipEntropy >= 0.5 {
			fallback = "Умеренная концентрация знаний"
		} else {
			fallback = "Высокая концентрация знаний"
		}
		renderMetricRow("ownership_entropy", "👥 Ownership Entropy",
			fmt.Sprintf("%.2f", sm.OwnershipEntropy), fallback)
	}

	buf.WriteString("</tbody></table>")
}

func renderMetricsNarrative(buf *bytes.Buffer, sm AnalyticsData) {
	if strings.TrimSpace(sm.MetricsNarrative) == "" {
		return
	}
	buf.WriteString(Spacer())
	buf.WriteString(`<div style="border-left: 3px solid ` + colorAccent + `; padding: 10px 0 10px 16px; margin: 8px 0 0 0;">`)
	fmt.Fprintf(buf, `<p style="font-size: 14px; line-height: 1.7; color: %s; margin: 0;">%s</p>`, colorDarkText, safehtml.Text(sm.MetricsNarrative))
	buf.WriteString("</div>")
}

func formatDuration(hours float64) string {
	if hours < 24 {
		return fmt.Sprintf("%.0f ч", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%.1f д", days)
	}
	months := days / 30
	return fmt.Sprintf("%.1f мес", months)
}

func renderProgressBar(pct float64, color string) string {
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf(`<div style="background: #eee; border-radius: 4px; width: 180px; height: 16px; overflow: hidden;"><div style="background: %s; width: %.0f%%; height: 16px; border-radius: 4px;"></div></div>`, color, pct)
}

func renderStackedBar(segments []struct {
	pct   float64
	color string
}) string {
	var buf bytes.Buffer
	buf.WriteString(`<div style="display: flex; border-radius: 4px; width: 180px; height: 16px; overflow: hidden;">`)
	for _, seg := range segments {
		if seg.pct <= 0 {
			continue
		}
		fmt.Fprintf(&buf, `<div style="background: %s; width: %.0f%%; height: 16px;"></div>`, seg.color, seg.pct)
	}
	buf.WriteString("</div>")
	return buf.String()
}

func renderContributorsTable(buf *bytes.Buffer, contributors []ContributorEntry) {
	if len(contributors) == 0 {
		return
	}

	buf.WriteString("<h3>" + safehtml.Text(activeLabels.TopContributors) + "</h3>")
	buf.WriteString(`<table><tbody>`)
	buf.WriteString("<tr><th>#</th><th>" + safehtml.Text(activeLabels.Author) + "</th><th>" + safehtml.Text(activeLabels.CommitsCol) + "</th><th>" + safehtml.Text(activeLabels.Share) + "</th></tr>")

	totalCommits := 0
	for _, c := range contributors {
		totalCommits += c.Commits
	}

	for i, c := range contributors {
		pct := 0.0
		if totalCommits > 0 {
			pct = float64(c.Commits) / float64(totalCommits) * 100
		}
		medal := ""
		switch i {
		case 0:
			medal = "🥇 "
		case 1:
			medal = "🥈 "
		case 2:
			medal = "🥉 "
		}
		fmt.Fprintf(buf, "<tr><td>%d</td><td>%s%s</td><td>%d</td><td>%.0f%%</td></tr>",
			i+1, medal, safehtml.Text(c.Name), c.Commits, pct)
	}
	buf.WriteString("</tbody></table>")
}

func renderOverviewTable(buf *bytes.Buffer, sm AnalyticsData) {
	buf.WriteString("<h3>" + safehtml.Text(activeLabels.ReleaseOverview) + "</h3>")
	buf.WriteString(`<table><tbody>`)
	buf.WriteString("<tr><th>" + safehtml.Text(activeLabels.Metric) + "</th><th>" + safehtml.Text(activeLabels.Value) + "</th></tr>")
	fmt.Fprintf(buf, "<tr><td>%s</td><td><strong>%d</strong></td></tr>", activeLabels.TotalCommits, sm.TotalCommits)
	fmt.Fprintf(buf, "<tr><td>%s</td><td><strong>%d</strong></td></tr>", activeLabels.UniqueAuthors, sm.TotalAuthors)
	if sm.DateRange != "" {
		fmt.Fprintf(buf, "<tr><td>%s</td><td>%s</td></tr>", activeLabels.DevPeriod, safehtml.Text(sm.DateRange))
	}
	if sm.CommitsPerDay > 0 {
		fmt.Fprintf(buf, "<tr><td>%s</td><td>%.1f</td></tr>", activeLabels.CommitsPerDay, sm.CommitsPerDay)
	}
	if sm.MostActiveDay != "" {
		fmt.Fprintf(buf, "<tr><td>%s</td><td>%s (%d)</td></tr>",
			activeLabels.MostActiveDay, safehtml.Text(sm.MostActiveDay), sm.MostActiveDayCount)
	}
	fmt.Fprintf(buf, "<tr><td>%s</td><td>%d</td></tr>", activeLabels.FilesChanged, sm.FilesTouched)
	fmt.Fprintf(buf, "<tr><td>%s</td><td><span style=\"color: %s;\">+%d</span></td></tr>", activeLabels.LinesAdded, colorGreen, sm.LinesAdded)
	fmt.Fprintf(buf, "<tr><td>%s</td><td><span style=\"color: %s;\">-%d</span></td></tr>", activeLabels.LinesDeleted, colorRed, sm.LinesDeleted)
	fmt.Fprintf(buf, "<tr><td>%s</td><td><strong>%+d</strong></td></tr>", activeLabels.NetGrowth, sm.NetLines)
	if sm.BreakingChanges > 0 {
		fmt.Fprintf(buf, "<tr><td>%s</td><td><span style=\"color: %s;\"><strong>%d</strong></span></td></tr>", activeLabels.BreakingChanges, colorRed, sm.BreakingChanges)
	}
	if sm.JiraTicketsLinked > 0 {
		fmt.Fprintf(buf, "<tr><td>%s</td><td>%d</td></tr>", activeLabels.JiraTicketsLinked, sm.JiraTicketsLinked)
	}
	buf.WriteString("</tbody></table>")
}

func renderImpactTable(buf *bytes.Buffer, significanceCounts, typeCounts map[string]int) {
	hasSig := false
	for _, lvl := range []string{"major", "minor", "patch"} {
		if c, ok := significanceCounts[lvl]; ok && c > 0 {
			hasSig = true
			break
		}
	}
	if !hasSig && len(typeCounts) == 0 {
		return
	}

	buf.WriteString("<h3>" + safehtml.Text(activeLabels.ImpactDistribution) + "</h3>")
	buf.WriteString(`<table><tbody>`)

	if hasSig {
		buf.WriteString("<tr><th>" + safehtml.Text(activeLabels.SignificanceLevel) + "</th><th>" + safehtml.Text(activeLabels.Count) + "</th><th>" + safehtml.Text(activeLabels.Share) + "</th></tr>")
		sigTotal := 0
		for _, lvl := range []string{"major", "minor", "patch"} {
			sigTotal += significanceCounts[lvl]
		}
		sigLabels := map[string]string{"major": "🔴 Major", "minor": "🟡 Minor", "patch": "🟢 Patch"}
		sigColors := map[string]string{"major": colorRed, "minor": colorYellow, "patch": colorGreen}
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := significanceCounts[lvl]; ok && c > 0 {
				pct := 0.0
				if sigTotal > 0 {
					pct = float64(c) / float64(sigTotal) * 100
				}
				fmt.Fprintf(buf, "<tr><td>%s</td><td>%d</td><td>%.0f%%</td></tr>",
					sigLabels[lvl], c, pct)
			}
		}
		var barSegments []struct {
			pct   float64
			color string
		}
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := significanceCounts[lvl]; ok && c > 0 && sigTotal > 0 {
				pct := float64(c) / float64(sigTotal) * 100
				barSegments = append(barSegments, struct {
					pct   float64
					color string
				}{pct, sigColors[lvl]})
			}
		}
		if len(barSegments) > 0 {
			fmt.Fprintf(buf, "<tr><td colspan=\"3\">%s</td></tr>", renderStackedBar(barSegments))
		}
	}

	if len(typeCounts) > 0 {
		buf.WriteString("<tr><th>" + safehtml.Text(activeLabels.CommitType) + "</th><th>" + safehtml.Text(activeLabels.Count) + "</th><th></th></tr>")
		typeLabels := map[string]string{
			"feat": "✨ feat", "fix": "🐛 fix", "perf": "⚡ perf",
			"refactor": "♻️ refactor", "docs": "📚 docs", "test": "🧪 test",
			"ci": "🔧 ci", "chore": "📦 chore", "style": "🎨 style",
			"revert": "⏪ revert", "other": "❓ other",
		}
		typeOrder := []string{"feat", "fix", "perf", "refactor", "docs", "test", "ci", "chore", "style", "revert", "other"}
		for _, t := range typeOrder {
			if c, ok := typeCounts[t]; ok && c > 0 {
				label := typeLabels[t]
				if label == "" {
					label = t
				}
				fmt.Fprintf(buf, "<tr><td>%s</td><td>%d</td><td></td></tr>", label, c)
			}
		}
	}

	buf.WriteString("</tbody></table>")
}
