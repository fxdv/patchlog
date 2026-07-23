package config

import (
	"bytes"
	"errors"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration loaded from patchlog.yaml.
type Config struct {
	Sections   map[string]string `yaml:"sections"`
	Ignore     []string          `yaml:"ignore"`
	Author     AuthorConfig      `yaml:"author"`
	Links      LinksConfig       `yaml:"links"`
	Repo       string            `yaml:"repo"`
	AI         AIConfig          `yaml:"ai"`
	Provider   ProviderConfig    `yaml:"provider"`
	Release    ReleaseConfig     `yaml:"release"`
	Bump       BumpConfig        `yaml:"bump"`
	Jira       JiraConfig        `yaml:"jira"`
	Confluence ConfluenceConfig  `yaml:"confluence"`
	Classify   ClassifyConfig    `yaml:"classify"`
	Changelog  ChangelogConfig   `yaml:"changelog"`
	Theme      ThemeConfig       `yaml:"theme"`
	Trends     TrendsConfig      `yaml:"trends"`
	Language   LanguageConfig    `yaml:"language"`
	Gate       GateConfig        `yaml:"gate"`
	Deps       DepsConfig        `yaml:"deps"`
	Infer      InferConfig       `yaml:"infer"`
	Semantic   SemanticConfig    `yaml:"semantic"`
	Drift      DriftConfig       `yaml:"drift"`
	Security   SecurityConfig    `yaml:"security"`
}

// SecurityConfig controls explicit opt-ins for unsafe integration behavior.
type SecurityConfig struct {
	AllowInsecureCredentials bool `yaml:"allow_insecure_credentials"`
}

// ReleaseConfig defines the protected-branch release lifecycle. Protected mode
// is the stable product direction; direct commit/tag/push remains explicit.
type ReleaseConfig struct {
	ProtectedBranch string `yaml:"protected_branch"`
	BranchPrefix    string `yaml:"branch_prefix"`
	TagPrefix       string `yaml:"tag_prefix"`
}

// DriftConfig controls plan-vs-actual Jira ticket comparison.
type DriftConfig struct {
	Enabled    bool   `yaml:"enabled"`
	FixVersion string `yaml:"fix_version"`
}

// LanguageConfig controls output language localization.
type LanguageConfig struct {
	Default           string `yaml:"default"`
	TranslateHeadings bool   `yaml:"translate_headings"`
}

// GateConfig defines CI quality gate thresholds. Zero values disable the check.
type GateConfig struct {
	Enabled              bool    `yaml:"enabled"`
	MinConventionalRatio float64 `yaml:"min_conventional_ratio"`
}

// DepsConfig controls dependency changelog detection and upstream fetching.
type DepsConfig struct {
	Enabled         bool              `yaml:"enabled"`
	FetchUpstream   bool              `yaml:"fetch_upstream"`
	Registries      map[string]string `yaml:"registries"`
	MaxDependencies int               `yaml:"max_dependencies"`
	GitHubReleases  bool              `yaml:"github_releases"`
}

// InferConfig controls AI-based commit type inference for uncategorized commits.
type InferConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Threshold float64  `yaml:"threshold"`
	Types     []string `yaml:"types"`
}

// SemanticConfig controls AI-based semantic diff summaries.
type SemanticConfig struct {
	Enabled        bool `yaml:"enabled"`
	Aggregate      bool `yaml:"aggregate"`
	MaxDiffChars   int  `yaml:"max_diff_chars"`
	StripGenerated bool `yaml:"strip_generated"`
}

// ThemeConfig controls AI-based thematic commit grouping.
type ThemeConfig struct {
	Enabled          bool `yaml:"enabled"`
	MinThemes        int  `yaml:"min_themes"`
	MaxThemes        int  `yaml:"max_themes"`
	IncludeNarrative bool `yaml:"include_narrative"`
}

// TrendsConfig controls cross-release trend snapshot storage and display.
type TrendsConfig struct {
	Store      bool             `yaml:"store"`
	Count      int              `yaml:"count"`
	Title      string           `yaml:"title"`
	Thresholds TrendsThresholds `yaml:"thresholds"`
}

// TrendsThresholds defines warning/critical levels for trend color-coding.
type TrendsThresholds struct {
	ReleaseCommitSpanWarning            float64 `yaml:"release_commit_span_warning"`
	ReleaseCommitSpanCritical           float64 `yaml:"release_commit_span_critical"`
	TechDebtWarning                     float64 `yaml:"tech_debt_warning"`
	TechDebtCritical                    float64 `yaml:"tech_debt_critical"`
	ReleaseContributionConcentrationMin int     `yaml:"release_contribution_concentration_min"`
}

// AuthorConfig controls author attribution in output.
type AuthorConfig struct {
	Show bool `yaml:"show"`
}

// LinksConfig defines URL templates for commit, issue, and compare links.
// %s placeholders: commit/issue → (repo, ref); compare → (repo, from, to).
type LinksConfig struct {
	Commit  string `yaml:"commit"`
	Issue   string `yaml:"issue"`
	Compare string `yaml:"compare"`
}

// AIConfig configures the AI provider for prose generation and enhancement.
type AIConfig struct {
	Provider      string   `yaml:"provider"`
	Model         string   `yaml:"model"`
	APIKey        string   `yaml:"api_key"`
	BaseURL       string   `yaml:"base_url"`
	MaxTokens     int      `yaml:"max_tokens"`
	MaxInputChars int      `yaml:"max_input_chars"`
	ExcludeFiles  []string `yaml:"exclude_files"`
}

// ProviderConfig configures the git hosting provider (GitHub, GitLab, Gitea).
type ProviderConfig struct {
	Type    string `yaml:"type"`
	Token   string `yaml:"token"`
	Repo    string `yaml:"repo"`
	BaseURL string `yaml:"base_url"`
	Draft   *bool  `yaml:"draft"`
}

// BumpConfig controls version file bumping behavior.
type BumpConfig struct {
	AutoDetect bool     `yaml:"auto_detect"`
	Files      []string `yaml:"files"`
}

// JiraConfig configures Jira Cloud/Server integration for ticket enrichment.
type JiraConfig struct {
	BaseURL        string `yaml:"base_url"`
	Email          string `yaml:"email"`
	APIToken       string `yaml:"api_token"`
	ProjectKey     string `yaml:"project_key"`
	MaxConcurrency int    `yaml:"max_concurrency"`
}

// ConfluenceConfig configures Confluence Cloud/Server page publishing.
type ConfluenceConfig struct {
	BaseURL         string   `yaml:"base_url"`
	Email           string   `yaml:"email"`
	APIToken        string   `yaml:"api_token"`
	SpaceKey        string   `yaml:"space_key"`
	ParentPageID    string   `yaml:"parent_page_id"`
	Labels          []string `yaml:"labels"`
	ViewRestriction []string `yaml:"view_restriction"`
	EditRestriction []string `yaml:"edit_restriction"`
	Template        string   `yaml:"template"`
}

// ClassifyConfig sets file-count thresholds for significance classification.
type ClassifyConfig struct {
	LargeFeatureFiles int `yaml:"large_feature_files"`
	LargeFixFiles     int `yaml:"large_fix_files"`
	LargeUnknownFiles int `yaml:"large_unknown_files"`
}

// ChangelogConfig controls changelog accumulation behavior.
// Destination: "md" (file), "wiki" (GitLab), "confluence".
type ChangelogConfig struct {
	Accumulate  bool   `yaml:"accumulate"`
	Destination string `yaml:"destination"`
	File        string `yaml:"file"`
	Title       string `yaml:"title"`
	Slug        string `yaml:"slug"`
	Emojis      *bool  `yaml:"emojis"`
}

func Default() Config {
	return Config{
		Sections: map[string]string{
			"feat":     "Features",
			"fix":      "Bug Fixes",
			"perf":     "Performance Improvements",
			"refactor": "Code Refactoring",
			"docs":     "Documentation",
			"test":     "Tests",
			"style":    "Style / Formatting",
			"ci":       "CI / Build",
			"chore":    "Chores",
		},
		Author: AuthorConfig{
			Show: true,
		},
		Links: LinksConfig{
			Commit:  "https://github.com/%s/commit/%s",
			Issue:   "https://github.com/%s/issues/%s",
			Compare: "https://github.com/%s/compare/%s...%s",
		},
		AI: AIConfig{
			Provider:      "ollama",
			MaxInputChars: 120000,
			ExcludeFiles: []string{
				".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx",
				"**/secrets.*", "**/vendor/**", "**/generated/**",
			},
		},
		Bump: BumpConfig{
			AutoDetect: true,
		},
		Release: ReleaseConfig{
			ProtectedBranch: "main",
			BranchPrefix:    "release/",
			TagPrefix:       "v",
		},
		Classify: ClassifyConfig{
			LargeFeatureFiles: 5,
			LargeFixFiles:     5,
			LargeUnknownFiles: 3,
		},
		Changelog: ChangelogConfig{
			Accumulate:  false,
			Destination: "md",
			File:        "CHANGELOG.md",
			Title:       "Changelog",
			Slug:        "changelog",
		},
		Theme: ThemeConfig{
			MinThemes:        3,
			MaxThemes:        7,
			IncludeNarrative: true,
		},
		Trends: TrendsConfig{
			Store: false,
			Count: 5,
			Title: "Release Trends",
			Thresholds: TrendsThresholds{
				ReleaseCommitSpanWarning:            500,
				ReleaseCommitSpanCritical:           700,
				TechDebtWarning:                     3000,
				TechDebtCritical:                    5000,
				ReleaseContributionConcentrationMin: 3,
			},
		},
		Language: LanguageConfig{
			Default:           "en",
			TranslateHeadings: true,
		},
		Gate: GateConfig{
			MinConventionalRatio: 0.0,
		},
		Deps: DepsConfig{
			FetchUpstream:   true,
			MaxDependencies: 10,
			GitHubReleases:  true,
			Registries: map[string]string{
				"npm":    "https://registry.npmjs.org",
				"crates": "https://crates.io",
				"pypi":   "https://pypi.org",
			},
		},
		Infer: InferConfig{
			Threshold: 0.6,
		},
		Semantic: SemanticConfig{
			Aggregate:      true,
			MaxDiffChars:   4000,
			StripGenerated: true,
		},
		Drift: DriftConfig{},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return cfg, errors.New("configuration must contain exactly one YAML document")
	} else if !errors.Is(err, io.EOF) {
		return cfg, err
	}
	return cfg, nil
}

func LoadValidated(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
