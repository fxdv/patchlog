package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

type repositorySnapshot struct {
	Head          string
	Status        string
	Tags          string
	WorktreeFiles map[string]string
}

func snapshotRepository(t *testing.T, repo string) repositorySnapshot {
	t.Helper()
	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	files := make(map[string]string)
	err := filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repositorySnapshot{
		Head:          runGit("rev-parse", "HEAD"),
		Status:        runGit("status", "--porcelain=v1", "--untracked-files=all"),
		Tags:          runGit("tag", "--list", "--sort=refname"),
		WorktreeFiles: files,
	}
}

func TestE2EDryRunPreservesFilesystemAndGitState(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	before := snapshotRepository(t, repo)
	outPath := filepath.Join(repo, "planned-release.md")

	cmd := exec.Command(bin,
		"release",
		"--repo", repo,
		"--from", "v0.1.0",
		"--bump", "patch",
		"--tag",
		"--html",
		"--changelog",
		"--out", outPath,
		"--dry-run",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}

	after := snapshotRepository(t, repo)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("dry-run changed repository state\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestE2EMutationsRequireReleaseSubcommand(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	versionBefore, _ := os.ReadFile(filepath.Join(repo, "VERSION"))

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--bump", "patch")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("primary report command accepted a release mutation:\n%s", out)
	}
	if !strings.Contains(string(out), "patchlog release") {
		t.Fatalf("missing focused-subcommand guidance:\n%s", out)
	}
	versionAfter, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if !reflect.DeepEqual(versionAfter, versionBefore) {
		t.Fatalf("VERSION changed outside release subcommand: %q -> %q", versionBefore, versionAfter)
	}
}

func TestE2EDirtyTreeFailsBeforeBumpOrTag(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	dirtyPath := filepath.Join(repo, "unrelated.txt")
	if err := os.WriteFile(dirtyPath, []byte("user work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	versionBefore, _ := os.ReadFile(filepath.Join(repo, "VERSION"))

	cmd := exec.Command(bin, "release", "--repo", repo, "--from", "v0.1.0", "--bump", "patch", "--tag")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("dirty release unexpectedly succeeded:\n%s", out)
	}
	versionAfter, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if !reflect.DeepEqual(versionAfter, versionBefore) {
		t.Fatalf("VERSION changed despite dirty-tree preflight: %q -> %q", versionBefore, versionAfter)
	}
	if data, _ := os.ReadFile(dirtyPath); string(data) != "user work\n" {
		t.Fatalf("unrelated user file changed: %q", data)
	}
	if tags := gitOutput(t, repo, "tag", "--list", "v0.1.1"); strings.TrimSpace(tags) != "" {
		t.Fatalf("tag was created despite failed preflight: %q", tags)
	}
}

func TestE2EFailedGatePreventsBumpAndTag(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"update release behavior"})
	versionBefore, _ := os.ReadFile(filepath.Join(repo, "VERSION"))

	cmd := exec.Command(bin,
		"release",
		"--repo", repo,
		"--from", "v0.1.0",
		"--bump", "patch",
		"--tag",
		"--gate",
		"--require-conv", "1",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("failed gate unexpectedly succeeded:\n%s", out)
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("gate exit = %v, want code 3\n%s", err, out)
	}
	versionAfter, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if !reflect.DeepEqual(versionAfter, versionBefore) {
		t.Fatalf("VERSION changed before gate completed: %q -> %q", versionBefore, versionAfter)
	}
	if tags := gitOutput(t, repo, "tag", "--list", "v0.1.1"); strings.TrimSpace(tags) != "" {
		t.Fatalf("tag was created before gate completed: %q", tags)
	}
}

func TestE2EPartialRemoteFailureStopsLaterOperations(t *testing.T) {
	var providerCalls atomic.Int32
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerCalls.Add(1)
		http.Error(w, "provider unavailable", http.StatusInternalServerError)
	}))
	defer providerServer.Close()

	var confluenceCalls atomic.Int32
	confluenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		confluenceCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer confluenceServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"feat: add release behavior"})
	writeConfig(t, repo, "", confluenceServer.URL, providerServer.URL, "github")
	commitTestConfig(t, repo)

	cfgPath := filepath.Join(repo, "patchlog.yaml")
	cmd := exec.Command(bin,
		"release",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", cfgPath,
		"--bump", "minor",
		"--tag",
		"--push",
		"--publish",
		"--confluence",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("provider failure unexpectedly succeeded:\n%s", out)
	}
	if providerCalls.Load() == 0 {
		t.Fatal("provider was not called")
	}
	if confluenceCalls.Load() != 0 {
		t.Fatalf("later Confluence action ran after provider failure (%d calls)", confluenceCalls.Load())
	}
	if !strings.Contains(string(out), "after [version bump, git commit/tag/push] completed") {
		t.Fatalf("partial completion was not disclosed:\n%s", out)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if strings.TrimSpace(string(version)) != "0.2.0" {
		t.Fatalf("expected completed local bump to be reported and retained, got %q", version)
	}
}

func TestE2ELaterRemoteFailureReportsEarlierRemoteCompletion(t *testing.T) {
	var providerCalls atomic.Int32
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"html_url":"https://example.invalid/releases/0.2.0","tag_name":"0.2.0"}`))
	}))
	defer providerServer.Close()

	var confluenceCalls atomic.Int32
	confluenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		confluenceCalls.Add(1)
		http.Error(w, "invalid Confluence request", http.StatusBadRequest)
	}))
	defer confluenceServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"feat: add release behavior"})
	writeConfig(t, repo, "", confluenceServer.URL, providerServer.URL, "github")
	commitTestConfig(t, repo)

	cmd := exec.Command(bin,
		"release",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", filepath.Join(repo, "patchlog.yaml"),
		"--bump", "minor",
		"--tag",
		"--push",
		"--publish",
		"--confluence",
		"--changelog",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("later remote failure unexpectedly succeeded:\n%s", out)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", providerCalls.Load())
	}
	if confluenceCalls.Load() == 0 {
		t.Fatal("Confluence failure was not reached")
	}
	if !strings.Contains(string(out), "after [version bump, git commit/tag/push, provider publish] completed") {
		t.Fatalf("earlier remote completion was not disclosed:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "CHANGELOG.md")); !os.IsNotExist(statErr) {
		t.Fatalf("later changelog operation ran after remote failure: %v", statErr)
	}
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
