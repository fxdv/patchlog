package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/gamify"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/trends"
)

func runTrends(args []string, defaultRepo string) {
	fs := flag.NewFlagSet("trends", flag.ExitOnError)
	var (
		count        int
		jsonOut      bool
		fromVer      string
		repoPath     string
		cfgPath      string
		confFlag     bool
		quiet        bool
		noUnreleased bool
		labs         bool
	)
	fs.IntVar(&count, "count", -1, "Number of releases to show (default: trends.count)")
	fs.BoolVar(&jsonOut, "json", false, "Output as JSON")
	fs.StringVar(&fromVer, "from-version", "", "Show trends from this version onwards")
	fs.StringVar(&repoPath, "repo", defaultRepo, "Path to git repository")
	fs.StringVar(&cfgPath, "config", defaultConfigPath(), "Config file path (or PATCHLOG_CONFIG)")
	fs.BoolVar(&confFlag, "confluence", false, "Publish trend dashboard to a separate Confluence page")
	fs.BoolVar(&quiet, "quiet", false, "Suppress spinner output")
	fs.BoolVar(&noUnreleased, "no-unreleased", false, "Exclude the Unreleased column")
	fs.BoolVar(&labs, "labs", false, "Enable experimental contributor gamification")
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
	if count < 0 {
		count = cfg.Trends.Count
	}

	snapshots, err := trends.Load(repoPath, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading trends from %s: %v\n", filepath.Join(repoPath, ".patchlog", "trends"), err)
		os.Exit(1)
	}

	if fromVer != "" {
		filtered := snapshots[:0]
		found := false
		for _, s := range snapshots {
			if s.Version == fromVer {
				found = true
			}
			if found {
				filtered = append(filtered, s)
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Warning: version %q not found in trend data, showing all releases\n", fromVer)
		} else {
			snapshots = filtered
		}
	}

	if count > 0 && len(snapshots) > count {
		snapshots = snapshots[len(snapshots)-count:]
	}

	if !noUnreleased {
		fetcher := &gitlog.Fetcher{RepoPath: repoPath}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		lastTag, tagErr := fetcher.LatestTag(ctx)
		if tagErr == nil {
			unreleased, err := computeSnapshot(ctx, fetcher, lastTag, "HEAD", cfg)
			if err == nil && unreleased.TotalCommits > 0 {
				snapshots = append(snapshots, unreleased)
			}
		}
	}

	if len(snapshots) == 0 {
		fmt.Fprintln(os.Stderr, "No trend data found.")
		fmt.Fprintln(os.Stderr, "Run a release with a version (e.g. patchlog release --bump auto) to start collecting snapshots.")
		os.Exit(0)
	}

	if confFlag {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		fileCache := cache.New(filepath.Join(repoPath, ".patchlog", "cache"))

		var gamificationHTML string
		var aiClient interface {
			Generate(context.Context, string) (string, error)
			StreamGenerate(context.Context, string, func(string)) (string, error)
		}
		if labs {
			aiClient = newAIClientOrWarn(cfg, "gamification", quiet)
		}

		var spinner *console.Spinner
		if !quiet {
			spinner = console.NewSpinner("Publishing trends to Confluence...")
			spinner.Start()
		}

		if aiClient != nil {
			results := buildGamifyFromSnapshots(snapshots)
			if len(results) > 0 {
				achResults := gamify.GenerateAchievements(ctx, results, snapshots[:max(0, len(snapshots)-1)], aiClient, cfg.Language.Default)
				gamificationHTML = gamify.FormatConfluence(achResults)
			}
		}

		url, updated, err := runTrendsConfluencePublishWithGamification(ctx, snapshots, cfg, fileCache, gamificationHTML)
		if err != nil {
			if !quiet && spinner != nil {
				spinner.Stop(false)
			}
			fmt.Fprintf(os.Stderr, "Error publishing trends to Confluence: %v\n", err)
			os.Exit(1)
		}
		if !quiet && spinner != nil {
			spinner.Stop(true)
		}
		if !quiet {
			if updated {
				console.Step(fmt.Sprintf("Updated Confluence page: %s", url))
			} else {
				console.Step(fmt.Sprintf("Created Confluence page: %s", url))
			}
		}
	}

	if jsonOut {
		data, err := json.MarshalIndent(snapshots, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(data)
		fmt.Println()
		return
	}

	output := trends.RenderTerminal(snapshots)
	fmt.Print(output)
}

func buildGamifyFromSnapshots(snapshots []trends.Snapshot) []gamify.ContributorResult {
	contributorCommits := make(map[string]int)
	contributorReleases := make(map[string]int)
	for _, snap := range snapshots {
		for _, c := range snap.TopContributors {
			contributorCommits[c.Name] += c.Commits
			contributorReleases[c.Name]++
		}
	}
	if len(contributorCommits) == 0 {
		return nil
	}

	type kv struct {
		Name    string
		Commits int
	}
	var sorted []kv
	for name, commits := range contributorCommits {
		sorted = append(sorted, kv{name, commits})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Commits > sorted[j].Commits
	})

	var results []gamify.ContributorResult
	for _, s := range sorted {
		tc := s.Commits
		releases := contributorReleases[s.Name]
		var level int
		var levelName string
		switch {
		case tc >= 100:
			level, levelName = 5, "Legend"
		case tc >= 51:
			level, levelName = 4, "Veteran"
		case tc >= 21:
			level, levelName = 3, "Regular"
		case tc >= 6:
			level, levelName = 2, "Contributor"
		default:
			level, levelName = 1, "Newcomer"
		}
		result := gamify.ContributorResult{
			Name:      s.Name,
			Commits:   s.Commits,
			Level:     level,
			LevelName: levelName,
		}
		if s.Commits == sorted[0].Commits {
			result.Badges = append(result.Badges, gamify.Badge{Emoji: "🏆", Name: "Release Hero", Reason: "Most commits across releases"})
		}
		if releases >= 3 {
			result.Badges = append(result.Badges, gamify.Badge{Emoji: "🔥", Name: "Streak Keeper", Reason: fmt.Sprintf("%d releases", releases)})
		}
		results = append(results, result)
	}
	return results
}
