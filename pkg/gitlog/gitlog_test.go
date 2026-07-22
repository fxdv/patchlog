package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("init")
	run("config", "user.name", "test")
	run("config", "user.email", "test@test.com")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "feat: initial commit")
	return dir
}

func TestFetchLogBasic(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	commits, err := f.FetchLog(context.Background(), "--root", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Author != "test" {
		t.Errorf("expected author 'test', got %q", commits[0].Author)
	}
	if commits[0].Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestFetchLogMultipleCommits(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("data\n"), 0644)
	run("add", ".")
	run("commit", "-m", "fix: second commit")

	f := &Fetcher{RepoPath: dir}
	commits, err := f.FetchLog(context.Background(), "--root", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestFetchLogRange(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("data\n"), 0644)
	run("add", ".")
	run("commit", "-m", "fix: second commit")

	f := &Fetcher{RepoPath: dir}
	commits, err := f.FetchLog(context.Background(), "v1.0.0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit after v1.0.0, got %d", len(commits))
	}
}

func TestFetchLogWithFilter(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	os.Mkdir(filepath.Join(dir, "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "pkg", "api.go"), []byte("package pkg\n"), 0644)
	run("add", ".")
	run("commit", "-m", "feat: add api")
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("data\n"), 0644)
	run("add", ".")
	run("commit", "-m", "chore: update other")

	f := &Fetcher{RepoPath: dir, Filter: "pkg"}
	commits, err := f.FetchLog(context.Background(), "--root", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit with pkg filter (only api commit), got %d", len(commits))
	}
}

func TestLatestTag(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	f := &Fetcher{RepoPath: dir}
	tag, err := f.LatestTag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", tag)
	}
}

func TestLatestTagNone(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	_, err := f.LatestTag(context.Background())
	if err == nil {
		t.Error("expected error when no tags exist")
	}
}

func TestChangedFiles(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("data\n"), 0644)
	run("add", ".")
	run("commit", "-m", "feat: add file")

	f := &Fetcher{RepoPath: dir}
	commits, _ := f.FetchLog(context.Background(), "--root", "HEAD")
	if len(commits) < 2 {
		t.Fatal("expected at least 2 commits")
	}
	files, err := f.ChangedFiles(context.Background(), commits[1].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Error("expected at least 1 changed file")
	}
}

func TestGetDiffStat(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	commits, _ := f.FetchLog(context.Background(), "--root", "HEAD")
	if len(commits) == 0 {
		t.Fatal("no commits")
	}
	stat, err := f.GetDiffStat(context.Background(), commits[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if stat.FilesChanged == 0 {
		t.Error("expected at least 1 file changed")
	}
	if stat.Insertions == 0 {
		t.Error("expected some insertions")
	}
}

func TestGetDiffStatFields(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	commits, _ := f.FetchLog(context.Background(), "--root", "HEAD")
	stat, _ := f.GetDiffStat(context.Background(), commits[0].Hash)
	if len(stat.Files) == 0 {
		t.Error("expected non-empty Files list")
	}
}

func TestFetchLogTimestamp(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	commits, _ := f.FetchLog(context.Background(), "--root", "HEAD")
	if commits[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestChangedFilesInRange(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	run("add", ".")
	run("commit", "-m", "feat: add files")

	f := &Fetcher{RepoPath: dir}
	files, err := f.ChangedFilesInRange(context.Background(), "v1.0.0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 changed files, got %d: %v", len(files), files)
	}
}

func TestChangedFilesInRangeRoot(t *testing.T) {
	dir := initTestRepo(t)
	f := &Fetcher{RepoPath: dir}
	files, err := f.ChangedFilesInRange(context.Background(), "--root", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Error("expected at least 1 changed file from root")
	}
}

func TestChangedFilesInRangeNone(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	f := &Fetcher{RepoPath: dir}
	files, err := f.ChangedFilesInRange(context.Background(), "v1.0.0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if files != nil {
		t.Fatalf("expected nil, got %v", files)
	}
}

func TestRangeDiff(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"^18.2.0"}}`), 0644)
	run("add", ".")
	run("commit", "-m", "chore: add react")

	f := &Fetcher{RepoPath: dir}
	diff, err := f.RangeDiff(context.Background(), "v1.0.0", "HEAD", "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "react") {
		t.Errorf("expected diff to contain 'react', got: %s", diff)
	}
	if !strings.Contains(diff, "+") {
		t.Error("expected diff to contain addition lines")
	}
}

func TestRangeDiffSpecificFile(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	run("tag", "v1.0.0")
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"^18.2.0"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("data\n"), 0644)
	run("add", ".")
	run("commit", "-m", "chore: add files")

	f := &Fetcher{RepoPath: dir}
	diff, err := f.RangeDiff(context.Background(), "v1.0.0", "HEAD", "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(diff, "other.txt") {
		t.Error("diff should not contain other.txt when filtering to package.json")
	}
}
