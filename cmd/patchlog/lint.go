package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/lint"
)

func cmdLint(ctx context.Context, cfg config.Config, repo, from, to string, useAI bool) {
	fetcher := &gitlog.Fetcher{RepoPath: repo}

	rawCommits, err := fetcher.FetchLog(ctx, from, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching log: %v\n", err)
		os.Exit(1)
	}

	var commits []commit.Commit
	for _, rc := range rawCommits {
		commits = append(commits, commit.Parse(rc))
	}

	result := lint.Lint(commits)

	if useAI {
		if aiClient := newAIClientOrWarn(cfg, "lint", false); aiClient != nil {
			result.Issues = lint.AISuggest(ctx, result.Issues, aiClient)
		}
	}

	fmt.Print(lint.FormatResult(result))

	if result.HasErrors() {
		os.Exit(1)
	}
}

func runLint(args []string) {
	lintFlags := flag.NewFlagSet("lint", flag.ExitOnError)
	var (
		from    string
		to      string
		repo    string
		cfgPath string
		useAI   bool
	)
	lintFlags.StringVar(&from, "from", "", "Start ref (default: latest tag)")
	lintFlags.StringVar(&to, "to", "HEAD", "End ref")
	lintFlags.StringVar(&repo, "repo", ".", "Path to git repository")
	lintFlags.StringVar(&cfgPath, "config", defaultConfigPath(), "Config file path (or PATCHLOG_CONFIG)")
	lintFlags.BoolVar(&useAI, "ai", false, "Use AI to suggest improved commit messages")
	lintFlags.Parse(args)

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
	fetcher := &gitlog.Fetcher{RepoPath: repo}

	rangeFrom := from
	if rangeFrom == "" {
		tag, err := fetcher.LatestTag(ctx)
		if err == nil {
			rangeFrom = tag
		}
	}

	cmdLint(ctx, cfg, repo, rangeFrom, to, useAI)
}
