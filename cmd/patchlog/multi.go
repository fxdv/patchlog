package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fxdv/patchlog/pkg/classify"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/multirepo"
)

func runMultiRepo(args []string) {
	multiFlags := flag.NewFlagSet("multi", flag.ExitOnError)
	var (
		from    string
		to      string
		cfgPath string
		filter  string
		format  string
		outPath string
	)
	multiFlags.StringVar(&from, "from", "", "Start ref (default: latest tag)")
	multiFlags.StringVar(&to, "to", "HEAD", "End ref")
	multiFlags.StringVar(&cfgPath, "config", defaultConfigPath(), "Config file path (or PATCHLOG_CONFIG)")
	multiFlags.StringVar(&filter, "filter", "", "Monorepo path filter")
	multiFlags.StringVar(&format, "format", "markdown", "Output format: markdown, json")
	multiFlags.StringVar(&outPath, "out", "", "Output file (default: stdout)")
	multiFlags.Parse(args)

	repoPaths := multiFlags.Args()
	if len(repoPaths) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: patchlog multi <repo-path> [repo-path...] [flags]\n")
		os.Exit(2)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error in %s: %v\n", cfgPath, err)
		os.Exit(2)
	}
	resolveEnvVars(&cfg)
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	thresholds := classify.DefaultThresholds()
	if cfg.Classify.LargeFeatureFiles > 0 {
		thresholds.LargeFeatureFiles = cfg.Classify.LargeFeatureFiles
	}
	if cfg.Classify.LargeFixFiles > 0 {
		thresholds.LargeFixFiles = cfg.Classify.LargeFixFiles
	}

	opts := multirepo.AggregateOptions{
		From:       from,
		To:         to,
		Filter:     filter,
		Classify:   true,
		Thresholds: thresholds,
	}

	results := multirepo.Aggregate(ctx, repoPaths, cfg, opts)

	useEmoji := true
	if cfg.Changelog.Emojis != nil {
		useEmoji = *cfg.Changelog.Emojis
	}

	output := multirepo.FormatAggregateMarkdown(results, cfg.Author.Show, useEmoji)

	if outPath != "" {
		if err := atomicWriteFile(outPath, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s\n", outPath)
	} else {
		os.Stdout.Write(output)
	}

	var hasErrors bool
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ %s: %v\n", r.Name, r.Error)
			hasErrors = true
		}
	}
	if hasErrors {
		os.Exit(1)
	}
}
