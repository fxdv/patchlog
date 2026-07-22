package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/postmortem"
)

func runPostmortem(args []string, defaultRepo string) {
	fs := flag.NewFlagSet("postmortem", flag.ExitOnError)
	var (
		tag      string
		days     int
		repoPath string
		quiet    bool
	)
	fs.StringVar(&tag, "tag", "", "Release tag to analyze (default: latest tag)")
	fs.IntVar(&days, "days", 7, "Days after release to analyze")
	fs.StringVar(&repoPath, "repo", defaultRepo, "Path to git repository")
	fs.BoolVar(&quiet, "quiet", false, "Suppress progress output")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: patchlog postmortem [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Analyze post-release stability: rollbacks, hotfixes, regressions.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  patchlog postmortem\n")
		fmt.Fprintf(os.Stderr, "  patchlog postmortem --tag v1.0.0 --days 14\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fetcher := &gitlog.Fetcher{RepoPath: repoPath}

	if tag == "" {
		latest, err := fetcher.LatestTag(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: no tags found in %s: %v\n", repoPath, err)
			fmt.Fprintf(os.Stderr, "Hint: specify a tag with --tag <name>\n")
			os.Exit(1)
		}
		tag = latest
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "  Analyzing post-release stability for %s (%d days)...\n", tag, days)
	}

	report, err := postmortem.Analyze(ctx, fetcher, tag, days)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing postmortem for tag %s: %v\n", tag, err)
		os.Exit(1)
	}

	if report == nil || (len(report.Rollbacks) == 0 && len(report.Hotfixes) == 0 && len(report.Regressions) == 0) {
		fmt.Fprintln(os.Stderr, "No post-release issues found. Release looks stable.")
		os.Exit(0)
	}

	output := postmortem.FormatTerminal(report)
	fmt.Print(output)
}
