package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/curate"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/render"
)

func runCurate(args []string, defaultRepo string) {
	fs := flag.NewFlagSet("curate", flag.ExitOnError)
	var (
		from      string
		to        string
		cfgPath   string
		repoPath  string
		outPath   string
		filter    string
		quiet     bool
		bumpLevel string
	)
	fs.StringVar(&from, "from", "", "Start ref (default: latest tag)")
	fs.StringVar(&to, "to", "HEAD", "End ref")
	fs.StringVar(&cfgPath, "config", defaultConfigPath(), "Config file path (or PATCHLOG_CONFIG)")
	fs.StringVar(&repoPath, "repo", defaultRepo, "Path to git repository")
	fs.StringVar(&outPath, "out", "", "Output file (default: stdout)")
	fs.StringVar(&filter, "filter", "", "Monorepo path filter")
	fs.BoolVar(&quiet, "quiet", false, "Suppress banner and spinner output")
	fs.StringVar(&bumpLevel, "bump", "", "Bump version: patch, minor, major, auto")
	fs.Parse(args)

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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fetcher := &gitlog.Fetcher{RepoPath: repoPath, Filter: filter}
	rangeFrom := resolveRange(ctx, fetcher, from, false)

	thresholds := thresholdsFromConfig(cfg)

	if !quiet {
		printBanner()
	}
	spinner := console.NewSpinner("Fetching commits...")
	if !quiet {
		spinner.Start()
	}
	report, parsedCommits, err := buildReport(ctx, fetcher, rangeFrom, to, cfg, true, thresholds)
	if !quiet {
		spinner.Stop(err == nil)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	report.ShowAuthor = cfg.Author.Show
	useEmoji := true
	if cfg.Changelog.Emojis != nil {
		useEmoji = *cfg.Changelog.Emojis
	}
	report.Emojis = useEmoji

	if cfg.Repo != "" {
		report.Repo = cfg.Repo
		report.CommitURLTemplate = cfg.Links.Commit
		report.IssueURLTemplate = cfg.Links.Issue
		if rangeFrom != "" && rangeFrom != "--root" {
			compareURL := cfg.Links.Compare
			if strings.Contains(compareURL, "%s") {
				report.CompareURL = fmt.Sprintf(compareURL, cfg.Repo, rangeFrom, to)
			}
		}
	}

	fileCache := cache.New(filepath.Join(repoPath, ".patchlog", "cache"), cache.WithDeferredWrites(true))

	if cfg.Jira.BaseURL != "" && cfg.Jira.APIToken != "" {
		enrichWithJira(ctx, &report, cfg.Jira, fileCache)
	}

	var bumpPlan *bump.Plan
	if bumpLevel == "auto" {
		lvl := determineBumpLevel(report, quiet)
		bumpPlan, err = bump.CreatePlan(repoPath, lvl, cfg.Bump.Files, cfg.Bump.AutoDetect)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error planning automatic version bump: %v\n", err)
			os.Exit(1)
		}
		report.Version = bumpPlan.NewVersion
	} else if bumpLevel != "" {
		lvl, err := bump.ParseLevel(bumpLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}
		bumpPlan, err = bump.CreatePlan(repoPath, lvl, cfg.Bump.Files, cfg.Bump.AutoDetect)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error planning version bump: %v\n", err)
			os.Exit(1)
		}
		report.Version = bumpPlan.NewVersion
	}

	if to == "HEAD" && report.Version == "" {
		report.Version = "Unreleased"
	}

	filtered, confirmed, err := curate.Run(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Curate error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back to non-interactive output\n")
		output, _ := render.Markdown(report)
		if outPath != "" {
			if writeErr := atomicWriteFile(outPath, output, 0644); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing fallback output: %v\n", writeErr)
			}
		} else {
			os.Stdout.Write(output)
		}
		os.Exit(1)
	}

	if !confirmed {
		fmt.Fprintf(os.Stderr, "Curate cancelled — no output generated\n")
		os.Exit(1)
	}

	output, err := render.Markdown(filtered)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering: %v\n", err)
		os.Exit(1)
	}

	if cfg.Gate.Enabled && cfg.Gate.MinConventionalRatio > 0 {
		reportMetrics := metrics.ComputeReportMetrics(report, parsedCommits)
		if reportMetrics.ConventionalRatio < cfg.Gate.MinConventionalRatio {
			fmt.Fprintf(os.Stderr, "Release gate failed: conventional_ratio %.0f%% < %.0f%%\n",
				reportMetrics.ConventionalRatio*100, cfg.Gate.MinConventionalRatio*100)
			os.Exit(3)
		}
	}

	applyState := &ApplyState{}
	if bumpPlan != nil {
		if err := applyState.Run(ctx, "version bump", func(context.Context) error { return bumpPlan.Apply() }); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying version bump: %v\n", err)
			os.Exit(1)
		}
		if !quiet {
			console.Step(fmt.Sprintf("Bumped version → %s (%s)", bumpPlan.NewVersion, strings.Join(bumpPlan.ChangedFiles(), ", ")))
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
}
