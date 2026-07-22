package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/i18n"
)

func GenerateMetricInterpretations(ctx context.Context, sm SummaryMetrics, client Client, lang i18n.Lang) (map[string]string, error) {
	if client == nil {
		return nil, fmt.Errorf("no AI client")
	}

	snapshot := map[string]any{
		"release_commit_span_hours":          sm.ReleaseCommitSpanHours,
		"release_age_hours":                  sm.ReleaseAgeHours,
		"batch_factor":                       sm.BatchFactor,
		"release_risk_score":                 sm.ReleaseRiskScore,
		"release_contribution_concentration": sm.ReleaseContributionConcentration,
		"ownership_conc":                     sm.OwnershipConc,
		"ownership_entropy":                  sm.OwnershipEntropy,
		"hotspot_density":                    sm.HotspotDensity,
		"hotspot_score":                      sm.HotspotScore,
		"file_volatility":                    sm.FileVolatility,
		"churn_factor":                       sm.ChurnFactor,
		"complexity_per_feat":                sm.ComplexityPerFeat,
		"change_complexity_proxy":            sm.ChangeComplexityProxy,
		"cross_cutting_change_risk":          sm.CrossCuttingChangeRisk,
		"technical_debt_usd":                 sm.TechnicalDebtUSD,
		"touched_test_file_ratio":            sm.TouchedTestFileRatio,
		"fix_to_feature_ratio":               sm.FixToFeatureRatio,
		"revert_rate":                        sm.RevertRate,
		"refactoring_ratio":                  sm.RefactoringRatio,
		"test_to_source_ratio":               sm.TestToSourceRatio,
		"scope_isolation":                    sm.ScopeIsolation,
		"cross_cutting_pct":                  sm.CrossCuttingPct,
		"api_surface_change":                 sm.APISurfaceChange,
	}

	snapshotJSON, _ := json.Marshal(snapshot)

	langName := "English"
	example1 := "First-to-last commit span is short"
	example2 := "Release commits are broadly distributed"
	if lang == i18n.LangRU {
		langName = "Russian"
		example1 = "Короткий интервал между первым и последним коммитом"
		example2 = "Коммиты релиза распределены широко"
	} else if lang == i18n.LangZH {
		langName = "Chinese"
		example1 = "首尾提交间隔较短"
		example2 = "发布提交分布较广"
	}

	prompt := fmt.Sprintf(`You are a software engineering metrics analyst. Given these release metrics (JSON), provide a brief 3-8 word assessment for each metric in %s. Be specific and actionable, not generic.

Metrics:
%s

Return ONLY valid JSON (no markdown, no backticks) mapping each metric key to its assessment string. Example:
{"release_commit_span_hours": "%s", "release_contribution_concentration": "%s"}`, langName, string(snapshotJSON), example1, example2)

	text, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI metric interpretations failed: %w", err)
	}

	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var interpretations map[string]string
	if err := json.Unmarshal([]byte(text), &interpretations); err != nil {
		return nil, fmt.Errorf("parse AI metric interpretations: %w", err)
	}

	return interpretations, nil
}

func GenerateMetricsNarrative(ctx context.Context, sm SummaryMetrics, client Client, lang i18n.Lang) (string, error) {
	if client == nil {
		return "", fmt.Errorf("no AI client")
	}

	snapshot := map[string]any{
		"total_commits":                      sm.TotalCommits,
		"total_authors":                      sm.TotalAuthors,
		"breaking_changes":                   sm.BreakingChanges,
		"release_commit_span_hours":          sm.ReleaseCommitSpanHours,
		"release_age_hours":                  sm.ReleaseAgeHours,
		"batch_factor":                       sm.BatchFactor,
		"release_risk_score":                 sm.ReleaseRiskScore,
		"release_contribution_concentration": sm.ReleaseContributionConcentration,
		"ownership_conc":                     sm.OwnershipConc,
		"ownership_entropy":                  sm.OwnershipEntropy,
		"hotspot_density":                    sm.HotspotDensity,
		"hotspot_score":                      sm.HotspotScore,
		"file_volatility":                    sm.FileVolatility,
		"churn_factor":                       sm.ChurnFactor,
		"complexity_per_feat":                sm.ComplexityPerFeat,
		"change_complexity_proxy":            sm.ChangeComplexityProxy,
		"cross_cutting_change_risk":          sm.CrossCuttingChangeRisk,
		"technical_debt_usd":                 sm.TechnicalDebtUSD,
		"touched_test_file_ratio":            sm.TouchedTestFileRatio,
		"fix_to_feature_ratio":               sm.FixToFeatureRatio,
		"revert_rate":                        sm.RevertRate,
		"refactoring_ratio":                  sm.RefactoringRatio,
		"test_to_source_ratio":               sm.TestToSourceRatio,
		"scope_isolation":                    sm.ScopeIsolation,
		"cross_cutting_pct":                  sm.CrossCuttingPct,
		"api_surface_change":                 sm.APISurfaceChange,
		"files_touched":                      sm.FilesTouched,
		"lines_added":                        sm.LinesAdded,
		"lines_deleted":                      sm.LinesDeleted,
		"net_lines":                          sm.NetLines,
		"commits_per_day":                    sm.CommitsPerDay,
	}

	snapshotJSON, _ := json.Marshal(snapshot)

	var prompt string
	switch lang {
	case i18n.LangRU:
		prompt = fmt.Sprintf(`Ты — опытный инженерный аналитик. Проанализируй метрики релиза ниже (JSON) и напиши развёрнутую интерпретацию на русском языке (3-5 предложений).

Включи:
- Общую оценку здоровья кодовой базы и рисков релиза
- Анализ hotspot score, change complexity proxy и cross-cutting change risk
- Оценку технического долга (в долларах) и доли затронутых тестовых файлов
- Анализ энтропии владения кодом (ownership entropy) и release contribution concentration
- Анализ интервала между первым и последним коммитом, возраста изменений и batch factor
- Практические рекомендации для следующего релиза

Не называй touched_test_file_ratio покрытием кода, cross_cutting_change_risk графом зависимостей,
release_contribution_concentration bus factor, а release_commit_span_hours lead/cycle time.

Метрики:
%s

Напиши только текст интерпретации, без заголовка и без markdown.`, string(snapshotJSON))
	case i18n.LangZH:
		prompt = fmt.Sprintf(`你是一位资深工程分析师。请分析以下发布指标（JSON），用中文撰写详细的解读（3-5句话）。

包含：
- 代码库整体健康度和发布风险评估
- 热点分数、变更复杂度代理和跨模块变更风险分析
- 技术债务（美元）和触及测试文件比例评估
- 代码所有权熵和发布贡献集中度分析
- 首尾提交间隔、变更年龄和批量因子分析
- 对下一个版本的实用建议

不要把 touched_test_file_ratio 称为代码覆盖率，不要把 cross_cutting_change_risk 称为依赖图，
不要把 release_contribution_concentration 称为巴士因子，也不要把 release_commit_span_hours 称为提前期或周期时间。

指标：
%s

只写解读文本，不要标题，不要markdown。`, string(snapshotJSON))
	default:
		prompt = fmt.Sprintf(`You are a senior engineering analyst. Analyze the release metrics below (JSON) and write a detailed interpretation in English (3-5 sentences).

Include:
- Overall codebase health and release risk assessment
- Analysis of hotspot score, change complexity proxy, and cross-cutting change risk
- Assessment of technical debt (in USD) and touched test file ratio
- Analysis of ownership entropy and release contribution concentration
- Analysis of first-to-last release commit span, release age, and batch factor
- Practical recommendations for the next release

Do not describe touched_test_file_ratio as code coverage, cross_cutting_change_risk as a dependency graph,
release_contribution_concentration as bus factor, or release_commit_span_hours as lead/cycle time.

Metrics:
%s

Write only the interpretation text, no heading, no markdown.`, string(snapshotJSON))
	}

	text, err := client.StreamGenerate(ctx, prompt, nil)
	if err != nil {
		return "", fmt.Errorf("AI metrics narrative failed: %w", err)
	}

	return strings.TrimSpace(text), nil
}
