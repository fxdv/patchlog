package classify

import (
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
)

type FileCategory int

const (
	FileCategoryGeneric FileCategory = iota
	FileCategoryAPI
	FileCategoryTest
	FileCategoryDocs
	FileCategoryConfig
	FileCategoryMigration
	FileCategoryGenerated
	FileCategoryLockfile
	FileCategorySecurity
	FileCategoryPublicAPI
	FileCategoryDeployment
)

func categorizeFile(path string) FileCategory {
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, ".lock") ||
		strings.HasSuffix(lower, "package-lock.json") || strings.HasSuffix(lower, "yarn.lock") ||
		strings.HasSuffix(lower, "go.sum") || strings.HasSuffix(lower, "cargo.lock") ||
		strings.HasSuffix(lower, "poetry.lock") || strings.HasSuffix(lower, "composer.lock") ||
		strings.HasSuffix(lower, "pnpm-lock.yaml") || strings.HasSuffix(lower, "gemfile.lock") {
		return FileCategoryLockfile
	}

	if strings.Contains(lower, "migration") || strings.Contains(lower, "migrations") ||
		strings.HasSuffix(lower, ".sql") && (strings.Contains(lower, "migration") || strings.Contains(lower, "schema")) {
		return FileCategoryMigration
	}

	if strings.HasSuffix(lower, "openapi.yaml") || strings.HasSuffix(lower, "openapi.json") ||
		strings.HasSuffix(lower, "swagger.json") || strings.HasSuffix(lower, "swagger.yaml") ||
		strings.HasSuffix(lower, ".proto") || strings.HasSuffix(lower, ".graphql") ||
		strings.HasSuffix(lower, ".gql") ||
		strings.Contains(lower, "/public/api/") ||
		strings.Contains(lower, "/api/contract/") ||
		strings.Contains(lower, "/api/v1/") || strings.Contains(lower, "/api/v2/") {
		return FileCategoryPublicAPI
	}

	if strings.Contains(lower, "auth") || strings.Contains(lower, "crypto") ||
		strings.Contains(lower, "security") || strings.Contains(lower, "password") ||
		strings.Contains(lower, "session") || strings.Contains(lower, "token") ||
		strings.Contains(lower, "permission") || strings.Contains(lower, "rbac") ||
		strings.Contains(lower, "acl") || strings.Contains(lower, "secret") ||
		strings.Contains(lower, "/ssl/") || strings.Contains(lower, "/tls/") ||
		strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") ||
		strings.HasSuffix(lower, ".crt") || strings.HasSuffix(lower, ".cer") {
		return FileCategorySecurity
	}

	if strings.Contains(lower, "dockerfile") || strings.Contains(lower, "docker-compose") ||
		strings.HasSuffix(lower, ".dockerfile") ||
		strings.Contains(lower, "/k8s/") || strings.Contains(lower, "/kubernetes/") ||
		strings.HasSuffix(lower, ".yaml") && (strings.Contains(lower, "deploy") ||
			strings.Contains(lower, "manifest") || strings.Contains(lower, "helm")) ||
		strings.Contains(lower, "/helm/") || strings.Contains(lower, "/charts/") ||
		strings.Contains(lower, "/terraform/") || strings.Contains(lower, ".tf") ||
		strings.Contains(lower, "/.github/workflows/") ||
		strings.Contains(lower, "/.gitlab-ci.") ||
		strings.HasSuffix(lower, "makefile") {
		return FileCategoryDeployment
	}

	if strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, "_test.py") ||
		strings.HasSuffix(lower, ".test.js") || strings.HasSuffix(lower, ".test.ts") ||
		strings.HasSuffix(lower, ".spec.js") || strings.HasSuffix(lower, ".spec.ts") ||
		strings.HasSuffix(lower, ".test.jsx") || strings.HasSuffix(lower, ".test.tsx") ||
		strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/__tests__/") || strings.Contains(lower, "/testdata/") {
		return FileCategoryTest
	}

	if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".rst") ||
		strings.HasSuffix(lower, ".txt") && strings.Contains(lower, "doc") ||
		strings.Contains(lower, "/docs/") || strings.Contains(lower, "/doc/") {
		return FileCategoryDocs
	}

	if strings.HasSuffix(lower, ".json") && (strings.Contains(lower, "config") ||
		strings.Contains(lower, "settings")) ||
		strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".toml") || strings.HasSuffix(lower, ".ini") ||
		strings.HasSuffix(lower, ".env") {
		return FileCategoryConfig
	}

	if strings.Contains(lower, ".generated.") || strings.Contains(lower, ".gen.") ||
		strings.Contains(lower, ".pb.go") || strings.Contains(lower, ".pb.py") ||
		strings.Contains(lower, "_generated_") || strings.Contains(lower, "/generated/") ||
		strings.Contains(lower, "/vendor/") || strings.Contains(lower, "/third_party/") {
		return FileCategoryGenerated
	}

	if strings.Contains(lower, "/api/") || strings.Contains(lower, "/handler") ||
		strings.Contains(lower, "/controller") || strings.Contains(lower, "/route") ||
		strings.Contains(lower, "/endpoint") || strings.Contains(lower, "/public/") ||
		strings.Contains(lower, "/openapi") {
		return FileCategoryAPI
	}

	return FileCategoryGeneric
}

type FileAnalysis struct {
	APIFiles        int
	TestFiles       int
	DocFiles        int
	ConfigFiles     int
	MigrationFiles  int
	GeneratedFiles  int
	Lockfiles       int
	GenericFiles    int
	SecurityFiles   int
	PublicAPIFiles  int
	DeploymentFiles int
	HasMigrations   bool
	HasSecurity     bool
	HasPublicAPI    bool
	HasDeployment   bool
}

func AnalyzeFiles(files []string) FileAnalysis {
	var fa FileAnalysis
	for _, f := range files {
		switch categorizeFile(f) {
		case FileCategoryAPI:
			fa.APIFiles++
		case FileCategoryTest:
			fa.TestFiles++
		case FileCategoryDocs:
			fa.DocFiles++
		case FileCategoryConfig:
			fa.ConfigFiles++
		case FileCategoryMigration:
			fa.MigrationFiles++
			fa.HasMigrations = true
		case FileCategoryGenerated:
			fa.GeneratedFiles++
		case FileCategoryLockfile:
			fa.Lockfiles++
		case FileCategorySecurity:
			fa.SecurityFiles++
			fa.HasSecurity = true
		case FileCategoryPublicAPI:
			fa.PublicAPIFiles++
			fa.HasPublicAPI = true
		case FileCategoryDeployment:
			fa.DeploymentFiles++
			fa.HasDeployment = true
		default:
			fa.GenericFiles++
		}
	}
	return fa
}

type DiffInfo struct {
	ChangedFiles int
	Insertions   int
	Deletions    int
	Files        []string
	FileAnalysis FileAnalysis
}

func ClassifyWithDiff(c commit.Commit, diff DiffInfo, th Thresholds) Result {
	if c.Breaking {
		return Result{Major, "breaking change", diff.ChangedFiles}
	}

	if diff.FileAnalysis.HasMigrations && c.Type != "docs" && c.Type != "test" {
		return Result{Major, "database migration detected", diff.ChangedFiles}
	}

	realChanged := diff.ChangedFiles - diff.FileAnalysis.GeneratedFiles - diff.FileAnalysis.Lockfiles
	if realChanged < 0 {
		realChanged = 0
	}

	totalLines := diff.Insertions + diff.Deletions
	deletionRatio := 0.0
	if totalLines > 0 {
		deletionRatio = float64(diff.Deletions) / float64(totalLines)
	}

	coreChanged := realChanged - diff.FileAnalysis.TestFiles - diff.FileAnalysis.DocFiles
	if coreChanged < 0 {
		coreChanged = 0
	}

	switch c.Type {
	case "feat":
		if diff.FileAnalysis.HasSecurity {
			return Result{Major, "security-related feature (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		if diff.FileAnalysis.HasPublicAPI {
			return Result{Major, "public API surface change (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		if realChanged >= th.LargeFeatureFiles || totalLines > 500 {
			return Result{Major, "large feature (" + plural(diff.ChangedFiles, "file") + ", " + itoa(totalLines) + " lines)", diff.ChangedFiles}
		}
		return Result{Minor, "new feature", diff.ChangedFiles}
	case "fix":
		if diff.FileAnalysis.HasSecurity {
			return Result{Major, "security fix (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		if realChanged >= th.LargeFixFiles || totalLines > 300 {
			return Result{Minor, "large fix (" + plural(diff.ChangedFiles, "file") + ", " + itoa(totalLines) + " lines)", diff.ChangedFiles}
		}
		return Result{Patch, "bug fix", diff.ChangedFiles}
	case "perf":
		if totalLines > 200 || realChanged >= th.LargeFeatureFiles {
			return Result{Major, "significant performance change (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		return Result{Minor, "performance improvement", diff.ChangedFiles}
	case "refactor":
		if deletionRatio > 0.6 && totalLines > 100 {
			return Result{Major, "deletion-heavy refactor (" + plural(diff.ChangedFiles, "file") + ", " + itoa(totalLines) + " lines)", diff.ChangedFiles}
		}
		if totalLines > 500 || realChanged >= th.LargeFeatureFiles {
			return Result{Minor, "large refactoring (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		return Result{Patch, "code refactoring", diff.ChangedFiles}
	case "revert":
		return Result{Patch, "revert", diff.ChangedFiles}
	case "docs":
		return Result{Skip, "documentation", diff.ChangedFiles}
	case "test":
		return Result{Skip, "tests", diff.ChangedFiles}
	case "style":
		return Result{Skip, "style/formatting", diff.ChangedFiles}
	case "ci":
		return Result{Skip, "CI/build", diff.ChangedFiles}
	case "chore":
		if diff.FileAnalysis.HasDeployment && coreChanged > 0 {
			return Result{Patch, "deployment config change", diff.ChangedFiles}
		}
		return Result{Skip, "maintenance", diff.ChangedFiles}
	default:
		if diff.FileAnalysis.HasSecurity {
			return Result{Major, "security-related change (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		if diff.FileAnalysis.HasPublicAPI {
			return Result{Major, "public API surface change (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		if realChanged >= th.LargeUnknownFiles || totalLines > 300 {
			return Result{Minor, "significant change (" + plural(diff.ChangedFiles, "file") + ")", diff.ChangedFiles}
		}
		return Result{Patch, "other change", diff.ChangedFiles}
	}
}

func itoa(n int) string {
	return plural(n, "line")
}
