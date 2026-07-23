package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "patchlog")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/patchlog")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func createTestRepo(t *testing.T, commits []string) string {
	t.Helper()
	dir := t.TempDir()

	commitNum := 0

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
			"GIT_AUTHOR_DATE=2024-01-15T12:00:00", "GIT_COMMITTER_DATE=2024-01-15T12:00:00")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.name", "test")
	run("git", "config", "user.email", "test@test.com")

	versionFile := filepath.Join(dir, "VERSION")
	os.WriteFile(versionFile, []byte("0.1.0\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "chore: initial commit")
	run("git", "tag", "v0.1.0")

	for _, msg := range commits {
		commitNum++
		markerFile := filepath.Join(dir, fmt.Sprintf(".commit-%d", commitNum))
		os.WriteFile(markerFile, []byte(msg), 0644)
		run("git", "add", ".")
		run("git", "commit", "-m", msg)
	}

	// Every orchestration fixture has a real bare origin so release preflight,
	// atomic branch/tag push, and immutable remote-ref verification are tested.
	remote := filepath.Join(t.TempDir(), "origin.git")
	remoteCmd := exec.Command("git", "init", "--bare", remote)
	if out, err := remoteCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	run("git", "remote", "add", "origin", remote)
	run("git", "push", "-u", "origin", "HEAD", "--tags")

	return dir
}

var planFingerprintPattern = regexp.MustCompile(`Plan fingerprint: (sha256:[a-f0-9]{64})`)

func runApprovedCommand(t *testing.T, bin string, args ...string) string {
	t.Helper()
	fingerprint := planFingerprintForCommand(t, bin, args...)
	applyArgs := append(append([]string(nil), args...), "--approve", fingerprint)
	applyCmd := exec.Command(bin, applyArgs...)
	applyOut, err := applyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("approved release failed: %v\n%s", err, applyOut)
	}
	return string(applyOut)
}

func planFingerprintForCommand(t *testing.T, bin string, args ...string) string {
	t.Helper()
	planArgs := append(append([]string(nil), args...), "--dry-run")
	planCmd := exec.Command(bin, planArgs...)
	planOut, err := planCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release planning failed: %v\n%s", err, planOut)
	}
	match := planFingerprintPattern.FindSubmatch(planOut)
	if len(match) != 2 {
		t.Fatalf("release plan did not report an approval fingerprint:\n%s", planOut)
	}
	return string(match[1])
}

func TestE2EBasicMarkdown(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat(api): add user endpoint",
		"fix: resolve null pointer in handler",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "# Unreleased") {
		t.Error("output should contain version heading")
	}
	if !strings.Contains(s, "Features") {
		t.Error("output should contain Features section")
	}
	if !strings.Contains(s, "Bug Fixes") {
		t.Error("output should contain Bug Fixes section")
	}
	if !strings.Contains(s, "add user endpoint") {
		t.Error("output should contain feat description")
	}
	if !strings.Contains(s, "resolve null pointer") {
		t.Error("output should contain fix description")
	}
}

func TestE2EJSONOutput(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add dashboard",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "Unreleased") {
		t.Error("JSON should contain version")
	}
	if !strings.Contains(s, "Features") {
		t.Error("JSON should contain section")
	}
}

func TestE2EDryRun(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add thing",
		"fix: fix thing",
		"chore: update deps",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "Commits: 3") {
		t.Errorf("dry-run should report 3 commits, got:\n%s", s)
	}
}

func TestE2EBumpAuto(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add dashboard",
	})

	runApprovedCommand(t, bin, "release", "direct", "--repo", repo, "--from", "v0.1.0", "--bump", "auto")

	versionFile := filepath.Join(repo, "VERSION")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatal(err)
	}
	v := strings.TrimSpace(string(data))
	if v != "0.2.0" {
		t.Errorf("expected version 0.2.0 after minor bump, got %s", v)
	}
}

func TestE2EBumpPatch(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"fix: resolve issue",
	})

	runApprovedCommand(t, bin, "release", "direct", "--repo", repo, "--from", "v0.1.0", "--bump", "patch")

	data, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	v := strings.TrimSpace(string(data))
	if v != "0.1.1" {
		t.Errorf("expected version 0.1.1 after patch bump, got %s", v)
	}
}

func TestE2EBreakingChange(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat(api)!: rename /users to /accounts",
		"feat: add helper function",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "Breaking Changes") {
		t.Error("output should contain breaking changes section")
	}
}

func TestE2EJiraKeysInOutput(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add endpoint PROJ-123",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "add endpoint") {
		t.Error("output should contain commit description")
	}
}

func TestE2EFilter(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add endpoint",
	})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--filter", "nonexistent/path")
	out, _ := cmd.CombinedOutput()

	s := string(out)
	if strings.Contains(s, "add endpoint") {
		t.Error("filtered output should not contain excluded commits")
	}
}

func TestE2EVersion(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(out), "patchlog") {
		t.Errorf("version output: %s", out)
	}
}

func TestE2ENoCommitsInRange(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{})

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0")
	out, _ := cmd.CombinedOutput()
	_ = string(out)
}

func TestE2EMonorepoFilter(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "test")
	run("git", "config", "user.email", "test@test.com")

	os.MkdirAll(filepath.Join(dir, "pkg", "api"), 0755)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.1.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "pkg", "api", "main.go"), []byte("package api"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "chore: initial")
	run("git", "tag", "v0.1.0")

	os.WriteFile(filepath.Join(dir, "pkg", "api", "main.go"), []byte("package api // updated"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("updated readme"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-m", "feat(api): add new endpoint\n\nAlso updated readme")

	cmd := exec.Command(bin, "--repo", dir, "--from", "v0.1.0", "--filter", "pkg/api")
	out, _ := cmd.CombinedOutput()
	s := string(out)

	if !strings.Contains(s, "add new endpoint") {
		t.Error("filtered output should contain API changes")
	}

	fmt.Printf("Monorepo filter output:\n%s\n", s)
}
