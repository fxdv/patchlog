package deps

import (
	"strings"
	"testing"
)

func TestDetectNPM(t *testing.T) {
	diff := `diff --git a/package.json b/package.json
--- a/package.json
+++ b/package.json
@@ -10,7 +10,7 @@
     "dependencies": {
-    "react": "^18.2.0",
+    "react": "^18.3.1",
     "lodash": "^4.17.21",
-    "express": "^4.18.0"
+    "express": "^4.18.2"
     }
`
	changes := Detect("package.json", diff)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	byName := make(map[string]Change)
	for _, c := range changes {
		byName[c.Name] = c
	}

	r, ok := byName["react"]
	if !ok {
		t.Fatal("expected react change")
	}
	if r.OldVersion != "^18.2.0" || r.NewVersion != "^18.3.1" {
		t.Errorf("react: expected ^18.2.0 → ^18.3.1, got %s → %s", r.OldVersion, r.NewVersion)
	}
	if r.Ecosystem != EcosystemNPM {
		t.Errorf("expected ecosystem npm, got %s", r.Ecosystem)
	}

	e, ok := byName["express"]
	if !ok {
		t.Fatal("expected express change")
	}
	if e.OldVersion != "^4.18.0" || e.NewVersion != "^4.18.2" {
		t.Errorf("express: expected ^4.18.0 → ^4.18.2, got %s → %s", e.OldVersion, e.NewVersion)
	}
}

func TestDetectNPMNewDependency(t *testing.T) {
	diff := `--- a/package.json
+++ b/package.json
@@ -10,6 +10,7 @@
     "dependencies": {
     "lodash": "^4.17.21",
+    "axios": "^1.6.0",
     }
`
	changes := Detect("package.json", diff)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "axios" {
		t.Errorf("expected axios, got %s", changes[0].Name)
	}
	if changes[0].OldVersion != "" {
		t.Errorf("expected empty old version, got %s", changes[0].OldVersion)
	}
	if changes[0].NewVersion != "^1.6.0" {
		t.Errorf("expected ^1.6.0, got %s", changes[0].NewVersion)
	}
}

func TestDetectNPMSkipsNonDependencies(t *testing.T) {
	diff := `--- a/package.json
+++ b/package.json
@@ -1,5 +1,5 @@
-  "version": "1.0.0",
+  "version": "1.1.0",
-  "name": "myapp",
+  "name": "myapp2",
`
	changes := Detect("package.json", diff)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes (version/name are not deps), got %d: %v", len(changes), changes)
	}
}

func TestDetectNPMNoVersionMatch(t *testing.T) {
	diff := `--- a/package.json
+++ b/package.json
@@ -5,5 +5,5 @@
-  "scripts": "webpack",
+  "scripts": "vite",
`
	changes := Detect("package.json", diff)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes, got %d: %v", len(changes), changes)
	}
}

func TestDetectCargo(t *testing.T) {
	diff := `--- a/Cargo.toml
+++ b/Cargo.toml
@@ -10,7 +10,7 @@
 [dependencies]
-tokio = "1.34.0"
+tokio = "1.35.1"
 serde = "1.0"
-serde_json = "1.0.100"
+serde_json = "1.0.108"
`
	changes := Detect("Cargo.toml", diff)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	byName := make(map[string]Change)
	for _, c := range changes {
		byName[c.Name] = c
	}

	if c, ok := byName["tokio"]; !ok || c.OldVersion != "1.34.0" || c.NewVersion != "1.35.1" {
		t.Errorf("tokio mismatch: %+v", byName["tokio"])
	}
	if c, ok := byName["serde_json"]; !ok || c.OldVersion != "1.0.100" || c.NewVersion != "1.0.108" {
		t.Errorf("serde_json mismatch: %+v", byName["serde_json"])
	}
}

func TestDetectCargoTableForm(t *testing.T) {
	diff := `--- a/Cargo.toml
+++ b/Cargo.toml
@@ -8,5 +8,5 @@
 [dependencies]
-tokio = { version = "1.34.0", features = ["full"] }
+tokio = { version = "1.35.1", features = ["full"] }
`
	changes := Detect("Cargo.toml", diff)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Name != "tokio" {
		t.Errorf("expected tokio, got %s", changes[0].Name)
	}
	if changes[0].OldVersion != "1.34.0" || changes[0].NewVersion != "1.35.1" {
		t.Errorf("expected 1.34.0 → 1.35.1, got %s → %s", changes[0].OldVersion, changes[0].NewVersion)
	}
}

func TestDetectGoMod(t *testing.T) {
	diff := `--- a/go.mod
+++ b/go.mod
@@ -5,7 +5,7 @@
 require (
-	github.com/gin-gonic/gin v1.9.0 // indirect
+	github.com/gin-gonic/gin v1.9.1 // indirect
-	github.com/stretchr/testify v1.8.0
+	github.com/stretchr/testify v1.8.1
 )
`
	changes := Detect("go.mod", diff)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	byName := make(map[string]Change)
	for _, c := range changes {
		byName[c.Name] = c
	}

	if c, ok := byName["github.com/gin-gonic/gin"]; !ok || c.OldVersion != "v1.9.0" || c.NewVersion != "v1.9.1" {
		t.Errorf("gin mismatch: %+v", byName["github.com/gin-gonic/gin"])
	}
	if c, ok := byName["github.com/stretchr/testify"]; !ok || c.OldVersion != "v1.8.0" || c.NewVersion != "v1.8.1" {
		t.Errorf("testify mismatch: %+v", byName["github.com/stretchr/testify"])
	}
}

func TestDetectGoModSkipsDirectives(t *testing.T) {
	diff := `--- a/go.mod
+++ b/go.mod
@@ -1,5 +1,5 @@
-module github.com/myorg/myapp
+module github.com/myorg/myapp2
-go 1.21
+go 1.22
`
	changes := Detect("go.mod", diff)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes (module/go directives), got %d: %v", len(changes), changes)
	}
}

func TestDetectRequirements(t *testing.T) {
	diff := `--- a/requirements.txt
+++ b/requirements.txt
@@ -1,3 +1,3 @@
-django==4.2.0
+django==4.2.1
-flask==2.3.0
+flask==2.3.2
`
	changes := Detect("requirements.txt", diff)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	byName := make(map[string]Change)
	for _, c := range changes {
		byName[c.Name] = c
	}

	if c, ok := byName["django"]; !ok || c.OldVersion != "4.2.0" || c.NewVersion != "4.2.1" {
		t.Errorf("django mismatch: %+v", byName["django"])
	}
}

func TestDetectPyProjectTOML(t *testing.T) {
	diff := `--- a/pyproject.toml
+++ b/pyproject.toml
@@ -10,5 +10,5 @@
 [project]
-django = "^4.2.0"
+django = "^4.2.1"
`
	changes := Detect("pyproject.toml", diff)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Name != "django" {
		t.Errorf("expected django, got %s", changes[0].Name)
	}
}

func TestDetectAll(t *testing.T) {
	diffs := map[string]string{
		"package.json": `-    "react": "^18.2.0",
+    "react": "^18.3.1",
`,
		"go.mod": `-	github.com/gin-gonic/gin v1.9.0
+	github.com/gin-gonic/gin v1.9.1
`,
	}
	changes := DetectAll(diffs)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	ecosystems := make(map[Ecosystem]bool)
	for _, c := range changes {
		ecosystems[c.Ecosystem] = true
	}
	if !ecosystems[EcosystemNPM] || !ecosystems[EcosystemGo] {
		t.Errorf("expected npm and go ecosystems, got %v", ecosystems)
	}
}

func TestDetectEmptyDiff(t *testing.T) {
	changes := Detect("package.json", "")
	if changes != nil {
		t.Fatalf("expected nil for empty diff, got %v", changes)
	}
}

func TestDetectNonManifestFile(t *testing.T) {
	changes := Detect("README.md", "some diff")
	if changes != nil {
		t.Fatalf("expected nil for non-manifest file, got %v", changes)
	}
}

func TestDetectNoChange(t *testing.T) {
	diff := `--- a/package.json
+++ b/package.json
@@ -10,3 +10,3 @@
     "dependencies": {
     "react": "^18.2.0",
     }
`
	changes := Detect("package.json", diff)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes (no version change), got %d", len(changes))
	}
}

func TestIsManifestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"package.json", true},
		{"src/package.json", true},
		{"Cargo.toml", true},
		{"go.mod", true},
		{"pyproject.toml", true},
		{"requirements.txt", true},
		{"README.md", false},
		{"main.go", false},
		{"subdir/go.mod", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsManifestFile(tt.path); got != tt.expected {
				t.Errorf("IsManifestFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestManifestFiles(t *testing.T) {
	files := ManifestFiles()
	if len(files) == 0 {
		t.Fatal("expected non-empty manifest file list")
	}
	found := make(map[string]bool)
	for _, f := range files {
		found[f] = true
	}
	expected := []string{"package.json", "Cargo.toml", "go.mod", "pyproject.toml", "requirements.txt"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected %s in ManifestFiles()", e)
		}
	}
}

func TestStripVersionPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^18.2.0", "18.2.0"},
		{"~1.2.3", "1.2.3"},
		{">=1.0.0", "1.0.0"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"==4.2.0", "4.2.0"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := stripVersionPrefix(tt.input); got != tt.expected {
				t.Errorf("stripVersionPrefix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "2.0.0", -1},
		{"v1.2.3", "v1.2.4", -1},
		{"^1.0.0", "1.0.1", -1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			if got := compareVersions(tt.a, tt.b); got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestIsVersionBetween(t *testing.T) {
	tests := []struct {
		tag, old, new string
		expected      bool
	}{
		{"v1.2.4", "1.2.3", "1.2.5", true},
		{"v1.2.3", "1.2.3", "1.2.5", false},
		{"v1.2.5", "1.2.3", "1.2.5", true},
		{"v1.2.6", "1.2.3", "1.2.5", false},
		{"v1.3.0", "1.2.0", "1.4.0", true},
		{"v18.3.1", "18.2.0", "18.3.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			if got := isVersionBetween(tt.tag, tt.old, tt.new); got != tt.expected {
				t.Errorf("isVersionBetween(%q, %q, %q) = %v, want %v", tt.tag, tt.old, tt.new, got, tt.expected)
			}
		})
	}
}

func TestFormatMarkdown(t *testing.T) {
	changes := []Change{
		{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: EcosystemNPM, Manifest: "package.json"},
		{Name: "axios", OldVersion: "", NewVersion: "^1.6.0", Ecosystem: EcosystemNPM, Manifest: "package.json", ChangelogURL: "https://www.npmjs.com/package/axios/v/1.6.0"},
	}
	out := FormatMarkdown(changes)
	if out == "" {
		t.Fatal("expected non-empty markdown")
	}
	if !strings.Contains(out, "## Dependencies") {
		t.Error("should contain Dependencies heading")
	}
	if !strings.Contains(out, "react") {
		t.Error("should contain react")
	}
	if !strings.Contains(out, "^18.2.0 → ^18.3.1") {
		t.Error("should contain version arrow")
	}
	if !strings.Contains(out, "axios") {
		t.Error("should contain axios")
	}
	if !strings.Contains(out, "^1.6.0") {
		t.Error("should contain new version for added dep")
	}
	if !strings.Contains(out, "https://www.npmjs.com/package/axios/v/1.6.0") {
		t.Error("should contain changelog URL")
	}
}

func TestFormatMarkdownWithChangelog(t *testing.T) {
	changes := []Change{
		{Name: "react", OldVersion: "^18.2.0", NewVersion: "^18.3.1", Ecosystem: EcosystemNPM, Manifest: "package.json", Changelog: "Removed legacy defaultProps"},
	}
	out := FormatMarkdown(changes)
	if !strings.Contains(out, "<details>") {
		t.Error("should contain details tag for changelog")
	}
	if !strings.Contains(out, "Removed legacy defaultProps") {
		t.Error("should contain changelog text")
	}
}

func TestFormatMarkdownEmpty(t *testing.T) {
	out := FormatMarkdown(nil)
	if out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

func TestParseGitHubRepo(t *testing.T) {
	tests := []struct {
		url   string
		owner string
		repo  string
	}{
		{"https://github.com/facebook/react.git", "facebook", "react"},
		{"git://github.com/facebook/react.git", "facebook", "react"},
		{"git+https://github.com/facebook/react.git", "facebook", "react"},
		{"github.com/facebook/react", "facebook", "react"},
		{"https://github.com/facebook/react", "facebook", "react"},
		{"https://gitlab.com/owner/repo", "", ""},
		{"not a url", "", ""},
		{"https://github.com/onlyone", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			owner, repo := parseGitHubRepo(tt.url)
			if owner != tt.owner || repo != tt.repo {
				t.Errorf("parseGitHubRepo(%q) = (%q, %q), want (%q, %q)", tt.url, owner, repo, tt.owner, tt.repo)
			}
		})
	}
}

func TestSetChangelogURLs(t *testing.T) {
	changes := []Change{
		{Name: "react", NewVersion: "^18.3.1", Ecosystem: EcosystemNPM},
		{Name: "tokio", NewVersion: "1.35.1", Ecosystem: EcosystemCargo},
		{Name: "django", NewVersion: "4.2.1", Ecosystem: EcosystemPyPI},
		{Name: "github.com/gin-gonic/gin", NewVersion: "v1.9.1", Ecosystem: EcosystemGo},
	}
	SetChangelogURLs(changes)

	if !strings.Contains(changes[0].ChangelogURL, "npmjs.com/package/react") {
		t.Errorf("npm URL wrong: %s", changes[0].ChangelogURL)
	}
	if !strings.Contains(changes[1].ChangelogURL, "crates.io/crates/tokio") {
		t.Errorf("crates URL wrong: %s", changes[1].ChangelogURL)
	}
	if !strings.Contains(changes[2].ChangelogURL, "pypi.org/project/django") {
		t.Errorf("pypi URL wrong: %s", changes[2].ChangelogURL)
	}
	if !strings.Contains(changes[3].ChangelogURL, "pkg.go.dev/github.com/gin-gonic/gin") {
		t.Errorf("go URL wrong: %s", changes[3].ChangelogURL)
	}
}
