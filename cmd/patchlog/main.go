package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/drift"
	"github.com/fxdv/patchlog/pkg/gamify"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/gittag"
	"github.com/fxdv/patchlog/pkg/htmlreport"
	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/semantic"
	"github.com/fxdv/patchlog/pkg/theme"
	"github.com/fxdv/patchlog/pkg/trends"
)

var version = "dev"

// Scoring formula constants
const (
	// Release risk score weights
	riskBreakingWeight  = 20.0
	riskBreakingMax     = 40.0
	riskHotspotWeight   = 0.3
	riskHotspotMax      = 30.0
	riskOwnershipWeight = 0.3
	riskOwnershipMax    = 30.0

	// Change complexity proxy divisors
	changeComplexityLinesDivisor = 50.0
	changeComplexityFilesDivisor = 5.0
	changeComplexityChurnDivisor = 10.0

	// Cross-cutting change risk weights
	crossCuttingChangeWeight = 0.4
	crossCuttingScopeWeight  = 35.0
	crossCuttingAPIWeight    = 3.0
	crossCuttingAPIMax       = 25.0

	// Technical debt costs (USD)
	debtChurnCost         = 0.50
	debtRefactorCost      = 25.0
	debtFixCost           = 15.0
	debtComplexityDivisor = 20.0

	// Hotspot score weights
	hotspotDensityWeight    = 0.35
	hotspotChurnWeight      = 8.0
	hotspotChurnMax         = 35.0
	hotspotVolatilityWeight = 12.0
	hotspotVolatilityMax    = 30.0
)

func main() {
	opts, args, err := parseCLI(os.Args[1:], os.Stderr)
	if err != nil {
		return
	}
	releaseMode := opts.releaseMode
	from, to, cfgPath, repo := opts.from, opts.to, opts.cfgPath, opts.repo
	outPath, format, filter, tone := opts.outPath, opts.format, opts.filter, opts.tone
	bumpLevel, langFlag := opts.bumpLevel, opts.lang
	first, dryRun, showVer := opts.first, opts.dryRun, opts.showVer
	classifyOn, publish, review := opts.classifyOn, opts.publish, opts.review
	confluenceFlag, changelogFlag, metricsFlag := opts.confluence, opts.changelog, opts.metrics
	aiEnhance, quiet, noCache := opts.aiEnhance, opts.quiet, opts.noCache
	themeFlag, tagFlag, pushFlag, forceFlag := opts.theme, opts.tag, opts.push, opts.force
	trendsFlag, gateFlag, depsFlag := opts.trends, opts.gate, opts.deps
	requireConv := opts.requireConv
	inferFlag, semanticFlag, driftFlag := opts.infer, opts.semantic, opts.drift
	gamifyFlag, htmlFlag, labsFlag := opts.gamify, opts.html, opts.labs

	if pushFlag && !tagFlag {
		fmt.Fprintf(os.Stderr, "Error: --push requires --tag\n")
		os.Exit(2)
	}
	if tagFlag && bumpLevel == "" {
		fmt.Fprintf(os.Stderr, "Error: --tag requires --bump\n")
		os.Exit(2)
	}
	if gamifyFlag && !labsFlag {
		fmt.Fprintf(os.Stderr, "Error: --gamify is experimental and requires --labs\n")
		os.Exit(2)
	}
	if !releaseMode && (bumpLevel != "" || tagFlag || pushFlag || forceFlag || publish || confluenceFlag || changelogFlag || trendsFlag) {
		fmt.Fprintf(os.Stderr, "Error: release mutations require the focused 'patchlog release' subcommand\n")
		os.Exit(2)
	}

	if releaseMode && len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected release argument %q\n", args[0])
		os.Exit(2)
	}
	if len(args) > 0 {
		switch args[0] {
		case "init":
			cmdInit()
			return
		case "lint":
			runLint(args[1:])
			return
		case "audit":
			runAudit(args[1:])
			return
		case "multi":
			runMultiRepo(args[1:])
			return
		case "recover":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: patchlog recover <json-file>\n")
				os.Exit(2)
			}
			cmdRecover(args[1])
			return
		case "cache":
			if len(args) < 2 || args[1] != "clear" {
				fmt.Fprintf(os.Stderr, "Usage: patchlog cache clear\n")
				os.Exit(2)
			}
			cmdCacheClear(repo)
			return
		case "trends":
			runTrends(args[1:], repo)
			return
		case "curate":
			runCurate(args[1:], repo)
			return
		case "postmortem":
			runPostmortem(args[1:], repo)
			return
		}
	}

	if showVer {
		fmt.Printf("patchlog %s\n", version)
		return
	}

	toneVal, err := ai.ParseTone(tone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if err := validateFormat(format); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(2)
	}
	resolveEnvVars(&cfg)
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(2)
	}
	if cfg.Changelog.Accumulate && !releaseMode {
		fmt.Fprintf(os.Stderr, "Config error: changelog.accumulate requires the focused 'patchlog release' subcommand\n")
		os.Exit(2)
	}

	langStr := langFlag
	if langStr == "" {
		langStr = cfg.Language.Default
	}
	langVals, err := i18n.ParseLangs(langStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	langVal := langVals[0]

	fileCache := cache.New(
		filepath.Join(repo, ".patchlog", "cache"),
		cache.WithEnabled(!noCache && !dryRun),
		cache.WithDeferredWrites(true),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	startTime := time.Now()

	fetcher := &gitlog.Fetcher{RepoPath: repo, Filter: filter}

	rangeFrom := resolveRange(ctx, fetcher, from, first)

	if !quiet {
		printBanner()
	}

	thresholds := thresholdsFromConfig(cfg)

	var spinner *console.Spinner
	if !quiet {
		spinner = console.NewSpinner("Fetching commits...")
		spinner.Start()
	}
	report, parsedCommits, err := buildReport(ctx, fetcher, rangeFrom, to, cfg, classifyOn, thresholds)
	if err != nil {
		if !quiet && spinner != nil {
			spinner.Stop(false)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !quiet && spinner != nil {
		spinner.Stop(true)
	}

	// Initialize pipeline state for stage-based execution
	state := &PipelineState{
		Ctx:           ctx,
		Cfg:           cfg,
		Fetcher:       fetcher,
		Repo:          repo,
		Quiet:         quiet,
		DryRun:        dryRun,
		Cache:         fileCache,
		Tone:          toneVal,
		RangeFrom:     rangeFrom,
		To:            to,
		Report:        report,
		ParsedCommits: parsedCommits,
		Flags: map[string]bool{
			"infer":      inferFlag,
			"deps":       depsFlag,
			"semantic":   semanticFlag,
			"drift":      driftFlag,
			"ai-enhance": aiEnhance,
			"theme":      themeFlag,
		},
	}

	// Stage: AI commit type inference
	if inferFlag || cfg.Infer.Enabled {
		stageInfer(ctx, state)
		parsedCommits = state.ParsedCommits
		report = state.Report
	}

	// Stage: Jira enrichment
	var jiraCount int
	state.Report = report
	jiraCount = stageJira(ctx, state)
	report = state.Report

	report.ShowAuthor = cfg.Author.Show

	if cfg.Language.TranslateHeadings && (len(langVals) > 1 || langVal != i18n.LangEN) {
		for i := range report.Sections {
			report.Sections[i].Heading = i18n.BilingualHeading(langVals, report.Sections[i].Type)
		}
	}

	useEmoji := true
	if cfg.Changelog.Emojis != nil {
		useEmoji = *cfg.Changelog.Emojis
	}
	report.Emojis = useEmoji

	if cfg.Repo != "" && cfg.Links.Compare != "" && rangeFrom != "" && rangeFrom != "--root" {
		compareURL := cfg.Links.Compare
		if strings.Contains(compareURL, "%s") {
			report.CompareURL = fmt.Sprintf(compareURL, cfg.Repo, rangeFrom, to)
		} else {
			report.CompareURL = compareURL
		}
	}

	if cfg.Links.Commit != "" {
		report.CommitURLTemplate = cfg.Links.Commit
	}
	if cfg.Links.Issue != "" {
		report.IssueURLTemplate = cfg.Links.Issue
	}
	if cfg.Repo != "" {
		report.Repo = cfg.Repo
	}

	// Stage: dependency changelog detection
	if depsFlag || cfg.Deps.Enabled {
		state.Report = report
		stageDeps(ctx, state)
		report = state.Report
	}

	// Stage: semantic diff summaries
	var semanticSummaries map[string]string
	if semanticFlag || cfg.Semantic.Enabled {
		state.Report = report
		semanticSummaries = stageSemantic(ctx, state)
	}

	// Stage: plan-vs-actual drift detection
	var driftReport *drift.Report
	if driftFlag || cfg.Drift.Enabled {
		state.Report = report
		driftReport = stageDrift(ctx, state)
	}

	var bumpLevelStr string
	var bumpPlan *bump.Plan
	if bumpLevel == "auto" {
		lvl := determineBumpLevel(report, quiet)
		bumpPlan, err = bump.CreatePlan(repo, lvl, cfg.Bump.Files, cfg.Bump.AutoDetect)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error planning automatic version bump: %v\n", err)
			os.Exit(1)
		}
		bumpLevelStr = lvl.String()
		if !quiet {
			console.Step(fmt.Sprintf("Planned auto bump (%s) → %s (%s)", bumpLevelStr, bumpPlan.NewVersion, strings.Join(bumpPlan.ChangedFiles(), ", ")))
		}
		report.Version = bumpPlan.NewVersion
	} else if bumpLevel != "" {
		lvl, err := bump.ParseLevel(bumpLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}
		bumpPlan, err = bump.CreatePlan(repo, lvl, cfg.Bump.Files, cfg.Bump.AutoDetect)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error planning version bump: %v\n", err)
			os.Exit(1)
		}
		bumpLevelStr = lvl.String()
		if !quiet {
			console.Step(fmt.Sprintf("Planned version bump → %s (%s)", bumpPlan.NewVersion, strings.Join(bumpPlan.ChangedFiles(), ", ")))
		}
		report.Version = bumpPlan.NewVersion
	}

	var tagResult *gittag.Result
	var tagName string
	var tagOpts gittag.Options
	if tagFlag && report.Version != "" {
		prefix := ""
		if prevTag, err := fetcher.LatestTag(ctx); err == nil {
			prefix = gittag.DetectPrefix(prevTag)
		}
		tagName = prefix + report.Version

		tagOpts = gittag.Options{
			Tag:    true,
			Push:   pushFlag,
			Force:  forceFlag,
			DryRun: dryRun,
		}
	}
	var htmlFile string
	if htmlFlag {
		htmlFile = fmt.Sprintf("report-%s.html", strings.ReplaceAll(report.Version, "/", "_"))
	}

	var output []byte
	var aiSummary string
	var themedReport *theme.ThemedReport

	var reportMetrics metrics.ReportMetrics
	var codeStats metrics.CodeStats
	metricsComputed := false
	computeMetrics := func() {
		if metricsComputed {
			return
		}
		reportMetrics = metrics.ComputeReportMetrics(report, parsedCommits)
		codeStats = metrics.ComputeCodeStats(ctx, fetcher, parsedCommits)
		metricsComputed = true
	}

	output, err = (DefaultReportOutputRenderer{}).Render(ctx, report, OutputRenderOptions{
		Format: format, Tone: toneVal, Config: cfg, Quiet: quiet, DryRun: dryRun, Lang: langVal,
	})
	if format == "prose" && err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v (using fallback)\n", err)
		err = nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering: %v\n", err)
		os.Exit(1)
	}

	// Stage: AI enhancement + summary
	if aiEnhance && format != "json" {
		state.Report = report
		state.Output = output
		aiSummary, output = stageEnhance(ctx, state, format, langVal)
		report = state.Report
	}

	// Stage: AI theme grouping
	if (themeFlag || cfg.Theme.Enabled) && format == "markdown" {
		state.Report = report
		tr, themedOutput := stageTheme(ctx, state, format, aiSummary)
		if tr != nil {
			themedReport = tr
			output = themedOutput
		}
	}

	if metricsFlag && format != "json" {
		computeMetrics()
		metricsMd := metrics.FormatMetricsMarkdown(reportMetrics, codeStats)
		output = append(output, []byte("\n"+metricsMd)...)
	}

	if len(semanticSummaries) > 0 && format != "json" {
		semMd := semantic.FormatMarkdown(semanticSummaries)
		output = append(output, []byte("\n"+semMd)...)
	}

	if driftReport != nil && format != "json" {
		driftMd := drift.FormatMarkdown(driftReport)
		output = append(output, []byte("\n"+driftMd)...)
	}

	if gamifyFlag {
		computeMetrics()
		allSnaps, _ := trends.Load(repo, cfg.Trends.Count)
		gamifyResults := gamify.Compute(parsedCommits, reportMetrics, allSnaps, gamify.Options{ShowLevels: true})
		if len(gamifyResults) > 0 && format != "json" {
			gamifyMd := gamify.FormatMarkdown(gamifyResults)
			output = append(output, []byte("\n"+gamifyMd)...)
		}
	}

	// Review and policy evaluation are the final read-only checks. No version
	// file, Git ref, output file, cache, changelog, or remote service has been
	// mutated before this point.
	if review {
		if dryRun {
			if !quiet {
				console.Step("[dry-run] Interactive review omitted to preserve filesystem immutability")
			}
		} else {
			reviewedOutput, confirmed, reviewErr := cmdReview(output)
			if reviewErr != nil {
				fmt.Fprintf(os.Stderr, "Review error: %v\n", reviewErr)
				os.Exit(1)
			}
			if !confirmed {
				return
			}
			output = reviewedOutput
		}
	}

	if gateFlag || cfg.Gate.Enabled {
		computeMetrics()
		gMinConv := cfg.Gate.MinConventionalRatio
		if requireConv > 0 {
			gMinConv = requireConv
		}
		var gateFailures []string
		if gMinConv > 0 && reportMetrics.ConventionalRatio < gMinConv {
			gateFailures = append(gateFailures, fmt.Sprintf("conventional_ratio %.0f%% < %.0f%%", reportMetrics.ConventionalRatio*100, gMinConv*100))
		}
		if len(gateFailures) > 0 {
			fmt.Fprintf(os.Stderr, "\n  %s\n", console.RedText("─── Release Gate FAILED ───"))
			for _, failure := range gateFailures {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n", failure)
			}
			_, _ = os.Stdout.Write(output)
			os.Exit(3)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "\n  %s\n", console.GreenText("─── Release Gate PASSED ───"))
		}
	}

	// Construct one immutable plan after review and gate evaluation. This is the
	// final side-effect-free boundary: every requested local and remote action
	// must pass deterministic preflight before dry-run returns or Apply begins.
	var releasePlan *ReleasePlan
	if releaseMode {
		releasePlan, err = NewReleasePlan(ctx, ReleasePlanRequest{
			Repo:          repo,
			Bump:          bumpPlan,
			TagName:       tagName,
			TagOptions:    tagOpts,
			Publish:       publish,
			Confluence:    confluenceFlag,
			Changelog:     changelogFlag || cfg.Changelog.Accumulate,
			Trends:        trendsFlag,
			HTMLPath:      htmlFile,
			OutputPath:    outPath,
			Configuration: cfg,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Release plan error: %v\n", err)
			os.Exit(2)
		}
	}

	if dryRun {
		displayDryRun(ctx, fetcher, rangeFrom, to, quiet)
		if bumpPlan != nil && !quiet {
			console.Step(fmt.Sprintf("[dry-run] Would bump %s → %s in: %s", bumpPlan.CurrentVersion, bumpPlan.NewVersion, strings.Join(bumpPlan.ChangedFiles(), ", ")))
		}
		if tagFlag && !quiet {
			console.Step(fmt.Sprintf("[dry-run] Would commit only: %s", strings.Join(bumpPlan.ChangedFiles(), ", ")))
			console.Step(fmt.Sprintf("[dry-run] Would create tag: %s", tagName))
			if pushFlag {
				console.Step("[dry-run] Would atomically push branch and tag")
			}
		}
		if publish && !quiet {
			console.Step("[dry-run] Would publish release draft to provider")
		}
		if confluenceFlag && !quiet {
			console.Step("[dry-run] Would publish to Confluence")
		}
		if changelogFlag || cfg.Changelog.Accumulate {
			if !quiet {
				console.Step("[dry-run] Would accumulate changelog")
			}
		}
		if htmlFlag && !quiet {
			console.Step("[dry-run] Would write an HTML report")
		}
		_, _ = os.Stdout.Write(output)
		return
	}

	publishOperation := ProviderPublishOperation(nil)
	if publish {
		publishOperation = func(ctx context.Context, ref RemoteReleaseRef) (string, error) {
			if !quiet {
				spinner = console.NewSpinner("Publishing release draft...")
				spinner.Start()
			}
			url, publishErr := runPublish(ctx, ref, cfg, string(output))
			if !quiet && spinner != nil {
				spinner.Stop(publishErr == nil)
			}
			return url, publishErr
		}
	}
	coreResult := &CoreReleaseApplyResult{State: &ApplyState{}}
	if releaseMode {
		coreResult, err = ApplyCoreRelease(ctx, CoreReleaseApplyRequest{
			Plan: releasePlan, PublishProvider: publishOperation,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error applying release plan: %v\n", err)
			os.Exit(1)
		}
	}
	applyState := coreResult.State
	tagResult = coreResult.Tag
	publishURL := coreResult.PublishURL
	if bumpPlan != nil && !quiet {
		console.Step(fmt.Sprintf("Bumped version → %s (%s)", bumpPlan.NewVersion, strings.Join(bumpPlan.ChangedFiles(), ", ")))
	}
	if tagResult != nil && !quiet {
		if len(tagResult.Files) > 0 && tagResult.Commit != "" {
			console.Step(fmt.Sprintf("Committed only planned files: %s (%s)", strings.Join(tagResult.Files, ", "), tagResult.Commit))
		}
		console.Step(fmt.Sprintf("Tagged: %s", tagName))
		if tagResult.Pushed {
			console.Step(fmt.Sprintf("Atomically pushed origin/%s and tag %s", tagResult.Branch, tagName))
		}
	}

	if htmlFlag {
		computeMetrics()
		sm := buildSummaryMetrics(reportMetrics, codeStats)
		allSnaps, _ := trends.Load(repo, cfg.Trends.Count)
		htmlData := htmlreport.ReportData{
			Version:   report.Version,
			Date:      report.Date,
			Report:    report,
			Metrics:   reportMetrics,
			CodeStats: codeStats,
			Summary:   sm,
			Snapshots: allSnaps,
			AISummary: aiSummary,
			Commits:   parsedCommits,
			Fetcher:   fetcher,
			Ctx:       ctx,
			Labs:      labsFlag,
		}
		htmlOut := htmlreport.Generate(htmlData)
		if err := applyState.Run(ctx, "HTML report write", func(context.Context) error {
			return atomicWriteFile(htmlFile, []byte(htmlOut), 0644)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML report: %v\n", err)
			os.Exit(1)
		}
		if !quiet {
			console.Step(fmt.Sprintf("HTML report: %s", htmlFile))
		}
	}

	var confluenceURL string
	if confluenceFlag {
		if dryRun {
			if !quiet {
				console.Step("[dry-run] Would publish to Confluence")
			}
		} else {
			if !quiet {
				spinner = console.NewSpinner("Publishing to Confluence...")
				spinner.Start()
			}
			computeMetrics()
			sm := buildSummaryMetrics(reportMetrics, codeStats)

			var metricInterpretations map[string]string
			var metricsNarrative string
			if aiEnhance {
				if aiClient := newAIClientOrWarn(cfg, "metrics", quiet); aiClient != nil {
					metricInterpretations, _ = ai.GenerateMetricInterpretations(ctx, sm, aiClient, langVal)
					metricsNarrative, _ = ai.GenerateMetricsNarrative(ctx, sm, aiClient, langVal)
				}
			}

			var url string
			var updated bool
			if err := applyState.Run(ctx, "Confluence publish", func(ctx context.Context) error {
				var publishErr error
				url, updated, publishErr = runConfluencePublish(ctx, report, cfg, fileCache, aiSummary, sm, buildCommandString(), metricInterpretations, metricsNarrative, themedReport, langVal)
				return publishErr
			}); err != nil {
				if !quiet && spinner != nil {
					spinner.Stop(false)
				}
				fmt.Fprintf(os.Stderr, "Error publishing to Confluence: %v\n", err)
				os.Exit(1)
			}
			if !quiet && spinner != nil {
				spinner.Stop(true)
			}
			confluenceURL = url
			if !quiet {
				if updated {
					console.Step(fmt.Sprintf("Updated Confluence page: %s", url))
				} else {
					console.Step(fmt.Sprintf("Created Confluence page: %s", url))
				}
			}
		}
	}

	var changelogURL string
	if changelogFlag || cfg.Changelog.Accumulate {
		if dryRun {
			if !quiet {
				console.Step("[dry-run] Would accumulate changelog")
			}
		} else {
			if !quiet {
				spinner = console.NewSpinner(fmt.Sprintf("Accumulating changelog (%s)...", cfg.Changelog.Destination))
				spinner.Start()
			}
			var url string
			if err := applyState.Run(ctx, "changelog update", func(ctx context.Context) error {
				var changelogErr error
				url, changelogErr = runChangelogAccumulate(ctx, report, cfg, string(output), repo, fileCache)
				return changelogErr
			}); err != nil {
				if !quiet && spinner != nil {
					spinner.Stop(false)
				}
				fmt.Fprintf(os.Stderr, "Error accumulating changelog: %v\n", err)
				os.Exit(1)
			}
			if !quiet && spinner != nil {
				spinner.Stop(true)
			}
			changelogURL = url
			if !quiet && url != "" {
				console.Step(fmt.Sprintf("Changelog updated: %s", url))
			}
		}
	}

	if releaseMode && cfg.Trends.Store && report.Version != "" && report.Version != "Unreleased" {
		computeMetrics()
		sm := buildSummaryMetrics(reportMetrics, codeStats)
		snap := snapshotFromMetrics(report.Version, reportMetrics, codeStats, sm)
		if err := applyState.Run(ctx, "trend snapshot write", func(context.Context) error {
			return trends.Store(repo, snap)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to store trend snapshot: %v\n", err)
		}
	}

	var trendsURL string
	if trendsFlag && confluenceFlag {
		if !quiet {
			spinner = console.NewSpinner("Publishing trends dashboard to Confluence...")
			spinner.Start()
		}
		allSnapshots, err := trends.Load(repo, cfg.Trends.Count)
		if err != nil {
			if !quiet && spinner != nil {
				spinner.Stop(false)
			}
			fmt.Fprintf(os.Stderr, "Warning: failed to load trend snapshots: %v\n", err)
		} else {
			var url string
			var updated bool
			if err := applyState.Run(ctx, "trends publish", func(ctx context.Context) error {
				var publishErr error
				url, updated, publishErr = runTrendsConfluencePublish(ctx, allSnapshots, cfg, fileCache)
				return publishErr
			}); err != nil {
				if !quiet && spinner != nil {
					spinner.Stop(false)
				}
				fmt.Fprintf(os.Stderr, "Warning: failed to publish trends to Confluence: %v\n", err)
			} else {
				if !quiet && spinner != nil {
					spinner.Stop(true)
				}
				trendsURL = url
				if !quiet {
					if updated {
						console.Step(fmt.Sprintf("Updated trends dashboard: %s", url))
					} else {
						console.Step(fmt.Sprintf("Created trends dashboard: %s", url))
					}
				}
			}
		}
	}

	if outPath != "" {
		if err := applyState.Run(ctx, "output write", func(context.Context) error {
			return atomicWriteFile(outPath, output, 0644)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		if !quiet {
			console.Step(fmt.Sprintf("Wrote %s", outPath))
		}
	} else {
		os.Stdout.Write(output)
	}
	if err := fileCache.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to flush cache after apply: %v\n", err)
	}

	totalCommits := report.ItemCount()

	if !quiet {
		summary := console.Summary{
			Version:       report.Version,
			Commits:       totalCommits,
			JiraTickets:   jiraCount,
			BumpLevel:     bumpLevelStr,
			Published:     publishURL != "",
			PublishURL:    publishURL,
			Confluence:    confluenceURL != "",
			ConfluenceURL: confluenceURL,
			Changelog:     changelogURL != "",
			ChangelogURL:  changelogURL,
			Trends:        trendsURL != "",
			TrendsURL:     trendsURL,
			DepsCount:     len(report.Dependencies),
			Duration:      time.Since(startTime),
		}
		if tagResult != nil {
			summary.Tag = tagResult.Tag
			summary.Pushed = tagResult.Pushed
		}
		console.PrintSummary(summary)
	}
}

func validateFormat(format string) error {
	switch format {
	case "markdown", "json", "prose":
		return nil
	default:
		return fmt.Errorf("unsupported format %q (use markdown, json, or prose)", format)
	}
}

func countOthers(commits []commit.Commit) int {
	n := 0
	for _, c := range commits {
		if c.Type == "other" {
			n++
		}
	}
	return n
}

func resolveRange(ctx context.Context, fetcher *gitlog.Fetcher, from string, first bool) string {
	if from != "" {
		return from
	}
	if first {
		return "--root"
	}
	tag, err := fetcher.LatestTag(ctx)
	if err == nil {
		return tag
	}
	return ""
}

func displayDryRun(ctx context.Context, fetcher *gitlog.Fetcher, rangeFrom, to string, quiet bool) {
	if !quiet {
		printBanner()
	}
	commits, err := fetcher.FetchLog(ctx, rangeFrom, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching log: %v\n", err)
		os.Exit(1)
	}
	displayFrom := rangeFrom
	if displayFrom == "" || displayFrom == "--root" {
		displayFrom = "<first commit>"
	}
	fmt.Fprintf(os.Stderr, "Range: %s..%s\n", displayFrom, to)
	fmt.Fprintf(os.Stderr, "Commits: %d\n", len(commits))
}

func cmdCacheClear(repo string) {
	dir := filepath.Join(repo, ".patchlog", "cache")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No cache directory at %s\n", dir)
		return
	}
	c := cache.New(dir)
	if err := c.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Cache cleared: %s\n", dir)
}

func buildCommandString() string {
	parts := make([]string, 0, len(os.Args))
	for i, arg := range os.Args {
		if i == 0 {
			parts = append(parts, filepath.Base(arg))
			continue
		}
		if strings.ContainsAny(arg, " \t") {
			parts = append(parts, fmt.Sprintf("%q", arg))
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}
