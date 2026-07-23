// Package gittag handles git tag creation, commit staging, and remote push for release automation.
package gittag

import (
	"bytes"
	"context"
	"fmt"
	"os"
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
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/tags/"+tag)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect tag %s: %w", tag, err)
	}
	return true, nil
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

func (m *Manager) LocalBranchExists(ctx context.Context, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = m.RepoPath
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect local branch %s: %w", branch, err)
	}
	return true, nil
}

func (m *Manager) RemoteBranchCommit(ctx context.Context, branch string) (string, error) {
	return m.remoteRefCommit(ctx, "refs/heads/"+branch)
}

func (m *Manager) RemoteTagCommit(ctx context.Context, tag string) (string, error) {
	out, err := m.remoteRefs(ctx, "refs/tags/"+tag, "refs/tags/"+tag+"^{}")
	if err != nil {
		return "", err
	}
	var commit string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		commit = fields[0]
		if strings.HasSuffix(fields[1], "^{}") {
			return commit, nil
		}
	}
	return commit, nil
}

func (m *Manager) remoteRefCommit(ctx context.Context, ref string) (string, error) {
	out, err := m.remoteRefs(ctx, ref)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return "", nil
	}
	if len(fields) != 2 {
		return "", fmt.Errorf("unexpected origin response for %s", ref)
	}
	return fields[0], nil
}

func (m *Manager) remoteRefs(ctx context.Context, refs ...string) (string, error) {
	args := append([]string{"ls-remote", "origin"}, refs...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve origin refs %s: %w: %s", strings.Join(refs, ", "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m *Manager) CreateBranch(ctx context.Context, branch string) error {
	return m.runGitWithEnv(ctx, os.Environ(), "switch", "-c", branch)
}

func (m *Manager) SwitchBranch(ctx context.Context, branch string) error {
	return m.runGitWithEnv(ctx, os.Environ(), "switch", branch)
}

func (m *Manager) DeleteBranch(ctx context.Context, branch string) error {
	return m.runGitWithEnv(ctx, os.Environ(), "branch", "-D", branch)
}

func (m *Manager) PushBranch(ctx context.Context, branch string) error {
	if err := m.runGitWithEnv(ctx, os.Environ(), "push", "--set-upstream", "origin", branch); err != nil {
		return fmt.Errorf("push prepared release branch %s: %w", branch, err)
	}
	return nil
}

func (m *Manager) PushTag(ctx context.Context, tag string) error {
	if err := m.runGitWithEnv(ctx, os.Environ(), "push", "origin", "refs/tags/"+tag); err != nil {
		return fmt.Errorf("push finalized release tag %s: %w", tag, err)
	}
	return nil
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

// CommitFiles creates a release commit from exactly files using a temporary
// index. The caller's index is never used to assemble the commit, so unrelated
// staged changes cannot leak into a release even when --force is explicit.
func (m *Manager) CommitFiles(ctx context.Context, message string, files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}
	index, err := os.CreateTemp("", "patchlog-git-index-*")
	if err != nil {
		return "", fmt.Errorf("create isolated git index: %w", err)
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		_ = os.Remove(indexPath)
		return "", fmt.Errorf("close isolated git index: %w", err)
	}
	// read-tree expects a missing path or a valid index, not an empty file.
	if err := os.Remove(indexPath); err != nil {
		return "", fmt.Errorf("initialize isolated git index: %w", err)
	}
	defer os.Remove(indexPath)

	env := withGitIndex(os.Environ(), indexPath)
	if err := m.runGitWithEnv(ctx, env, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("initialize isolated git index: %w", err)
	}
	args := append([]string{"add", "--"}, files...)
	if err := m.runGitWithEnv(ctx, env, args...); err != nil {
		return "", fmt.Errorf("stage release files in isolated index: %w", err)
	}
	if err := m.runGitWithEnv(ctx, env, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("commit isolated release index: %w", err)
	}

	// Synchronize only the committed paths in the caller's index to the new
	// HEAD. Any unrelated entries that were staged before the release remain.
	resetArgs := append([]string{"reset", "-q", "HEAD", "--"}, files...)
	if err := m.runGitWithEnv(ctx, os.Environ(), resetArgs...); err != nil {
		return "", fmt.Errorf("release commit succeeded but caller index synchronization failed: %w", err)
	}
	return m.ShortHash(ctx)
}

func (m *Manager) runGitWithEnv(ctx context.Context, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoPath
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func withGitIndex(env []string, indexPath string) []string {
	clean := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			clean = append(clean, entry)
		}
	}
	return append(clean, "GIT_INDEX_FILE="+indexPath)
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

func (m *Manager) HeadCommit(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD^{commit}")
	cmd.Dir = m.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *Manager) ValidateRemote(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = m.RepoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("origin remote is required for --push: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("origin remote is required for --push")
	}
	return nil
}

// ValidateProtectedPrepare proves that prepare starts from the exact remote
// protected-branch commit and that its release branch is still unused.
func (m *Manager) ValidateProtectedPrepare(ctx context.Context, protectedBranch, releaseBranch string) error {
	if err := m.validateBranchName(ctx, protectedBranch); err != nil {
		return fmt.Errorf("invalid protected branch: %w", err)
	}
	if err := m.validateBranchName(ctx, releaseBranch); err != nil {
		return fmt.Errorf("invalid release branch: %w", err)
	}
	if err := m.ValidateRemote(ctx); err != nil {
		return err
	}
	currentBranch, err := m.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if currentBranch != protectedBranch {
		return fmt.Errorf("protected prepare must start on %s, current branch is %s", protectedBranch, currentBranch)
	}
	head, err := m.HeadCommit(ctx)
	if err != nil {
		return err
	}
	remoteHead, err := m.RemoteBranchCommit(ctx, protectedBranch)
	if err != nil {
		return err
	}
	if remoteHead == "" {
		return fmt.Errorf("origin/%s does not exist", protectedBranch)
	}
	if remoteHead != head {
		return fmt.Errorf("protected branch is not current: local %s is %s, origin/%s is %s", protectedBranch, head, protectedBranch, remoteHead)
	}
	if exists, err := m.LocalBranchExists(ctx, releaseBranch); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("local release branch %s already exists", releaseBranch)
	}
	remoteRelease, err := m.RemoteBranchCommit(ctx, releaseBranch)
	if err != nil {
		return err
	}
	if remoteRelease != "" {
		return fmt.Errorf("origin release branch %s already exists at %s", releaseBranch, remoteRelease)
	}
	return nil
}

// ValidateProtectedFinalize proves that finalize tags only the exact remote
// protected-branch commit and never a local-only or dirty checkout.
func (m *Manager) ValidateProtectedFinalize(ctx context.Context, protectedBranch, tag string) error {
	if err := m.validateBranchName(ctx, protectedBranch); err != nil {
		return fmt.Errorf("invalid protected branch: %w", err)
	}
	if err := m.validateTagName(ctx, tag); err != nil {
		return fmt.Errorf("invalid release tag: %w", err)
	}
	if err := m.ValidateRemote(ctx); err != nil {
		return err
	}
	currentBranch, err := m.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if currentBranch != protectedBranch {
		return fmt.Errorf("protected finalize must run on %s, current branch is %s", protectedBranch, currentBranch)
	}
	changes, err := m.WorktreeChanges(ctx)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return fmt.Errorf("protected finalize requires a clean worktree; found: %s", strings.Join(changes, ", "))
	}
	head, err := m.HeadCommit(ctx)
	if err != nil {
		return err
	}
	remoteHead, err := m.RemoteBranchCommit(ctx, protectedBranch)
	if err != nil {
		return err
	}
	if remoteHead != head {
		return fmt.Errorf("green protected commit mismatch: local %s is %s, origin/%s is %s", protectedBranch, head, protectedBranch, remoteHead)
	}
	if exists, err := m.TagExists(ctx, tag); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("tag %s already exists locally", tag)
	}
	remoteTag, err := m.RemoteTagCommit(ctx, tag)
	if err != nil {
		return err
	}
	if remoteTag != "" {
		return fmt.Errorf("tag %s already exists on origin at %s", tag, remoteTag)
	}
	return nil
}

// VerifyRemoteTag proves that the immutable remote tag resolves to the same
// commit as the local tag before a provider release is created.
func (m *Manager) VerifyRemoteTag(ctx context.Context, tag string) error {
	localCmd := exec.CommandContext(ctx, "git", "rev-parse", "refs/tags/"+tag+"^{commit}")
	localCmd.Dir = m.RepoPath
	localOut, err := localCmd.Output()
	if err != nil {
		return fmt.Errorf("resolve local tag %s: %w", tag, err)
	}
	remoteCmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "origin", "refs/tags/"+tag, "refs/tags/"+tag+"^{}")
	remoteCmd.Dir = m.RepoPath
	remoteOut, err := remoteCmd.Output()
	if err != nil {
		return fmt.Errorf("resolve remote tag %s: %w", tag, err)
	}
	remoteCommit := ""
	for _, line := range strings.Split(strings.TrimSpace(string(remoteOut)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		remoteCommit = fields[0]
		if strings.HasSuffix(fields[1], "^{}") {
			break
		}
	}
	localCommit := strings.TrimSpace(string(localOut))
	if remoteCommit == "" {
		return fmt.Errorf("remote tag %s does not exist", tag)
	}
	if remoteCommit != localCommit {
		return fmt.Errorf("remote tag %s resolves to %s, expected %s", tag, remoteCommit, localCommit)
	}
	return nil
}

func (m *Manager) CreateTag(ctx context.Context, tag, message string) error {
	return m.createTag(ctx, tag, message, false)
}

func (m *Manager) createTag(ctx context.Context, tag, message string, force bool) error {
	if err := m.validateTagName(ctx, tag); err != nil {
		return err
	}
	args := []string{"tag"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, "-a", "-m", message, "--", tag)
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
	if err := m.validateTagName(ctx, version); err != nil {
		return fmt.Errorf("invalid release tag: %w", err)
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
		if err := m.ValidateRemote(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) validateBranchName(ctx context.Context, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch)
	cmd.Dir = m.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%q is not a valid branch name: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) validateTagName(ctx context.Context, tag string) error {
	if strings.HasPrefix(tag, "-") {
		return fmt.Errorf("%q is not a valid tag name: option-like names are rejected", tag)
	}
	cmd := exec.CommandContext(ctx, "git", "check-ref-format", "refs/tags/"+tag)
	cmd.Dir = m.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%q is not a valid tag name: %w: %s", tag, err, strings.TrimSpace(string(out)))
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
		commitMsg := fmt.Sprintf("chore(release): %s", version)
		hash, err := m.CommitFiles(ctx, commitMsg, files)
		if err != nil {
			return res, fmt.Errorf("release commit failed for planned files [%s]; the worktree retains the planned changes: %w", strings.Join(files, ", "), err)
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
