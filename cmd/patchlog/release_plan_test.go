package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/gittag"
)

func initReleasePlanRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.name", "test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.1.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "VERSION")
	run("commit", "-m", "chore: initial")
	return dir
}

func initProtectedReleasePlanRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	remote := t.TempDir()
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run(remote, "init", "--bare")
	run(repo, "init", "-b", "main")
	run(repo, "config", "user.name", "test")
	run(repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "VERSION"), []byte("0.1.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "VERSION")
	run(repo, "commit", "-m", "chore: initial")
	run(repo, "tag", "-a", "v0.1.0", "-m", "v0.1.0")
	run(repo, "remote", "add", "origin", remote)
	run(repo, "push", "-u", "origin", "main")
	run(repo, "push", "origin", "v0.1.0")
	return repo
}

func TestReleasePlanRequiresImmutablePushedTagForPublish(t *testing.T) {
	repo := initReleasePlanRepo(t)
	_, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo: repo, Publish: true, Configuration: config.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "--tag --push") {
		t.Fatalf("publish preflight error = %v", err)
	}
}

func TestReleasePlanRevalidatesImmediatelyBeforeApply(t *testing.T) {
	repo := initReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo: repo, Bump: bumpPlan, Configuration: config.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "concurrent.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := plan.Revalidate(context.Background()); err == nil || !strings.Contains(err.Error(), "changed after planning") {
		t.Fatalf("revalidation error = %v", err)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if string(version) != "0.1.0\n" {
		t.Fatalf("VERSION mutated during preflight: %q", version)
	}
}

func TestReleasePlanPreflightsDirtyTreeForBumpWithoutTag(t *testing.T) {
	repo := initReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo: repo, Bump: bumpPlan, Configuration: config.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "clean worktree") {
		t.Fatalf("dirty bump preflight error = %v", err)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if string(version) != "0.1.0\n" {
		t.Fatalf("VERSION mutated during dirty preflight: %q", version)
	}
}

func TestReleasePlanActionsReturnsCopy(t *testing.T) {
	repo := initReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo: repo, Bump: bumpPlan, Configuration: config.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	actions := plan.Actions()
	actions[0] = "tampered"
	if plan.Actions()[0] != "version bump" {
		t.Fatal("release plan exposed mutable action storage")
	}
}

func TestReleasePlanPreflightsConfluenceBeforeMutation(t *testing.T) {
	repo := initReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo: repo, Bump: bumpPlan, Confluence: true, Configuration: config.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "Confluence preflight") {
		t.Fatalf("Confluence preflight error = %v", err)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if string(version) != "0.1.0\n" {
		t.Fatalf("VERSION mutated during failed preflight: %q", version)
	}
}

func TestPreflightProviderReportsEveryMissingFieldAndRecovery(t *testing.T) {
	err := preflightProvider(config.Config{})
	if err == nil {
		t.Fatal("expected provider preflight error")
	}
	for _, field := range []string{"provider.type", "provider.token", "provider.repo"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q does not name %s", err, field)
		}
	}
	if hint := errorHint(err); !strings.Contains(hint, "patchlog release --dry-run") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestPreflightConfluenceReportsEveryMissingFieldAndRecovery(t *testing.T) {
	err := preflightConfluence(config.Config{})
	if err == nil {
		t.Fatal("expected Confluence preflight error")
	}
	for _, field := range []string{"confluence.base_url", "confluence.api_token", "confluence.space_key"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q does not name %s", err, field)
		}
	}
	if hint := errorHint(err); !strings.Contains(hint, "patchlog release --dry-run") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestReleasePlanIncludesOptInTrendSnapshotWrite(t *testing.T) {
	repo := initReleasePlanRepo(t)
	path := filepath.Join(repo, ".patchlog", "trends", "0.1.1.json")
	plan, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo:              repo,
		TrendSnapshotPath: path,
		Configuration:     config.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	actions := plan.Actions()
	if len(actions) != 1 || actions[0] != "trend snapshot write" {
		t.Fatalf("actions = %v", actions)
	}
	plannedPath, ok := plan.TrendSnapshotPath()
	if !ok || plannedPath != path {
		t.Fatalf("planned trend snapshot path = %q, %v; want %q, true", plannedPath, ok, path)
	}
	if _, err := os.Stat(filepath.Join(repo, ".patchlog")); !os.IsNotExist(err) {
		t.Fatalf("planning created analytics directory: %v", err)
	}
}

func TestReleasePlanFingerprintIsStableAndApprovalIsExact(t *testing.T) {
	repo := initReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	request := ReleasePlanRequest{
		Repo:          repo,
		Phase:         ReleasePhaseDirect,
		Bump:          bumpPlan,
		TargetVersion: bumpPlan.NewVersion,
		Configuration: config.Default(),
	}
	first, err := NewReleasePlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewReleasePlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint() == "" || first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("fingerprints are not stable: %q != %q", first.Fingerprint(), second.Fingerprint())
	}
	if err := first.RequireApproval(first.Fingerprint()); err != nil {
		t.Fatalf("exact approval failed: %v", err)
	}
	if err := first.RequireApproval("sha256:stale"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale approval error = %v", err)
	}
}

func TestReleasePlanRejectsTagVersionMismatch(t *testing.T) {
	repo := initReleasePlanRepo(t)
	_, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo:          repo,
		Phase:         ReleasePhaseDirect,
		TagName:       "v0.2.0",
		TargetVersion: "0.1.1",
		Configuration: config.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "does not match target version") {
		t.Fatalf("tag/version mismatch error = %v", err)
	}
}

func TestReleasePlanFingerprintCoversRenderedMutationContent(t *testing.T) {
	repo := initReleasePlanRepo(t)
	outputPath := filepath.Join(repo, "release.md")
	request := ReleasePlanRequest{
		Repo:           repo,
		Phase:          ReleasePhaseDirect,
		TargetVersion:  "0.1.0",
		OutputPath:     outputPath,
		RenderedOutput: []byte("first release body\n"),
		Configuration:  config.Default(),
	}
	first, err := NewReleasePlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.RenderedOutput = []byte("different release body\n")
	second, err := NewReleasePlan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint() == second.Fingerprint() {
		t.Fatal("rendered output mutation did not change the release fingerprint")
	}
}

func TestProtectedPrepareCreatesOnlyReleaseBranchCommit(t *testing.T) {
	repo := initProtectedReleasePlanRepo(t)
	bumpPlan, err := bump.CreatePlan(repo, bump.Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo:            repo,
		Phase:           ReleasePhasePrepare,
		Bump:            bumpPlan,
		TagName:         "v0.1.1",
		TargetVersion:   "0.1.1",
		ProtectedBranch: "main",
		ReleaseBranch:   "release/v0.1.1",
		Configuration:   config.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := plan.ApplyProtectedPrepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch != "release/v0.1.1" || !result.Pushed || result.Tag != "" {
		t.Fatalf("prepare result = %#v", result)
	}
	if plan.HasTag() {
		t.Fatal("protected prepare fingerprinted a tag mutation")
	}
	if files := result.Files; len(files) != 1 || files[0] != "VERSION" {
		t.Fatalf("prepared files = %v", files)
	}
	if got := strings.TrimSpace(gitCommandOutput(t, repo, "show", "HEAD:VERSION")); got != "0.1.1" {
		t.Fatalf("prepared VERSION = %q", got)
	}
	if tags := strings.TrimSpace(gitCommandOutput(t, repo, "tag", "--list", "v0.1.1")); tags != "" {
		t.Fatalf("prepare created tag %q", tags)
	}
	remote := strings.Fields(gitCommandOutput(t, repo, "ls-remote", "origin", "refs/heads/release/v0.1.1"))
	if len(remote) != 2 {
		t.Fatalf("prepared remote branch = %v", remote)
	}
}

func TestProtectedFinalizeRejectsLocalCommitNotOnRemoteMain(t *testing.T) {
	repo := initProtectedReleasePlanRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "VERSION"), []byte("0.1.1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, repo, "add", "VERSION")
	runGitCommand(t, repo, "commit", "-m", "chore(release): 0.1.1")
	_, err := NewReleasePlan(context.Background(), ReleasePlanRequest{
		Repo:            repo,
		Phase:           ReleasePhaseFinalize,
		TagName:         "v0.1.1",
		TargetVersion:   "0.1.1",
		ProtectedBranch: "main",
		TagOptions:      gittag.Options{Tag: true, Push: true},
		Configuration:   config.Default(),
	})
	if err == nil || !strings.Contains(err.Error(), "green protected commit mismatch") {
		t.Fatalf("finalize mismatch error = %v", err)
	}
	if tags := strings.TrimSpace(gitCommandOutput(t, repo, "tag", "--list", "v0.1.1")); tags != "" {
		t.Fatalf("failed finalize created tag %q", tags)
	}
}

func gitCommandOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func runGitCommand(t *testing.T, repo string, args ...string) {
	t.Helper()
	_ = gitCommandOutput(t, repo, args...)
}
