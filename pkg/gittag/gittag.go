// Package gittag handles git tag creation, commit staging, and remote push for release automation.
package gittag

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Manager struct {
	RepoPath string
}

type Options struct {
	Tag    bool
	Push   bool
	Force  bool
	DryRun bool
}

type Result struct {
	Files  []string
	Commit string
	Tag    string
	Branch string
	Pushed bool
}

var digitRe = regexp.MustCompile(`\d`)

func DetectPrefix(tag string) string {
	idx := digitRe.FindStringIndex(tag)
	if idx == nil {
		return tag
	}
	return tag[:idx[0]]
}

// WorktreeChanges returns paths only for clean-tree safety checks. Release
// files always come from bump.Plan.ChangedFiles, never from this status data.
func (m *Manager) WorktreeChanges(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = m.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	var files []string
	records := bytes.Split(out, []byte{0})
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}
		status := string(record[:2])
		file := string(record[3:])
		if file != "" {
			files = append(files, file)
		}
		if strings.ContainsAny(status, "RC") && i+1 < len(records) {
			i++ // -z rename/copy records include the original path next.
		}
	}
	return files, nil
}

func (m *Manager) IsDirty(ctx context.Context) (bool, error) {
	files, err := m.WorktreeChanges(ctx)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

func (m *Manager) HasUnrelatedChanges(ctx context.Context, expected []string) (bool, error) {
	files, err := m.WorktreeChanges(ctx)
	if err != nil {
		return false, err
	}
	expectedSet := make(map[string]bool, len(expected))
	for _, f := range expected {
		expectedSet[f] = true
	}
	for _, f := range files {
		if !expectedSet[f] {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) TagExists(ctx context.Context, tag string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "tag", "-l", tag)
	cmd.Dir = m.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git tag -l: %w", err)
	}
	return strings.TrimSpace(string(out)) == tag, nil
}

func (m *Manager) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD, checkout a branch before --push")
	}
	return branch, nil
}

func (m *Manager) Stage(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

func (m *Manager) Commit(ctx context.Context, message string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	return m.ShortHash(ctx)
}

func (m *Manager) ShortHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Dir = m.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --short: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *Manager) CreateTag(ctx context.Context, tag, message string) error {
	return m.createTag(ctx, tag, message, false)
}

func (m *Manager) createTag(ctx context.Context, tag, message string, force bool) error {
	args := []string{"tag"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, "-a", tag, "-m", message)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git tag: %w", err)
	}
	return nil
}

func (m *Manager) Push(ctx context.Context, branch, tag string) error {
	return m.pushAtomic(ctx, branch, tag, false)
}

func (m *Manager) pushAtomic(ctx context.Context, branch, tag string, force bool) error {
	args := []string{"push", "--atomic", "origin"}
	if branch != "" {
		args = append(args, branch)
	}
	tagRef := "refs/tags/" + tag
	if force {
		tagRef = "+" + tagRef
	}
	args = append(args, tagRef)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("atomic git push of branch %q and tag %q: %w", branch, tag, err)
	}
	return nil
}

// ValidatePlan checks release-tag preconditions without changing Git state.
// Before a bump is applied, a clean tree is required unless Force is explicit.
func (m *Manager) ValidatePlan(ctx context.Context, version string, opts Options) error {
	if !opts.Tag {
		return nil
	}
	files, err := m.WorktreeChanges(ctx)
	if err != nil {
		return err
	}
	if len(files) > 0 && !opts.Force {
		return fmt.Errorf("working tree must be clean before applying a release plan; found: %s", strings.Join(files, ", "))
	}
	if exists, err := m.TagExists(ctx, version); err != nil {
		return err
	} else if exists && !opts.Force {
		return fmt.Errorf("tag %s already exists (use --force to replace it)", version)
	}
	if opts.Push {
		if _, err := m.CurrentBranch(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Run(ctx context.Context, version string, files []string, opts Options) (*Result, error) {
	res := &Result{Files: files}

	if !opts.Tag {
		return res, nil
	}

	dirty, err := m.HasUnrelatedChanges(ctx, files)
	if err != nil {
		return nil, err
	}
	if dirty && !opts.Force {
		return nil, fmt.Errorf("working tree has uncommitted changes beyond the bumped files, commit or stash first (or use --force)")
	}

	if exists, err := m.TagExists(ctx, version); err != nil {
		return nil, err
	} else if exists && !opts.Force {
		return nil, fmt.Errorf("tag %s already exists (use --force to override)", version)
	}

	if opts.DryRun {
		if len(files) > 0 {
			commitMsg := fmt.Sprintf("chore(release): %s", version)
			res.Commit = commitMsg
		}
		res.Tag = version
		if opts.Push {
			branch, err := m.CurrentBranch(ctx)
			if err == nil {
				res.Branch = branch
			}
			res.Pushed = true
		}
		return res, nil
	}

	if len(files) > 0 {
		if err := m.Stage(ctx, files); err != nil {
			return nil, err
		}
		commitMsg := fmt.Sprintf("chore(release): %s", version)
		hash, err := m.Commit(ctx, commitMsg)
		if err != nil {
			return res, fmt.Errorf("release commit failed after staging [%s]; the index and worktree retain the planned changes: %w", strings.Join(files, ", "), err)
		}
		res.Commit = hash
	}

	tagMsg := fmt.Sprintf("Release %s", version)
	if err := m.createTag(ctx, version, tagMsg, opts.Force); err != nil {
		if res.Commit != "" {
			return res, fmt.Errorf("tag creation failed after local release commit %s completed: %w", res.Commit, err)
		}
		return res, err
	}
	res.Tag = version

	if opts.Push {
		branch, err := m.CurrentBranch(ctx)
		if err != nil {
			return res, fmt.Errorf("push precondition failed; local commit %s and tag %s remain: %w", res.Commit, res.Tag, err)
		}
		res.Branch = branch
		if err := m.pushAtomic(ctx, branch, version, opts.Force); err != nil {
			return res, fmt.Errorf("remote push failed; local commit %s and tag %s remain, while the atomic remote update made no partial branch/tag change: %w", res.Commit, res.Tag, err)
		}
		res.Pushed = true
	}

	return res, nil
}
