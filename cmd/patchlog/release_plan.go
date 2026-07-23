package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/confluence"
	"github.com/fxdv/patchlog/pkg/gittag"
	"github.com/fxdv/patchlog/pkg/gitwiki"
	"github.com/fxdv/patchlog/pkg/provider"
)

// RemoteReleaseRef is an immutable provider release identity. A provider
// release is never created from HEAD or the display label "Unreleased".
type RemoteReleaseRef struct {
	tag string
}

func newRemoteReleaseRef(tag string) (RemoteReleaseRef, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.EqualFold(tag, "unreleased") || tag == "HEAD" {
		return RemoteReleaseRef{}, fmt.Errorf("provider publishing requires an immutable release tag")
	}
	if strings.ContainsAny(tag, " \t\r\n") {
		return RemoteReleaseRef{}, fmt.Errorf("release tag %q contains whitespace", tag)
	}
	return RemoteReleaseRef{tag: tag}, nil
}

func (r RemoteReleaseRef) Tag() string { return r.tag }

type ReleasePlanRequest struct {
	Repo              string
	Bump              *bump.Plan
	TagName           string
	TagOptions        gittag.Options
	Publish           bool
	Confluence        bool
	Changelog         bool
	Trends            bool
	HTMLPath          string
	OutputPath        string
	TrendSnapshotPath string
	Configuration     config.Config
}

// ReleasePlan is the single immutable description of every requested release
// mutation. All fields are private, mutable inputs are cloned, and construction
// performs every deterministic local and remote preflight before Apply.
type ReleasePlan struct {
	repo              string
	head              string
	bump              *bump.Plan
	tagName           string
	tagOptions        gittag.Options
	remoteRef         *RemoteReleaseRef
	trendSnapshotPath string
	actions           []string
	manager           *gittag.Manager
}

func NewReleasePlan(ctx context.Context, req ReleasePlanRequest) (*ReleasePlan, error) {
	repo, err := filepath.Abs(req.Repo)
	if err != nil {
		return nil, fmt.Errorf("resolve release repository: %w", err)
	}
	manager := &gittag.Manager{RepoPath: repo}
	head, err := manager.HeadCommit(ctx)
	if err != nil {
		return nil, err
	}
	plan := &ReleasePlan{
		repo:       repo,
		head:       head,
		bump:       req.Bump.Clone(),
		tagName:    req.TagName,
		tagOptions: req.TagOptions,
		manager:    manager,
	}

	if plan.bump != nil {
		if len(plan.bump.ChangedFiles()) == 0 {
			return nil, fmt.Errorf("version bump plan contains no changed files")
		}
		changes, err := manager.WorktreeChanges(ctx)
		if err != nil {
			return nil, err
		}
		if len(changes) > 0 && !req.TagOptions.Force {
			return nil, fmt.Errorf("bump preflight requires a clean worktree; found: %s", strings.Join(changes, ", "))
		}
		plan.actions = append(plan.actions, "version bump")
	}
	if req.TagOptions.Tag {
		if plan.bump == nil {
			return nil, fmt.Errorf("tagging requires an exact version bump plan")
		}
		if err := manager.ValidatePlan(ctx, req.TagName, req.TagOptions); err != nil {
			return nil, fmt.Errorf("tag preflight: %w", err)
		}
		plan.actions = append(plan.actions, "git commit/tag/push")
	}
	if req.Publish {
		if !req.TagOptions.Tag || !req.TagOptions.Push {
			return nil, fmt.Errorf("--publish requires --tag --push so the provider release is bound to an immutable remote tag")
		}
		ref, err := newRemoteReleaseRef(req.TagName)
		if err != nil {
			return nil, err
		}
		if err := preflightProvider(req.Configuration); err != nil {
			return nil, err
		}
		plan.remoteRef = &ref
		plan.actions = append(plan.actions, "provider publish")
	}
	if req.Confluence || req.Trends || (req.Changelog && changelogDestination(req.Configuration) == "confluence") {
		if err := preflightConfluence(req.Configuration); err != nil {
			return nil, err
		}
	}
	if req.Confluence {
		plan.actions = append(plan.actions, "Confluence publish")
	}
	if req.Changelog {
		if err := preflightChangelog(repo, req.Configuration); err != nil {
			return nil, err
		}
		plan.actions = append(plan.actions, "changelog update")
	}
	if req.Trends {
		if !req.Confluence {
			return nil, fmt.Errorf("--trends requires --confluence")
		}
		plan.actions = append(plan.actions, "trends publish")
	}
	for _, target := range []struct {
		name string
		path string
	}{
		{name: "HTML report", path: req.HTMLPath},
		{name: "output", path: req.OutputPath},
	} {
		if target.path == "" {
			continue
		}
		if err := preflightFileTarget(target.path); err != nil {
			return nil, fmt.Errorf("%s preflight: %w", target.name, err)
		}
		plan.actions = append(plan.actions, target.name+" write")
	}
	if req.TrendSnapshotPath != "" {
		if err := preflightCreatableFileTarget(req.TrendSnapshotPath); err != nil {
			return nil, fmt.Errorf("trend snapshot preflight: %w", err)
		}
		plan.trendSnapshotPath, err = filepath.Abs(req.TrendSnapshotPath)
		if err != nil {
			return nil, fmt.Errorf("resolve trend snapshot path: %w", err)
		}
		plan.actions = append(plan.actions, "trend snapshot write")
	}
	return plan, nil
}

func (p *ReleasePlan) Actions() []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.actions...)
}

func (p *ReleasePlan) ChangedFiles() []string {
	if p == nil || p.bump == nil {
		return nil
	}
	return p.bump.ChangedFiles()
}

func (p *ReleasePlan) TrendSnapshotPath() (string, bool) {
	if p == nil || p.trendSnapshotPath == "" {
		return "", false
	}
	return p.trendSnapshotPath, true
}

func (p *ReleasePlan) HasBump() bool { return p != nil && p.bump != nil }

func (p *ReleasePlan) HasTag() bool { return p != nil && p.tagOptions.Tag }

func (p *ReleasePlan) RemoteRef() (RemoteReleaseRef, bool) {
	if p == nil || p.remoteRef == nil {
		return RemoteReleaseRef{}, false
	}
	return *p.remoteRef, true
}

// Revalidate closes the planning/apply time-of-check gap. It is called at the
// irreversible boundary immediately before the first version-file write.
func (p *ReleasePlan) Revalidate(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("release plan is nil")
	}
	head, err := p.manager.HeadCommit(ctx)
	if err != nil {
		return err
	}
	if head != p.head {
		return fmt.Errorf("repository HEAD changed after planning: %s -> %s", p.head, head)
	}
	changes, err := p.manager.WorktreeChanges(ctx)
	if err != nil {
		return err
	}
	if len(changes) > 0 && !p.tagOptions.Force {
		return fmt.Errorf("repository changed after planning; found: %s", strings.Join(changes, ", "))
	}
	if p.tagOptions.Tag {
		if err := p.manager.ValidatePlan(ctx, p.tagName, p.tagOptions); err != nil {
			return err
		}
	}
	return nil
}

func (p *ReleasePlan) ApplyBump() error {
	if p == nil || p.bump == nil {
		return nil
	}
	return p.bump.Apply()
}

func (p *ReleasePlan) ApplyGit(ctx context.Context) (*gittag.Result, error) {
	if p == nil || !p.tagOptions.Tag {
		return nil, nil
	}
	return p.manager.Run(ctx, p.tagName, p.bump.ChangedFiles(), p.tagOptions)
}

func (p *ReleasePlan) VerifyRemoteRef(ctx context.Context) error {
	if p == nil || p.remoteRef == nil {
		return fmt.Errorf("release plan has no immutable remote release ref")
	}
	return p.manager.VerifyRemoteTag(ctx, p.remoteRef.tag)
}

func preflightProvider(cfg config.Config) error {
	providerCfg := providerConfig(cfg)
	var missing []string
	if providerCfg.Type == "" {
		missing = append(missing, "provider.type")
	}
	if providerCfg.Token == "" {
		missing = append(missing, "provider.token")
	}
	if providerCfg.Repo == "" {
		missing = append(missing, "provider.repo")
	}
	if len(missing) > 0 {
		return withHint(
			fmt.Errorf("publish preflight missing required configuration: %s", strings.Join(missing, ", ")),
			"run `patchlog init`, configure the missing provider fields, then retry `patchlog release --dry-run`; or omit `--publish`",
		)
	}
	if _, err := provider.New(providerCfg); err != nil {
		return withHint(
			fmt.Errorf("provider preflight: %w", err),
			"fix the provider configuration and retry `patchlog release --dry-run`",
		)
	}
	return nil
}

func preflightConfluence(cfg config.Config) error {
	c := resolveConfluenceConfig(cfg)
	var missing []string
	if c.BaseURL == "" {
		missing = append(missing, "confluence.base_url")
	}
	if c.APIToken == "" {
		missing = append(missing, "confluence.api_token")
	}
	if c.SpaceKey == "" {
		missing = append(missing, "confluence.space_key")
	}
	if len(missing) > 0 {
		return withHint(
			fmt.Errorf("Confluence preflight missing required configuration: %s", strings.Join(missing, ", ")),
			"run `patchlog init`, configure the missing Confluence fields, then retry `patchlog release --dry-run`; or omit the Confluence option",
		)
	}
	client := confluence.NewClient(confluence.Config{
		BaseURL: c.BaseURL, Email: c.Email, APIToken: c.APIToken,
		SpaceKey: c.SpaceKey, ParentPageID: c.ParentPageID,
	})
	if !client.Configured() {
		return withHint(
			fmt.Errorf("Confluence configuration is incomplete"),
			"run `patchlog init`, fix the Confluence configuration, then retry `patchlog release --dry-run`",
		)
	}
	return nil
}

func preflightChangelog(repo string, cfg config.Config) error {
	switch changelogDestination(cfg) {
	case "md":
		path := cfg.Changelog.File
		if path == "" {
			path = "CHANGELOG.md"
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		return preflightFileTarget(path)
	case "wiki":
		if cfg.Provider.Type != "gitlab" {
			return withHint(
				fmt.Errorf("wiki changelog preflight requires provider.type gitlab"),
				"set `provider.type: gitlab` and retry `patchlog release --dry-run`; or choose another changelog destination",
			)
		}
		client := gitwiki.NewClient(gitwiki.Config{Token: cfg.Provider.Token, Repo: cfg.Provider.Repo, BaseURL: cfg.Provider.BaseURL, Slug: cfg.Changelog.Slug})
		if !client.Configured() {
			return withHint(
				fmt.Errorf("wiki changelog preflight missing required configuration: provider.token, provider.repo"),
				"run `patchlog init`, configure the GitLab provider, then retry `patchlog release --dry-run`",
			)
		}
		return nil
	case "confluence":
		return preflightConfluence(cfg)
	default:
		return fmt.Errorf("unsupported changelog destination %q", cfg.Changelog.Destination)
	}
}

func changelogDestination(cfg config.Config) string {
	if cfg.Changelog.Destination == "" {
		return "md"
	}
	return cfg.Changelog.Destination
}

func preflightFileTarget(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink output target %s", abs)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(abs)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("output directory %s: %w", parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output parent %s is not a directory", parent)
	}
	return nil
}

func preflightCreatableFileTarget(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink output target %s", abs)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	parent := filepath.Dir(abs)
	for {
		info, err := os.Lstat(parent)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing symlink output directory %s", parent)
			}
			if !info.IsDir() {
				return fmt.Errorf("output parent %s is not a directory", parent)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return fmt.Errorf("no existing output ancestor for %s", abs)
		}
		parent = next
	}
}
