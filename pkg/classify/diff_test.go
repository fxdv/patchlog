package classify

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
)

func TestClassifyWithDiffMigration(t *testing.T) {
	c := commit.Commit{Type: "feat"}
	diff := DiffInfo{
		ChangedFiles: 2,
		Insertions:   50,
		Deletions:    10,
		Files:        []string{"src/api.go", "migrations/001_add_users.sql"},
		FileAnalysis: FileAnalysis{MigrationFiles: 1, GenericFiles: 1, HasMigrations: true},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for migration, got %s", r.Level)
	}
}

func TestClassifyWithDiffLargeByLines(t *testing.T) {
	c := commit.Commit{Type: "feat"}
	diff := DiffInfo{
		ChangedFiles: 3,
		Insertions:   600,
		Deletions:    50,
		Files:        []string{"a.go", "b.go", "c.go"},
		FileAnalysis: FileAnalysis{GenericFiles: 3},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for large-by-lines feature, got %s", r.Level)
	}
}

func TestClassifyWithDiffExcludesGeneratedFiles(t *testing.T) {
	c := commit.Commit{Type: "feat"}
	diff := DiffInfo{
		ChangedFiles: 7,
		Insertions:   100,
		Deletions:    10,
		Files:        []string{"a.go", "b.go", "c.pb.go", "d.pb.go", "e.pb.go", "f.pb.go", "g.pb.go"},
		FileAnalysis: FileAnalysis{GenericFiles: 2, GeneratedFiles: 5},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Minor {
		t.Errorf("expected Minor when generated files excluded (real=2), got %s", r.Level)
	}
}

func TestClassifyWithDiffRevert(t *testing.T) {
	c := commit.Commit{Type: "revert"}
	diff := DiffInfo{ChangedFiles: 3, Files: []string{"a.go", "b.go", "c.go"}}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Patch {
		t.Errorf("expected Patch for revert, got %s", r.Level)
	}
}

func TestClassifyWithDiffPerfLarge(t *testing.T) {
	c := commit.Commit{Type: "perf"}
	diff := DiffInfo{
		ChangedFiles: 8,
		Insertions:   250,
		Deletions:    100,
		Files:        []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go"},
		FileAnalysis: FileAnalysis{GenericFiles: 8},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for large perf, got %s", r.Level)
	}
}

func TestAnalyzeFiles(t *testing.T) {
	files := []string{
		"src/api/handler.go",
		"src/api/handler_test.go",
		"docs/README.md",
		"migrations/001_init.sql",
		"package-lock.json",
		"src/generated/proto.pb.go",
		"config.yaml",
		"src/main.go",
	}
	fa := AnalyzeFiles(files)
	if fa.APIFiles != 1 {
		t.Errorf("APIFiles: got %d, want 1", fa.APIFiles)
	}
	if fa.TestFiles != 1 {
		t.Errorf("TestFiles: got %d, want 1", fa.TestFiles)
	}
	if fa.DocFiles != 1 {
		t.Errorf("DocFiles: got %d, want 1", fa.DocFiles)
	}
	if fa.MigrationFiles != 1 {
		t.Errorf("MigrationFiles: got %d, want 1", fa.MigrationFiles)
	}
	if fa.Lockfiles != 1 {
		t.Errorf("Lockfiles: got %d, want 1", fa.Lockfiles)
	}
	if fa.GeneratedFiles != 1 {
		t.Errorf("GeneratedFiles: got %d, want 1", fa.GeneratedFiles)
	}
	if fa.ConfigFiles != 1 {
		t.Errorf("ConfigFiles: got %d, want 1", fa.ConfigFiles)
	}
	if fa.GenericFiles != 1 {
		t.Errorf("GenericFiles: got %d, want 1", fa.GenericFiles)
	}
	if !fa.HasMigrations {
		t.Error("HasMigrations should be true")
	}
}

func TestClassifyWithDiffBreakingStillMajor(t *testing.T) {
	c := commit.Commit{Type: "feat", Breaking: true}
	diff := DiffInfo{ChangedFiles: 1, Files: []string{"a.go"}}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for breaking, got %s", r.Level)
	}
}

func TestClassifyWithDiffSecurityFeature(t *testing.T) {
	c := commit.Commit{Type: "feat"}
	diff := DiffInfo{
		ChangedFiles: 2,
		Insertions:   50,
		Deletions:    10,
		Files:        []string{"src/auth/login.go", "src/main.go"},
		FileAnalysis: FileAnalysis{SecurityFiles: 1, GenericFiles: 1, HasSecurity: true},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for security feature, got %s", r.Level)
	}
}

func TestClassifyWithDiffSecurityFix(t *testing.T) {
	c := commit.Commit{Type: "fix"}
	diff := DiffInfo{
		ChangedFiles: 1,
		Insertions:   20,
		Deletions:    5,
		Files:        []string{"src/crypto/encrypt.go"},
		FileAnalysis: FileAnalysis{SecurityFiles: 1, HasSecurity: true},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for security fix, got %s", r.Level)
	}
}

func TestClassifyWithDiffPublicAPIChange(t *testing.T) {
	c := commit.Commit{Type: "feat"}
	diff := DiffInfo{
		ChangedFiles: 1,
		Insertions:   30,
		Deletions:    10,
		Files:        []string{"api/openapi.yaml"},
		FileAnalysis: FileAnalysis{PublicAPIFiles: 1, HasPublicAPI: true},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for public API change, got %s", r.Level)
	}
}

func TestClassifyWithDiffDeletionHeavyRefactor(t *testing.T) {
	c := commit.Commit{Type: "refactor"}
	diff := DiffInfo{
		ChangedFiles: 5,
		Insertions:   20,
		Deletions:    200,
		Files:        []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
		FileAnalysis: FileAnalysis{GenericFiles: 5},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Major {
		t.Errorf("expected Major for deletion-heavy refactor, got %s", r.Level)
	}
}

func TestClassifyWithDiffChoreDeployment(t *testing.T) {
	c := commit.Commit{Type: "chore"}
	diff := DiffInfo{
		ChangedFiles: 2,
		Insertions:   30,
		Deletions:    10,
		Files:        []string{"Dockerfile", "docker-compose.yml"},
		FileAnalysis: FileAnalysis{DeploymentFiles: 2, HasDeployment: true},
	}
	r := ClassifyWithDiff(c, diff, DefaultThresholds())
	if r.Level != Patch {
		t.Errorf("expected Patch for deployment chore, got %s", r.Level)
	}
	if r.Reason != "deployment config change" {
		t.Errorf("expected deployment reason, got %s", r.Reason)
	}
}

func TestAnalyzeFilesSecurityAndDeployment(t *testing.T) {
	files := []string{
		"src/auth/login.go",
		"src/crypto/hash.go",
		"api/openapi.yaml",
		"Dockerfile",
		"src/handlers/routes.go",
	}
	fa := AnalyzeFiles(files)
	if fa.SecurityFiles != 2 {
		t.Errorf("SecurityFiles: got %d, want 2", fa.SecurityFiles)
	}
	if !fa.HasSecurity {
		t.Error("HasSecurity should be true")
	}
	if fa.PublicAPIFiles != 1 {
		t.Errorf("PublicAPIFiles: got %d, want 1", fa.PublicAPIFiles)
	}
	if !fa.HasPublicAPI {
		t.Error("HasPublicAPI should be true")
	}
	if fa.DeploymentFiles != 1 {
		t.Errorf("DeploymentFiles: got %d, want 1", fa.DeploymentFiles)
	}
	if !fa.HasDeployment {
		t.Error("HasDeployment should be true")
	}
}
