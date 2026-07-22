package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fxdv/patchlog/pkg/audit"
	"github.com/fxdv/patchlog/pkg/gitlog"
)

func runAudit(args []string) {
	auditFlags := flag.NewFlagSet("audit", flag.ExitOnError)
	var (
		from          string
		to            string
		repo          string
		changelogFile string
	)
	auditFlags.StringVar(&from, "from", "", "Start ref (default: latest tag)")
	auditFlags.StringVar(&to, "to", "HEAD", "End ref")
	auditFlags.StringVar(&repo, "repo", ".", "Path to git repository")
	auditFlags.StringVar(&changelogFile, "changelog", "", "Path to changelog file (default: CHANGELOG.md in repo)")
	auditFlags.Parse(args)

	if changelogFile == "" {
		changelogFile = filepath.Join(repo, "CHANGELOG.md")
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

	result, err := audit.Audit(ctx, fetcher, changelogFile, rangeFrom, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(audit.FormatResult(result))

	if result.HasIssues() {
		os.Exit(1)
	}
}
