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
