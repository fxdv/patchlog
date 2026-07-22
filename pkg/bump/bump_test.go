package bump

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBumpVersionPatch(t *testing.T) {
	got, err := bumpVersion("1.2.3", Patch)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4, got %s", got)
	}
}

func TestBumpVersionMinor(t *testing.T) {
	got, err := bumpVersion("1.2.3", Minor)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.3.0" {
		t.Errorf("expected 1.3.0, got %s", got)
	}
}

func TestBumpVersionMajor(t *testing.T) {
	got, err := bumpVersion("1.2.3", Major)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got)
	}
}

func TestBumpVersionTwoPart(t *testing.T) {
	got, err := bumpVersion("1.2", Minor)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.3.0" {
		t.Errorf("expected 1.3.0, got %s", got)
	}
}

func TestBumpVersionPreRelease(t *testing.T) {
	got, err := bumpVersion("1.2.3-beta.1", Minor)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.3.0" {
		t.Errorf("expected 1.3.0 (pre stripped), got %s", got)
	}
}

func TestBumpVersionInvalidTooShort(t *testing.T) {
	_, err := bumpVersion("1", Patch)
	if err == nil {
		t.Error("expected error for single-part version")
	}
}

func TestBumpVersionMajorResetsLower(t *testing.T) {
	got, err := bumpVersion("3.9.7", Major)
	if err != nil {
		t.Fatal(err)
	}
	if got != "4.0.0" {
		t.Errorf("expected 4.0.0, got %s", got)
	}
}

func TestBumpVersionMinorResetsPatch(t *testing.T) {
	got, err := bumpVersion("2.5.9", Minor)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2.6.0" {
		t.Errorf("expected 2.6.0, got %s", got)
	}
}

func TestParseLevelValid(t *testing.T) {
	tests := map[string]Level{
		"patch": Patch,
		"minor": Minor,
		"major": Major,
	}
	for s, want := range tests {
		got, err := ParseLevel(s)
		if err != nil {
			t.Errorf("ParseLevel(%q) error: %v", s, err)
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestParseLevelInvalid(t *testing.T) {
	_, err := ParseLevel("hotfix")
	if err == nil {
		t.Error("expected error for invalid level")
	}
}

func TestLevelString(t *testing.T) {
	if Patch.String() != "patch" {
		t.Errorf("Patch.String() = %q", Patch.String())
	}
	if Minor.String() != "minor" {
		t.Errorf("Minor.String() = %q", Minor.String())
	}
	if Major.String() != "major" {
		t.Errorf("Major.String() = %q", Major.String())
	}
}

func TestRunVersionFile(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, "VERSION")
	os.WriteFile(versionFile, []byte("1.2.3\n"), 0644)

	got, err := Run(dir, Minor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.NewVersion != "1.3.0" {
		t.Errorf("expected 1.3.0, got %s", got.NewVersion)
	}
	if !reflect.DeepEqual(got.ChangedFiles, []string{"VERSION"}) {
		t.Fatalf("changed files = %v", got.ChangedFiles)
	}

	data, _ := os.ReadFile(versionFile)
	if string(data) != "1.3.0\n" {
		t.Errorf("file content: got %q, want %q", string(data), "1.3.0\n")
	}
}

func TestRunNPM(t *testing.T) {
	dir := t.TempDir()
	pkgFile := filepath.Join(dir, "package.json")
	os.WriteFile(pkgFile, []byte(`{"name": "test", "version": "2.0.0"}`), 0644)

	got, err := Run(dir, Patch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.NewVersion != "2.0.1" {
		t.Errorf("expected 2.0.1, got %s", got.NewVersion)
	}
}

func TestRunNoVersionFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Run(dir, Patch, nil)
	if err == nil {
		t.Error("expected error when no version file found")
	}
}

func TestRunCargo(t *testing.T) {
	dir := t.TempDir()
	cargoFile := filepath.Join(dir, "Cargo.toml")
	os.WriteFile(cargoFile, []byte(`[package]
name = "test"
version = "0.1.0"
`), 0644)

	got, err := Run(dir, Minor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.NewVersion != "0.2.0" {
		t.Errorf("expected 0.2.0, got %s", got.NewVersion)
	}
}

func TestBumpVersionNeverDecreases(t *testing.T) {
	versions := []string{"0.0.1", "0.1.0", "1.0.0", "10.20.30"}
	for _, v := range versions {
		for _, lvl := range []Level{Patch, Minor, Major} {
			got, err := bumpVersion(v, lvl)
			if err != nil {
				t.Errorf("bumpVersion(%q, %d): %v", v, lvl, err)
				continue
			}
			if got <= v {
				t.Errorf("bumpVersion(%q, %d) = %q, should be > %q", v, lvl, got, v)
			}
		}
	}
}

func TestBumpVersionInvalidNonNumeric(t *testing.T) {
	_, err := bumpVersion("1.x.3", Patch)
	if err == nil {
		t.Error("expected error for non-numeric minor part")
	}
}

func TestBumpVersionInvalidMajor(t *testing.T) {
	_, err := bumpVersion("x.1.0", Patch)
	if err == nil {
		t.Error("expected error for non-numeric major part")
	}
}

func TestBumpVersionInvalidLevel(t *testing.T) {
	_, err := bumpVersion("1.2.3", Level(0))
	if err == nil {
		t.Error("expected error for invalid bump level 0")
	}
}

func TestRunWithExtraFiles(t *testing.T) {
	dir := t.TempDir()
	customFile := filepath.Join(dir, "VERSION")
	os.WriteFile(customFile, []byte("3.0.0\n"), 0644)

	got, err := Run(dir, Minor, []string{"VERSION"})
	if err != nil {
		t.Fatal(err)
	}
	if got.NewVersion != "3.1.0" {
		t.Errorf("expected 3.1.0, got %s", got.NewVersion)
	}
}

func TestCreatePlanIsImmutableAndReturnsExactFiles(t *testing.T) {
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "package.json")
	versionPath := filepath.Join(dir, "VERSION")
	packageBefore := []byte("{\n  \"name\": \"demo\",\n  \"version\": \"1.2.3\"\n}\n")
	versionBefore := []byte("1.2.3\n")
	if err := os.WriteFile(packagePath, packageBefore, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, versionBefore, 0600); err != nil {
		t.Fatal(err)
	}

	plan, err := CreatePlan(dir, Minor, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentVersion != "1.2.3" || plan.NewVersion != "1.3.0" {
		t.Fatalf("unexpected versions: %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
	if got, want := plan.ChangedFiles(), []string{"package.json", "VERSION"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFiles() = %v, want %v", got, want)
	}
	assertFileContent(t, packagePath, packageBefore)
	assertFileContent(t, versionPath, versionBefore)

	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, packagePath, []byte("{\n  \"name\": \"demo\",\n  \"version\": \"1.3.0\"\n}\n"))
	assertFileContent(t, versionPath, []byte("1.3.0\n"))
	if info, err := os.Stat(versionPath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("version file mode = %v, err = %v", info.Mode().Perm(), err)
	}
}

func TestCreatePlanRejectsMismatchedVersions(t *testing.T) {
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "package.json")
	versionPath := filepath.Join(dir, "VERSION")
	packageBefore := []byte(`{"name":"demo","version":"1.2.3"}`)
	versionBefore := []byte("2.0.0\n")
	if err := os.WriteFile(packagePath, packageBefore, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, versionBefore, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := CreatePlan(dir, Patch, nil, true); err == nil {
		t.Fatal("expected mismatched versions to fail")
	}
	assertFileContent(t, packagePath, packageBefore)
	assertFileContent(t, versionPath, versionBefore)
}

func TestPlanApplyPreflightPreventsPartialMutation(t *testing.T) {
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "package.json")
	versionPath := filepath.Join(dir, "VERSION")
	packageBefore := []byte(`{"name":"demo","version":"1.2.3"}`)
	versionBefore := []byte("1.2.3\n")
	if err := os.WriteFile(packagePath, packageBefore, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, versionBefore, 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := CreatePlan(dir, Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	concurrentContent := []byte("1.2.3 # edited after planning\n")
	if err := os.WriteFile(versionPath, concurrentContent, 0644); err != nil {
		t.Fatal(err)
	}

	if err := plan.Apply(); err == nil {
		t.Fatal("expected preflight failure")
	}
	assertFileContent(t, packagePath, packageBefore)
	assertFileContent(t, versionPath, concurrentContent)
}

func TestPlanApplyRollsBackAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "package.json")
	versionPath := filepath.Join(dir, "VERSION")
	packageBefore := []byte(`{"name":"demo","version":"1.2.3"}`)
	versionBefore := []byte("1.2.3\n")
	if err := os.WriteFile(packagePath, packageBefore, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, versionBefore, 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := CreatePlan(dir, Patch, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	renames := 0
	err = plan.applyWithRename(func(oldPath, newPath string) error {
		renames++
		if renames == 2 {
			return errors.New("injected rename failure")
		}
		return os.Rename(oldPath, newPath)
	})
	if err == nil {
		t.Fatal("expected injected apply failure")
	}
	assertFileContent(t, packagePath, packageBefore)
	assertFileContent(t, versionPath, versionBefore)
}

func TestCreatePlanDoesNotFallBackForMissingRepository(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := CreatePlan(missing, Patch, nil, true); err == nil {
		t.Fatal("expected missing repository to fail")
	}
}

func TestCreatePlanRejectsEscapingExtraFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreatePlan(dir, Patch, []string{"../VERSION"}, false); err == nil {
		t.Fatal("expected escaping extra file to fail")
	}
}

func TestCreatePlanRejectsAutoDetectedSymlinkOutsideRepo(t *testing.T) {
	repo := t.TempDir()
	outside := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(outside, []byte("1.2.3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "VERSION")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := CreatePlan(repo, Patch, nil, true); err == nil || !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("expected outside-repository symlink rejection, got %v", err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s content = %q, want %q", path, got, want)
	}
}
