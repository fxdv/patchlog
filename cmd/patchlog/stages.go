package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/categorize"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/drift"
	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/infer"
	"github.com/fxdv/patchlog/pkg/jira"
	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/semantic"
	"github.com/fxdv/patchlog/pkg/theme"
)

// stageInfer runs AI commit type inference on uncategorized commits.
func stageInfer(ctx context.Context, state *PipelineState) {
	if state.DryRun {
		return
	}
	if !(state.hasFlag("infer") || state.Cfg.Infer.Enabled) {
		return
	}
	aiClient := newAIClientOrWarn(state.Cfg, "inference", state.Quiet)
	if aiClient == nil {
		return
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Inferring commit types with AI...")
		spinner.Start()
		defer func() { spinner.Stop(false) }()
	}
	oldOthers := countOthers(state.ParsedCommits)
	state.ParsedCommits = infer.InferTypes(ctx, state.ParsedCommits, aiClient, infer.Options{
		Threshold: state.Cfg.Infer.Threshold,
		Types:     state.Cfg.Infer.Types,
	})
	newOthers := countOthers(state.ParsedCommits)
	if newOthers < oldOthers {
		state.Report = categorize.ByType(state.ParsedCommits, state.Cfg.Sections)
		if state.To == "HEAD" {
			state.Report.Version = "Unreleased"
		} else {
			state.Report.Version = state.To
		}
		if !state.Quiet {
			console.Step(fmt.Sprintf("Inferred types for %d commit(s)", oldOthers-newOthers))
		}
	}
}

// stageJira enriches the report with Jira ticket details.
func stageJira(ctx context.Context, state *PipelineState) int {
	if state.DryRun {
		return 0
	}
	if state.Cfg.Jira.BaseURL == "" || state.Cfg.Jira.APIToken == "" {
		return 0
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Enriching Jira issues...")
		spinner.Start()
		defer func() { spinner.Stop(false) }()
	}
	fileCache := state.Cache
	if fileCache == nil {
		fileCache = cache.New(filepath.Join(state.Repo, ".patchlog", "cache"), cache.WithEnabled(false))
	}
	count := enrichWithJira(ctx, &state.Report, state.Cfg.Jira, fileCache)
	if !state.Quiet {
		fmt.Fprintf(os.Stderr, "\r  \033[32m✓\033[0m Enriching Jira issues...    \n")
	}
	return count
}

// stageDeps detects dependency changes and fetches upstream changelogs.
func stageDeps(ctx context.Context, state *PipelineState) {
	if !(state.hasFlag("deps") || state.Cfg.Deps.Enabled) {
		return
	}
	if state.RangeFrom == "" || state.RangeFrom == "--root" {
		return
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Detecting dependency changes...")
		spinner.Start()
		defer func() { spinner.Stop(false) }()
	}
	depsConfig := state.Cfg
	if state.DryRun {
		depsConfig.Deps.FetchUpstream = false
		depsConfig.Deps.GitHubReleases = false
	}
	depChanges := detectDependencyChanges(ctx, state.Fetcher, state.RangeFrom, state.To, depsConfig)
	if len(depChanges) > 0 {
		state.Report.Dependencies = depChanges
		if !state.Quiet {
			fmt.Fprintf(os.Stderr, "\r  \033[32m✓\033[0m Detecting dependency changes...    \n")
			console.Step(fmt.Sprintf("Found %d dependency change(s)", len(depChanges)))
		}
	}
}

// stageSemantic generates AI semantic diff summaries.
func stageSemantic(ctx context.Context, state *PipelineState) map[string]string {
	if state.DryRun {
		return nil
	}
	if !(state.hasFlag("semantic") || state.Cfg.Semantic.Enabled) {
		return nil
	}
	aiClient := newAIClientOrWarn(state.Cfg, "semantic", state.Quiet)
	if aiClient == nil {
		return nil
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Generating semantic diff summaries...")
		spinner.Start()
		defer func() { spinner.Stop(false) }()
	}
	diffs := collectCommitDiffs(ctx, state.Fetcher, state.ParsedCommits, state.Cfg.AI.ExcludeFiles, state.Cfg.Semantic.StripGenerated)
	result := semantic.SummarizeSections(ctx, state.Report, diffs, aiClient, semantic.Options{
		Aggregate:    state.Cfg.Semantic.Aggregate,
		MaxDiffChars: state.Cfg.Semantic.MaxDiffChars,
	})
	if !state.Quiet && len(result) > 0 {
		fmt.Fprintf(os.Stderr, "\r  \033[32m✓\033[0m Generating semantic diff summaries...    \n")
	}
	return result
}

// stageDrift compares planned Jira tickets vs delivered.
func stageDrift(ctx context.Context, state *PipelineState) *drift.Report {
	if state.DryRun {
		return nil
	}
	if !(state.hasFlag("drift") || state.Cfg.Drift.Enabled) {
		return nil
	}
	if state.Cfg.Jira.BaseURL == "" || state.Cfg.Jira.APIToken == "" {
		fmt.Fprintf(os.Stderr, "Warning: drift detection requires Jira config, skipping\n")
		return nil
	}
	fixVer := state.Cfg.Drift.FixVersion
	if fixVer == "" {
		fixVer = stripVersionPrefix(state.To)
	}
	deliveredKeys := collectJiraKeys(&state.Report)
	jiraClient := jira.NewClient(jira.Config{
		BaseURL:    state.Cfg.Jira.BaseURL,
		Email:      state.Cfg.Jira.Email,
		APIToken:   state.Cfg.Jira.APIToken,
		ProjectKey: state.Cfg.Jira.ProjectKey,
	})
	report, err := drift.Analyze(ctx, jiraClient, state.Cfg.Jira.ProjectKey, fixVer, deliveredKeys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: drift analysis failed: %v\n", err)
		return nil
	}
	return report
}

// stageEnhance runs AI description enhancement and summary generation.
func stageEnhance(ctx context.Context, state *PipelineState, format string, langVal i18n.Lang) (string, []byte) {
	if state.DryRun {
		return state.AISummary, state.Output
	}
	if !state.hasFlag("ai-enhance") || format == "json" {
		return "", nil
	}
	aiClient := newAIClientOrWarn(state.Cfg, "enhancement", state.Quiet)
	if aiClient == nil {
		return state.AISummary, state.Output
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Enhancing descriptions with AI...")
		spinner.Start()
	}
	var enhanceErr error
	state.Report, enhanceErr = ai.EnhanceReport(ctx, state.Report, aiClient)
	if enhanceErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", enhanceErr)
	}
	if !state.Quiet {
		fmt.Fprintf(os.Stderr, "\r  \033[32m✓\033[0m Enhancing descriptions with AI...    \n")
	}

	// Re-render after enhancement
	var output []byte
	switch format {
	case "markdown":
		output, _ = render.Markdown(state.Report)
	case "prose":
		output, _ = renderProseLang(ctx, state.Report, state.Tone, state.Cfg, state.Quiet, langVal)
	}

	// Generate AI summary
	var aiSummary string
	if !state.Quiet {
		console.Step("Generating AI summary...")
	}
	state.ComputeMetrics()
	sm := buildSummaryMetrics(state.ReportMetrics, state.CodeStats)
	summary, err := ai.GenerateSummaryLang(ctx, state.Report, aiClient, sm, langVal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if summary != "" {
		aiSummary = summary
		enhanced := append([]byte("> "+summary+"\n\n"), output...)
		output = enhanced
	}

	return aiSummary, output
}

// stageTheme runs AI thematic grouping.
func stageTheme(ctx context.Context, state *PipelineState, format string, aiSummary string) (*theme.ThemedReport, []byte) {
	if state.DryRun {
		return nil, nil
	}
	if !(state.hasFlag("theme") || state.Cfg.Theme.Enabled) || format != "markdown" {
		return nil, nil
	}
	aiClient := newAIClientOrWarn(state.Cfg, "themes", state.Quiet)
	if aiClient == nil {
		return nil, nil
	}
	if !state.Quiet {
		spinner := console.NewSpinner("Grouping into themes with AI...")
		spinner.Start()
		defer func() { spinner.Stop(true) }()
	}
	themeOpts := theme.Options{
		MinThemes:        state.Cfg.Theme.MinThemes,
		MaxThemes:        state.Cfg.Theme.MaxThemes,
		IncludeNarrative: state.Cfg.Theme.IncludeNarrative,
	}
	capturedThemedReport, err := theme.GroupCommits(ctx, state.Report, aiClient, themeOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	themedOutput, err := theme.Markdown(capturedThemedReport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering themed output: %v\n", err)
		return nil, nil
	}
	if aiSummary != "" {
		themedOutput = append([]byte("> "+aiSummary+"\n\n"), themedOutput...)
	}
	return &capturedThemedReport, themedOutput
}

// hasFlag is a placeholder for flag checking. In the current architecture,
// flags are local to main(). This method allows stage functions to be
// called conditionally. The actual flag values are set on the PipelineState
// by the caller before invoking stages.
func (s *PipelineState) hasFlag(name string) bool {
	if s.Flags == nil {
		return false
	}
	return s.Flags[name]
}

// stripVersionPrefix removes common version tag prefixes (kexp/, v).
func stripVersionPrefix(s string) string {
	for _, prefix := range []string{"kexp/", "v"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			return s[len(prefix):]
		}
	}
	return s
}

// _ imports to prevent unused warnings during incremental refactoring
