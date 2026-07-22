// Package multirepo aggregates changelogs from multiple repositories into a single document.
package multirepo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/internal/ignore"
	"github.com/fxdv/patchlog/pkg/categorize"
	"github.com/fxdv/patchlog/pkg/classify"
	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/render"
)

type RepoResult struct {
	Name    string
	Path    string
	Version string
	Report  render.Report
	Commits []commit.Commit
	Error   error
}

type AggregateOptions struct {
	From       string
	To         string
	Filter     string
	Classify   bool
	Thresholds classify.Thresholds
}

func Aggregate(ctx context.Context, repoPaths []string, cfg config.Config, opts AggregateOptions) []RepoResult {
	var results []RepoResult

	for _, path := range repoPaths {
		result := buildRepoResult(ctx, path, cfg, opts)
		results = append(results, result)
	}

	return results
}

func buildRepoResult(ctx context.Context, path string, cfg config.Config, opts AggregateOptions) RepoResult {
	fetcher := &gitlog.Fetcher{RepoPath: path, Filter: opts.Filter}

	rangeFrom := opts.From
	if rangeFrom == "" {
		tag, err := fetcher.LatestTag(ctx)
		if err == nil {
			rangeFrom = tag
		}
	}

	rawCommits, err := fetcher.FetchLog(ctx, rangeFrom, opts.To)
	if err != nil {
		return RepoResult{
			Name:  repoName(path),
			Path:  path,
			Error: fmt.Errorf("fetch log: %w", err),
		}
	}

	ignoreRe := ignore.Compile(cfg.Ignore)
	var parsed []commit.Commit
	for _, rc := range rawCommits {
		if ignoreRe != nil && ignoreRe.MatchString(rc.Message) {
			continue
		}
		c := commit.Parse(rc)

		if opts.Classify {
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
				result := classify.ClassifyWithDiff(c, diff, opts.Thresholds)
				c.Significance = result.Level.String()
				c.ClassReason = result.Reason
			} else {
				files, _ := fetcher.ChangedFiles(ctx, c.Hash)
				result := classify.ClassifyWithThresholds(c, len(files), opts.Thresholds)
				c.Significance = result.Level.String()
				c.ClassReason = result.Reason
			}
		}

		parsed = append(parsed, c)
	}

	report := categorize.ByType(parsed, cfg.Sections)
	version := opts.To
	if version == "HEAD" {
		version = "Unreleased"
	}
	report.Version = version

	return RepoResult{
		Name:    repoName(path),
		Path:    path,
		Version: version,
		Report:  report,
		Commits: parsed,
	}
}

func repoName(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	return parts[len(parts)-1]
}

func FormatAggregateMarkdown(results []RepoResult, showAuthor bool, useEmoji bool) []byte {
	var buf strings.Builder

	buf.WriteString("# Aggregated Changelog\n\n")

	var errors []string
	for _, r := range results {
		if r.Error != nil {
			errors = append(errors, fmt.Sprintf("- **%s**: %s", r.Name, r.Error.Error()))
			continue
		}
	}

	if len(errors) > 0 {
		buf.WriteString("## ⚠ Errors\n\n")
		for _, e := range errors {
			buf.WriteString(e + "\n")
		}
		buf.WriteString("\n")
	}

	validResults := make([]RepoResult, 0)
	for _, r := range results {
		if r.Error == nil {
			validResults = append(validResults, r)
		}
	}

	sort.Slice(validResults, func(i, j int) bool {
		return validResults[i].Name < validResults[j].Name
	})

	for _, r := range validResults {
		r.Report.ShowAuthor = showAuthor
		r.Report.Emojis = useEmoji

		data, err := render.Markdown(r.Report)
		if err != nil {
			continue
		}

		header := fmt.Sprintf("# %s\n\n", r.Name)
		buf.WriteString(header)
		buf.Write(data)
		buf.WriteString("\n---\n\n")
	}

	return []byte(buf.String())
}
