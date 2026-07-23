// Package gitlog provides git history fetching via subprocess calls,
// including commit logs, diff stats, changed files, and tag queries.
package gitlog

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/commit"
)

// Fetcher wraps git subprocess calls for a specific repository path.
type Fetcher struct {
	RepoPath string
	Filter   string
}

// DiffStat holds per-commit diff statistics.
type DiffStat struct {
	FilesChanged int
	Insertions   int
	Deletions    int
	Files        []string
}

func (f *Fetcher) FetchLog(ctx context.Context, from, to string) ([]commit.RawCommit, error) {
	args := []string{"log", "-z", "--reverse", "--no-merges", "--format=%H%x00%an%x00%ae%x00%at%x00%B"}
	switch {
	case from == "--root":
		args = append(args, "--root", to)
	case from != "":
		args = append(args, from+".."+to)
	default:
		args = append(args, to)
	}
	if f.Filter != "" {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	parts := strings.Split(string(out), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	var commits []commit.RawCommit
	for i := 0; i+4 < len(parts); i += 5 {
		ts, _ := strconv.ParseInt(parts[i+3], 10, 64)
		commits = append(commits, commit.RawCommit{
			Hash:      parts[i],
			Author:    parts[i+1],
			Email:     parts[i+2],
			Timestamp: time.Unix(ts, 0),
			Message:   parts[i+4],
		})
	}
	return commits, nil
}

func (f *Fetcher) ChangedFiles(ctx context.Context, hash string) ([]string, error) {
	args := []string{"diff-tree", "--no-commit-id", "--name-only", "-r", hash}
	if f.Filter != "" {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}

func (f *Fetcher) GetDiffStat(ctx context.Context, hash string) (*DiffStat, error) {
	args := []string{"show", "--numstat", "--format=", hash}
	if f.Filter != "" {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show --numstat: %w", err)
	}
	return parseDiffStat(string(out)), nil
}

// BatchDiffStats fetches diff stats for multiple commits in a single git call.
// Returns a map of commit hash → DiffStat.
func (f *Fetcher) BatchDiffStats(ctx context.Context, from, to string) (map[string]*DiffStat, error) {
	args := []string{"log", "--numstat", "--format=COMMIT%x00%H", "--no-merges", "--reverse"}
	switch {
	case from == "--root" || from == "":
		args = append(args, "--root", to)
	default:
		args = append(args, from+".."+to)
	}
	if f.Filter != "" {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log --numstat: %w", err)
	}

	results := make(map[string]*DiffStat)
	var currentHash string
	var currentStat *DiffStat

	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "COMMIT\x00") {
			if currentHash != "" && currentStat != nil {
				results[currentHash] = currentStat
			}
			currentHash = strings.TrimPrefix(line, "COMMIT\x00")
			currentStat = &DiffStat{}
			continue
		}
		if currentStat == nil {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		currentStat.FilesChanged++
		if fields[0] != "-" {
			n, _ := strconv.Atoi(fields[0])
			currentStat.Insertions += n
		}
		if fields[1] != "-" {
			n, _ := strconv.Atoi(fields[1])
			currentStat.Deletions += n
		}
		currentStat.Files = append(currentStat.Files, fields[2])
	}
	if currentHash != "" && currentStat != nil {
		results[currentHash] = currentStat
	}

	return results, nil
}

func parseDiffStat(output string) *DiffStat {
	stat := &DiffStat{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		stat.FilesChanged++
		if fields[0] != "-" {
			n, _ := strconv.Atoi(fields[0])
			stat.Insertions += n
		}
		if fields[1] != "-" {
			n, _ := strconv.Atoi(fields[1])
			stat.Deletions += n
		}
		stat.Files = append(stat.Files, fields[2])
	}
	return stat
}

const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func (f *Fetcher) resolveDiffArgs(from, to string) []string {
	switch {
	case from == "--root" || from == "":
		return []string{"diff", emptyTreeHash, to}
	default:
		return []string{"diff", from, to}
	}
}

func (f *Fetcher) ChangedFilesInRange(ctx context.Context, from, to string) ([]string, error) {
	args := f.resolveDiffArgs(from, to)
	args = append(args, "--name-only")
	if f.Filter != "" {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only: %w", err)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) == 1 && files[0] == "" {
		return nil, nil
	}
	return files, nil
}

func (f *Fetcher) RangeDiff(ctx context.Context, from, to string, paths ...string) (string, error) {
	args := f.resolveDiffArgs(from, to)
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	if f.Filter != "" && len(paths) == 0 {
		args = append(args, "--", f.Filter)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (f *Fetcher) TagDate(ctx context.Context, tag string) (time.Time, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%at", tag)
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("git log tag date: %w", err)
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing tag date: %w", err)
	}
	return time.Unix(ts, 0), nil
}

func (f *Fetcher) LatestTag(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(ctx, "git", "tag", "--sort=-version:refname")
		cmd.Dir = f.RepoPath
		out, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("no tags found: %w", err)
		}
		tags := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(tags) == 0 || tags[0] == "" {
			return "", fmt.Errorf("no tags found")
		}
		return tags[0], nil
	}
	return strings.TrimSpace(string(out)), nil
}

// LatestStableTag returns the highest reachable tag matching the protected
// release identity prefix + MAJOR.MINOR.PATCH. Nightly, prerelease, and
// deployment-marker tags cannot accidentally steer version planning.
func (f *Fetcher) LatestStableTag(ctx context.Context, prefix string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "tag", "--merged", "HEAD", "--list", "--sort=-version:refname")
	cmd.Dir = f.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list stable release tags: %w", err)
	}
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `[0-9]+\.[0-9]+\.[0-9]+$`)
	for _, tag := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pattern.MatchString(tag) {
			return tag, nil
		}
	}
	return "", fmt.Errorf("no stable release tags found with prefix %q", prefix)
}
