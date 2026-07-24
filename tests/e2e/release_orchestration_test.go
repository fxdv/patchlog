package e2e

import (
	"encoding/json"
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
		"direct",
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

func TestE2EGoldenPathDryRunPlansSafeReleaseWithoutMutation(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	before := snapshotRepository(t, repo)

	cmd := exec.Command(bin, "release", "--repo", repo, "--from", "v0.1.0", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golden-path dry-run failed: %v\n%s", err, out)
	}
	for _, expected := range []string{
		"Would bump 0.1.0",
		"Would create release branch: release/v0.1.1",
		"Would push release branch: origin/release/v0.1.1",
		"Plan fingerprint: sha256:",
	} {
		if !strings.Contains(string(out), expected) {
			t.Fatalf("golden-path dry-run output is missing %q:\n%s", expected, out)
		}
	}

	after := snapshotRepository(t, repo)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("golden-path dry-run changed repository state\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestE2EPlanJSONExportsVersionedImmutablePlan(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	before := snapshotRepository(t, repo)

	cmd := exec.Command(bin,
		"release",
		"--repo", repo,
		"--from", "v0.1.0",
		"--dry-run",
		"--plan-json",
		"--quiet",
	)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("plan JSON failed: %v", err)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatalf("decode plan JSON: %v\n%s", err, raw)
	}
	if plan["schema"] != "https://patchlog.dev/schemas/release-plan/v1" ||
		plan["phase"] != "prepare" ||
		!strings.HasPrefix(plan["fingerprint"].(string), "sha256:") {
		t.Fatalf("plan JSON = %#v", plan)
	}

	after := snapshotRepository(t, repo)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("plan JSON changed repository state\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestE2EProtectedPrepareBumpsAndPushesBranchWithoutTag(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})

	runApprovedCommand(t, bin, "release", "prepare", "--repo", repo, "--from", "v0.1.0")

	version, err := os.ReadFile(filepath.Join(repo, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(version)) != "0.1.1" {
		t.Fatalf("VERSION = %q, want 0.1.1", version)
	}
	if branch := strings.TrimSpace(gitOutput(t, repo, "branch", "--show-current")); branch != "release/v0.1.1" {
		t.Fatalf("current branch = %q, want release/v0.1.1", branch)
	}
	remoteBranch := strings.TrimSpace(gitOutput(t, repo, "ls-remote", "origin", "refs/heads/release/v0.1.1"))
	if fields := strings.Fields(remoteBranch); len(fields) != 2 {
		t.Fatalf("prepared remote branch = %q", remoteBranch)
	}
	if tags := strings.TrimSpace(gitOutput(t, repo, "tag", "--list", "v0.1.1")); tags != "" {
		t.Fatalf("protected prepare created tag %q", tags)
	}
	if remoteMainVersion := strings.TrimSpace(gitOutput(t, repo, "show", "origin/main:VERSION")); remoteMainVersion != "0.1.0" {
		t.Fatalf("protected prepare mutated origin/main VERSION to %q", remoteMainVersion)
	}
	if _, err := os.Stat(filepath.Join(repo, ".patchlog", "trends")); !os.IsNotExist(err) {
		t.Fatalf("golden path wrote optional trend analytics: %v", err)
	}
}

func TestE2EProtectedPrepareRequiresExactCurrentApproval(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	versionBefore, _ := os.ReadFile(filepath.Join(repo, "VERSION"))

	missing := exec.Command(bin, "release", "prepare", "--repo", repo, "--from", "v0.1.0")
	missingOut, missingErr := missing.CombinedOutput()
	if missingErr == nil {
		t.Fatalf("prepare without approval unexpectedly succeeded:\n%s", missingOut)
	}
	if !strings.Contains(string(missingOut), "requires approval of plan sha256:") {
		t.Fatalf("missing approval guidance:\n%s", missingOut)
	}

	staleFingerprint := planFingerprintForCommand(t, bin,
		"release", "prepare", "--repo", repo, "--from", "v0.1.0",
	)

	stale := exec.Command(bin,
		"release", "prepare",
		"--repo", repo,
		"--from", "v0.1.0",
		"--bump", "minor",
		"--approve", staleFingerprint,
	)
	staleOut, staleErr := stale.CombinedOutput()
	if staleErr == nil {
		t.Fatalf("stale approval unexpectedly succeeded:\n%s", staleOut)
	}
	if !strings.Contains(string(staleOut), "does not match current plan") {
		t.Fatalf("stale approval guidance:\n%s", staleOut)
	}
	versionAfter, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if !reflect.DeepEqual(versionAfter, versionBefore) {
		t.Fatalf("approval rejection mutated VERSION: %q -> %q", versionBefore, versionAfter)
	}
	if branches := strings.TrimSpace(gitOutput(t, repo, "branch", "--list", "release/v0.2.0")); branches != "" {
		t.Fatalf("approval rejection created release branch %q", branches)
	}
}

func TestE2EProtectedFinalizeTagsExactMergedRemoteMain(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})
	providerServer := newMockGitHubServer(t)
	defer providerServer.Close()
	writeConfig(t, repo, "", "", providerServer.URL, "github")
	commitTestConfig(t, repo)
	cfgPath := filepath.Join(repo, "patchlog.yaml")

	runApprovedCommand(t, bin, "release", "prepare", "--repo", repo, "--from", "v0.1.0", "--config", cfgPath)
	gitOutput(t, repo, "switch", "main")
	gitOutput(t, repo, "merge", "--squash", "release/v0.1.1")
	gitOutput(t, repo, "commit", "-m", "chore(release): prepare v0.1.1")
	gitOutput(t, repo, "push", "origin", "main")

	universalPlan := exec.Command(bin, "release", "--repo", repo, "--from", "v0.1.0", "--config", cfgPath, "--dry-run")
	planOut, err := universalPlan.CombinedOutput()
	if err != nil {
		t.Fatalf("universal finalize planning failed: %v\n%s", err, planOut)
	}
	if !strings.Contains(string(planOut), "Would tag exact verified origin/main commit") {
		t.Fatalf("universal planner did not detect finalize phase:\n%s", planOut)
	}

	runApprovedCommand(t, bin, "release", "finalize", "--repo", repo, "--from", "v0.1.0", "--config", cfgPath)
	head := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	localTag := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "v0.1.1^{commit}"))
	remoteTag := strings.Fields(gitOutput(t, repo, "ls-remote", "origin", "refs/tags/v0.1.1^{}"))
	if localTag != head {
		t.Fatalf("finalized tag target %s != green main %s", localTag, head)
	}
	if len(remoteTag) != 2 || remoteTag[0] != head {
		t.Fatalf("remote finalized tag = %v, want %s", remoteTag, head)
	}
	if tagType := strings.TrimSpace(gitOutput(t, repo, "cat-file", "-t", "v0.1.1")); tagType != "tag" {
		t.Fatalf("finalized tag type = %q, want annotated tag", tagType)
	}
}

func TestE2EExplicitDirectModeBumpsTagsAndAtomicallyPushes(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: correct release behavior"})

	runApprovedCommand(t, bin,
		"release", "direct",
		"--repo", repo,
		"--from", "v0.1.0",
		"--bump", "patch",
		"--tag",
		"--push",
	)

	localTag := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "v0.1.1^{commit}"))
	localHead := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	remoteTag := strings.TrimSpace(gitOutput(t, repo, "ls-remote", "origin", "refs/tags/v0.1.1^{}"))
	if localTag != localHead {
		t.Fatalf("local tag target %s != HEAD %s", localTag, localHead)
	}
	if fields := strings.Fields(remoteTag); len(fields) != 2 || fields[0] != localHead {
		t.Fatalf("remote tag target = %q, want %s", remoteTag, localHead)
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

	cmd := exec.Command(bin, "release", "direct", "--repo", repo, "--from", "v0.1.0", "--bump", "patch", "--tag")
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
		"direct",
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
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveSuccessfulGitHubCommitPolicy(w, r) {
			return
		}
		providerCalls.Add(1)
		http.Error(w, "provider unavailable", http.StatusInternalServerError)
	}))
	defer providerServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"feat: add release behavior"})
	writeConfig(t, repo, "", "", providerServer.URL, "github")
	commitTestConfig(t, repo)

	cfgPath := filepath.Join(repo, "patchlog.yaml")
	args := []string{
		"release", "direct",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", cfgPath,
		"--bump", "minor",
		"--tag",
		"--push",
		"--publish",
		"--changelog",
	}
	fingerprint := planFingerprintForCommand(t, bin, args...)
	cmd := exec.Command(bin, append(args, "--approve", fingerprint)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("provider failure unexpectedly succeeded:\n%s", out)
	}
	if providerCalls.Load() == 0 {
		t.Fatal("provider was not called")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "CHANGELOG.md")); !os.IsNotExist(statErr) {
		t.Fatalf("later changelog operation ran after provider failure: %v", statErr)
	}
	if !strings.Contains(string(out), "after [version bump, git commit/tag/push] completed") {
		t.Fatalf("partial completion was not disclosed:\n%s", out)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if strings.TrimSpace(string(version)) != "0.2.0" {
		t.Fatalf("expected completed local bump to be reported and retained, got %q", version)
	}
}

func TestE2EProtectedFinalizeReportsTagBeforeProviderFailure(t *testing.T) {
	var providerCalls atomic.Int32
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveSuccessfulGitHubCommitPolicy(w, r) {
			return
		}
		providerCalls.Add(1)
		http.Error(w, "provider unavailable", http.StatusInternalServerError)
	}))
	defer providerServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"feat: add protected release behavior"})
	writeConfig(t, repo, "", "", providerServer.URL, "github")
	commitTestConfig(t, repo)

	runApprovedCommand(t, bin,
		"release", "prepare",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", filepath.Join(repo, "patchlog.yaml"),
	)
	gitOutput(t, repo, "switch", "main")
	gitOutput(t, repo, "merge", "--squash", "release/v0.2.0")
	gitOutput(t, repo, "commit", "-m", "chore(release): prepare v0.2.0")
	gitOutput(t, repo, "push", "origin", "main")

	args := []string{
		"release", "finalize",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", filepath.Join(repo, "patchlog.yaml"),
		"--publish",
	}
	fingerprint := planFingerprintForCommand(t, bin, args...)
	cmd := exec.Command(bin, append(args, "--approve", fingerprint)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("protected provider failure unexpectedly succeeded:\n%s", out)
	}
	if providerCalls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", providerCalls.Load())
	}
	if !strings.Contains(string(out), "after [protected release finalize] completed") {
		t.Fatalf("completed immutable tag was not disclosed:\n%s", out)
	}
	head := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	remoteTag := strings.Fields(gitOutput(t, repo, "ls-remote", "origin", "refs/tags/v0.2.0^{}"))
	if len(remoteTag) != 2 || remoteTag[0] != head {
		t.Fatalf("remote protected tag = %v, want %s", remoteTag, head)
	}
}

func TestE2EDedicatedConfluenceCannotJoinReleaseTransaction(t *testing.T) {
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
		"direct",
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
		t.Fatalf("mixed release and Confluence workflow unexpectedly succeeded:\n%s", out)
	}
	if providerCalls.Load() != 0 {
		t.Fatalf("provider was called before focused-subcommand validation: %d", providerCalls.Load())
	}
	if confluenceCalls.Load() != 0 {
		t.Fatalf("Confluence was called before focused-subcommand validation: %d", confluenceCalls.Load())
	}
	if !strings.Contains(string(out), "moved to `patchlog confluence`") {
		t.Fatalf("focused-subcommand guidance missing:\n%s", out)
	}
	version, _ := os.ReadFile(filepath.Join(repo, "VERSION"))
	if strings.TrimSpace(string(version)) != "0.1.0" {
		t.Fatalf("mixed workflow mutated VERSION: %q", version)
	}
}

func TestE2EConfluenceChangelogDestinationRequiresFocusedSubcommand(t *testing.T) {
	var confluenceCalls atomic.Int32
	confluenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		confluenceCalls.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer confluenceServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{"fix: keep Confluence outside release"})
	writeConfig(t, repo, "", confluenceServer.URL, "", "")
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg = append(cfg, []byte("changelog:\n  accumulate: true\n  destination: confluence\n  title: Changelog\n")...)
	if err := os.WriteFile(cfgPath, cfg, 0644); err != nil {
		t.Fatal(err)
	}
	commitTestConfig(t, repo)

	cmd := exec.Command(bin,
		"release", "direct",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", cfgPath,
		"--changelog",
		"--dry-run",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("release accepted Confluence changelog destination:\n%s", out)
	}
	if !strings.Contains(string(out), "patchlog confluence") {
		t.Fatalf("focused Confluence guidance missing:\n%s", out)
	}
	if confluenceCalls.Load() != 0 {
		t.Fatalf("Confluence was called before focused-subcommand validation: %d", confluenceCalls.Load())
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
