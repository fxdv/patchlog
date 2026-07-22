package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fxdv/patchlog/internal/ignore"
	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/categorize"
	"github.com/fxdv/patchlog/pkg/classify"
	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/deps"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/jira"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/significance"
	"github.com/fxdv/patchlog/pkg/trends"
)

func buildReport(ctx context.Context, fetcher *gitlog.Fetcher, rangeFrom, to string, cfg config.Config, doClassify bool, thresholds classify.Thresholds) (render.Report, []commit.Commit, error) {
	rawCommits, err := fetcher.FetchLog(ctx, rangeFrom, to)
	if err != nil {
		return render.Report{}, nil, fmt.Errorf("fetching log: %w", err)
	}

	ignoreRe := ignore.Compile(cfg.Ignore)
	if ignoreRe == nil && len(cfg.Ignore) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: some ignore patterns are invalid and were skipped\n")
	}

	var parsed []commit.Commit
	for _, rc := range rawCommits {
		if ignoreRe != nil && ignoreRe.MatchString(rc.Message) {
			continue
		}
		c := commit.Parse(rc)

		if doClassify {
			stat, ferr := fetcher.GetDiffStat(ctx, c.Hash)
			if ferr == nil {
				c.ChangedFiles = stat.FilesChanged
				diff := classify.DiffInfo{
					ChangedFiles: stat.FilesChanged,
					Insertions:   stat.Insertions,
					Deletions:    stat.Deletions,
					Files:        stat.Files,
					FileAnalysis: classify.AnalyzeFiles(stat.Files),
				}
				result := classify.ClassifyWithDiff(c, diff, thresholds)
				c.Significance = result.Level.String()
				c.ClassReason = result.Reason
			} else {
				files, _ := fetcher.ChangedFiles(ctx, c.Hash)
				nfiles := len(files)
				c.ChangedFiles = nfiles
				result := classify.ClassifyWithThresholds(c, nfiles, thresholds)
				c.Significance = result.Level.String()
				c.ClassReason = result.Reason
			}
		}

		parsed = append(parsed, c)
	}

	report := categorize.ByType(parsed, cfg.Sections)

	if to == "HEAD" {
		report.Version = "Unreleased"
	} else {
		report.Version = to
	}

	return report, parsed, nil
}

func enrichWithJira(ctx context.Context, report *render.Report, cfg config.JiraConfig, fileCache *cache.Cache) int {
	client := jira.NewClient(jira.Config{
		BaseURL:        cfg.BaseURL,
		Email:          cfg.Email,
		APIToken:       cfg.APIToken,
		ProjectKey:     cfg.ProjectKey,
		MaxConcurrency: cfg.MaxConcurrency,
	})
	client.SetCache(fileCache)
	if !client.Configured() {
		return 0
	}

	allKeys := collectJiraKeys(report)
	filtered := client.FilterKeys(allKeys)
	issues := client.EnrichKeys(ctx, filtered)

	applyJiraIssues(report, issues)
	return len(issues)
}

func collectJiraKeys(report *render.Report) []string {
	seen := make(map[string]bool)
	var keys []string

	report.ForEachItem(func(item *render.Item) {
		for _, k := range item.JiraKeys {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	})

	return keys
}

func applyJiraIssues(report *render.Report, issues map[string]*jira.Issue) {
	report.ForEachItem(func(item *render.Item) {
		var jiraIssues []*render.JiraInfo
		for _, k := range item.JiraKeys {
			if issue, ok := issues[k]; ok {
				jiraIssues = append(jiraIssues, &render.JiraInfo{
					Key:         issue.Key,
					Summary:     issue.Summary,
					Priority:    issue.Priority,
					Status:      issue.Status,
					URL:         issue.URL,
					Labels:      issue.Labels,
					Type:        issue.Type,
					EpicKey:     issue.EpicKey,
					FixVersions: issue.FixVersions,
					Components:  issue.Components,
					Description: issue.Description,
					Assignee:    issue.Assignee,
				})
			}
		}
		item.JiraIssues = jiraIssues
	})
}

func determineBumpLevel(report render.Report, quiet bool) bump.Level {
	maxLevel := significance.Skip
	counts := make(map[string]int)

	report.ForEachItem(func(item *render.Item) {
		l, _ := significance.ParseLevel(item.Significance)
		if l > maxLevel {
			maxLevel = l
		}
		if item.Significance != "" && item.Significance != "skip" {
			counts[item.Significance]++
		}
	})

	if maxLevel == significance.Skip {
		return bump.Patch
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "  ")
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := counts[lvl]; ok && c > 0 {
				colorFn := console.RedText
				switch lvl {
				case "minor":
					colorFn = console.YellowText
				case "patch":
					colorFn = console.GreenText
				}
				fmt.Fprintf(os.Stderr, "%s  ", colorFn(fmt.Sprintf("%dx %s", c, lvl)))
			}
		}
		fmt.Fprint(os.Stderr, "\n")
	}

	switch maxLevel {
	case significance.Major:
		return bump.Major
	case significance.Minor:
		return bump.Minor
	default:
		return bump.Patch
	}
}

func detectDependencyChanges(ctx context.Context, fetcher *gitlog.Fetcher, rangeFrom, to string, cfg config.Config) []render.DependencyChange {
	changedFiles, err := fetcher.ChangedFilesInRange(ctx, rangeFrom, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get changed files for deps: %v\n", err)
		return nil
	}

	diffs := make(map[string]string)
	for _, f := range changedFiles {
		if !deps.IsManifestFile(f) {
			continue
		}
		diff, err := fetcher.RangeDiff(ctx, rangeFrom, to, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to diff %s: %v\n", f, err)
			continue
		}
		if diff != "" {
			diffs[f] = diff
		}
	}

	if len(diffs) == 0 {
		return nil
	}

	changes := deps.DetectAll(diffs)
	if len(changes) == 0 {
		return nil
	}

	deps.SetChangelogURLs(changes)

	if cfg.Deps.FetchUpstream {
		opts := deps.FetchOptions{
			MaxDependencies: cfg.Deps.MaxDependencies,
			GitHubReleases:  cfg.Deps.GitHubReleases,
			NPMRegistry:     cfg.Deps.Registries["npm"],
			CratesRegistry:  cfg.Deps.Registries["crates"],
			PyPIRegistry:    cfg.Deps.Registries["pypi"],
		}
		changes = deps.FetchChangelogs(ctx, changes, opts)
	}

	result := make([]render.DependencyChange, len(changes))
	for i, c := range changes {
		result[i] = render.DependencyChange{
			Name:         c.Name,
			OldVersion:   c.OldVersion,
			NewVersion:   c.NewVersion,
			Ecosystem:    string(c.Ecosystem),
			Manifest:     c.Manifest,
			Changelog:    c.Changelog,
			ChangelogURL: c.ChangelogURL,
		}
	}
	return result
}

func collectCommitDiffs(ctx context.Context, fetcher *gitlog.Fetcher, commits []commit.Commit, excludeFiles []string, stripGenerated bool) map[string]string {
	diffs := make(map[string]string)
	patterns := append([]string(nil), excludeFiles...)
	if stripGenerated {
		patterns = append(patterns, "**/generated/**", "*.generated.*", "*.gen.*")
	}
	for _, c := range commits {
		if c.Hash == "" {
			continue
		}
		files, err := fetcher.ChangedFiles(ctx, c.Hash)
		if err != nil {
			continue
		}
		included := files[:0]
		for _, file := range files {
			if !ai.PathExcluded(file, patterns) {
				included = append(included, file)
			}
		}
		if len(included) == 0 {
			continue
		}
		diff, err := fetcher.RangeDiff(ctx, c.Hash+"^", c.Hash, included...)
		if err != nil {
			continue
		}
		if diff != "" {
			diffs[c.Hash] = diff
		}
	}
	return diffs
}

func computeSnapshot(ctx context.Context, fetcher *gitlog.Fetcher, rangeFrom, to string, cfg config.Config) (trends.Snapshot, error) {
	thresholds := thresholdsFromConfig(cfg)

	report, parsedCommits, err := buildReport(ctx, fetcher, rangeFrom, to, cfg, true, thresholds)
	if err != nil {
		return trends.Snapshot{}, err
	}

	rm := metrics.ComputeReportMetrics(report, parsedCommits)
	cs := metrics.ComputeCodeStats(ctx, fetcher, parsedCommits)
	sm := buildSummaryMetrics(rm, cs)

	return snapshotFromMetrics("Unreleased", rm, cs, sm), nil
}
