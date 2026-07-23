package gittag

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.name", "test")
	run("git", "config", "user.email", "test@test.com")
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "chore: initial")
	return dir
}

func TestDetectPrefix(t *testing.T) {
	tests := []struct {
		tag, want string
	}{
		{"v1.2.3", "v"},
		{"kexp/0.33.0", "kexp/"},
		{"0.5.0", ""},
		{"release-2.0", "release-"},
	}
	for _, tc := range tests {
		got := DetectPrefix(tc.tag)
		if got != tc.want {
			t.Errorf("DetectPrefix(%q) = %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func TestIsDirtyClean(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	dirty, err := m.IsDirty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("expected clean tree")
	}
}

func TestIsDirtyModified(t *testing.T) {
	dir := initRepo(t)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	m := &Manager{RepoPath: dir}
	dirty, err := m.IsDirty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Error("expected dirty tree")
	}
}

func TestWorktreeChangesPreservesPathsWithSpaces(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "release notes.txt"), []byte("draft\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := &Manager{RepoPath: dir}
	files, err := m.WorktreeChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "release notes.txt" {
		t.Fatalf("worktree changes = %q", files)
	}
}

func TestTagExistsFalse(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	exists, err := m.TagExists(context.Background(), "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected tag to not exist")
	}
}

func TestTagExistsTrue(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	if err := m.CreateTag(context.Background(), "v1.0.0", "Release 1.0.0"); err != nil {
		t.Fatal(err)
	}
	exists, err := m.TagExists(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected tag to exist")
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	branch, err := m.CurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" || branch == "HEAD" {
		t.Errorf("expected branch name, got %q", branch)
	}
}

func TestStageAndCommit(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	if err := m.Stage(context.Background(), []string{"VERSION"}); err != nil {
		t.Fatal(err)
	}
	hash, err := m.Commit(context.Background(), "chore(release): 1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("expected non-empty commit hash")
	}
	dirty, _ := m.IsDirty(context.Background())
	if dirty {
		t.Error("expected clean tree after commit")
	}
}

func TestRunTagOnly(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	res, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tag != "1.0.1" {
		t.Errorf("expected tag 1.0.1, got %s", res.Tag)
	}
	if res.Commit == "" {
		t.Error("expected non-empty commit hash")
	}
	if res.Pushed {
		t.Error("expected not pushed")
	}
	exists, _ := m.TagExists(context.Background(), "1.0.1")
	if !exists {
		t.Error("expected tag to exist after Run")
	}
}

func TestRunRefusesDirty(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("noise"), 0644)
	_, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true})
	if err == nil {
		t.Error("expected error for unrelated dirty file without --force")
	}
}

func TestRunRefusesExistingTag(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	if err := m.CreateTag(context.Background(), "1.0.1", "existing"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	_, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true})
	if err == nil {
		t.Error("expected error for existing tag without --force")
	}
}

func TestCreateTagRejectsOptionLikeTagName(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	if err := m.CreateTag(context.Background(), "--force", "option-like tag"); err == nil || !strings.Contains(err.Error(), "option-like") {
		t.Fatalf("option-like tag validation error = %v", err)
	}
	if exists, err := m.TagExists(context.Background(), "--force"); err != nil || exists {
		t.Fatalf("option-like tag ref: exists=%v err=%v", exists, err)
	}
}

func TestRunForceOverridesDirty(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("noise"), 0644)
	res, err := m.Run(context.Background(), "1.0.1", []string{"VERSION", "extra.txt"}, Options{Tag: true, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tag != "1.0.1" {
		t.Errorf("expected tag 1.0.1, got %s", res.Tag)
	}
}

func TestRunUsesIsolatedIndexAndPreservesUnrelatedStagedChanges(t *testing.T) {
	dir := initRepo(t)
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("keep staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "unrelated.txt")

	m := &Manager{RepoPath: dir}
	if _, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true, Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := run("show", "--pretty=format:", "--name-only", "HEAD"); got != "VERSION" {
		t.Fatalf("release commit files = %q, want VERSION", got)
	}
	if got := run("diff", "--cached", "--name-only"); got != "unrelated.txt" {
		t.Fatalf("staged files after release = %q, want unrelated.txt", got)
	}
}

func TestRunNoTagReturnsEmpty(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	res, err := m.Run(context.Background(), "1.0.1", nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tag != "" {
		t.Errorf("expected empty tag, got %s", res.Tag)
	}
}

func TestRunDryRunDoesNotExecute(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	res, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tag != "1.0.1" {
		t.Errorf("expected planned tag 1.0.1, got %s", res.Tag)
	}
	if res.Commit == "" {
		t.Error("expected non-empty commit message in dry-run")
	}
	if res.Pushed {
		t.Error("expected not pushed in dry-run")
	}
	exists, _ := m.TagExists(context.Background(), "1.0.1")
	if exists {
		t.Error("tag should NOT exist after dry-run")
	}
	dirty, _ := m.IsDirty(context.Background())
	if !dirty {
		t.Error("working tree should still be dirty after dry-run (no commit made)")
	}
}

func TestRunDryRunWithPush(t *testing.T) {
	dir := initRepo(t)
	m := &Manager{RepoPath: dir}
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644)
	res, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true, Push: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pushed {
		t.Error("expected Pushed=true in dry-run result")
	}
	if res.Branch == "" {
		t.Error("expected non-empty branch in dry-run with push")
	}
	exists, _ := m.TagExists(context.Background(), "1.0.1")
	if exists {
		t.Error("tag should NOT exist after dry-run")
	}
}

func TestRunPushFailureReportsLocalCommitAndTag(t *testing.T) {
	dir := initRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &Manager{RepoPath: dir}
	res, err := m.Run(context.Background(), "1.0.1", []string{"VERSION"}, Options{Tag: true, Push: true})
	if err == nil {
		t.Fatal("expected push failure")
	}
	if res == nil || res.Commit == "" || res.Tag != "1.0.1" {
		t.Fatalf("partial local result not returned: %#v", res)
	}
	if !strings.Contains(err.Error(), "local commit") || !strings.Contains(err.Error(), "atomic remote update") {
		t.Fatalf("partial state not disclosed: %v", err)
	}
	exists, tagErr := m.TagExists(context.Background(), "1.0.1")
	if tagErr != nil || !exists {
		t.Fatalf("expected local tag to remain: exists=%v err=%v", exists, tagErr)
	}
}
