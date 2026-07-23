package main

import (
	"context"
	"errors"
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

type ReleasePhase string

const (
	ReleasePhaseDirect   ReleasePhase = "direct"
	ReleasePhasePrepare  ReleasePhase = "prepare"
	ReleasePhaseFinalize ReleasePhase = "finalize"
)

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
	Phase             ReleasePhase
	Bump              *bump.Plan
	TagName           string
	TargetVersion     string
	ProtectedBranch   string
	ReleaseBranch     string
	TagOptions        gittag.Options
	Publish           bool
	Confluence        bool
	Changelog         bool
	Trends            bool
	HTMLPath          string
	OutputPath        string
	TrendSnapshotPath string
	RenderedOutput    []byte
	Configuration     config.Config
}

// ReleasePlan is the single immutable description of every requested release
// mutation. All fields are private, mutable inputs are cloned, and construction
// performs every deterministic local and remote preflight before Apply.
type ReleasePlan struct {
	repo               string
	head               string
	phase              ReleasePhase
	bump               *bump.Plan
	tagName            string
	targetVersion      string
	protectedBranch    string
	releaseBranch      string
	tagOptions         gittag.Options
	remoteRef          *RemoteReleaseRef
	trendSnapshotPath  string
	renderedOutputHash string
	publishTarget      string
	confluenceTarget   string
	mutationTargets    []string
	actions            []string
	fingerprint        string
	manager            *gittag.Manager
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
	phase := req.Phase
	if phase == "" {
		phase = ReleasePhaseDirect
	}
	plan := &ReleasePlan{
		repo:            repo,
		head:            head,
		phase:           phase,
		bump:            req.Bump.Clone(),
		tagName:         req.TagName,
		targetVersion:   req.TargetVersion,
		protectedBranch: req.ProtectedBranch,
		releaseBranch:   req.ReleaseBranch,
		tagOptions:      req.TagOptions,
		manager:         manager,
	}
	if plan.targetVersion == "" && plan.bump != nil {
		plan.targetVersion = plan.bump.NewVersion
	}
	if plan.tagName != "" && plan.targetVersion != "" {
		tagVersion := strings.TrimPrefix(plan.tagName, gittag.DetectPrefix(plan.tagName))
		if tagVersion != plan.targetVersion {
			return nil, fmt.Errorf("release tag %s does not match target version %s", plan.tagName, plan.targetVersion)
		}
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
	}
	switch plan.phase {
	case ReleasePhasePrepare:
		if req.Publish || req.Confluence || req.Changelog || req.Trends ||
			req.HTMLPath != "" || req.OutputPath != "" || req.TrendSnapshotPath != "" {
			return nil, fmt.Errorf("protected prepare contains only branch, version, and commit mutations; run optional publishing workflows separately")
		}
		if plan.bump == nil {
			return nil, fmt.Errorf("protected prepare requires an exact version bump plan")
		}
		if plan.protectedBranch == "" || plan.releaseBranch == "" {
			return nil, fmt.Errorf("protected prepare requires protected and release branch identities")
		}
		if err := manager.ValidateProtectedPrepare(ctx, plan.protectedBranch, plan.releaseBranch); err != nil {
			return nil, fmt.Errorf("protected prepare preflight: %w", err)
		}
		plan.actions = append(plan.actions,
			"create release branch",
			"version bump",
			"isolated release commit",
			"push release branch",
		)
	case ReleasePhaseFinalize:
		if req.Confluence || req.Changelog || req.Trends ||
			req.HTMLPath != "" || req.OutputPath != "" || req.TrendSnapshotPath != "" {
			return nil, fmt.Errorf("protected finalize contains only immutable tag and provider release operations; run optional extensions separately")
		}
		if plan.bump != nil {
			return nil, fmt.Errorf("protected finalize cannot contain a version bump")
		}
		if plan.targetVersion == "" || plan.tagName == "" {
			return nil, fmt.Errorf("protected finalize requires an exact version and tag")
		}
		if plan.protectedBranch == "" {
			return nil, fmt.Errorf("protected finalize requires a protected branch identity")
		}
		if err := manager.ValidateProtectedFinalize(ctx, plan.protectedBranch, plan.tagName); err != nil {
			return nil, fmt.Errorf("protected finalize preflight: %w", err)
		}
		plan.actions = append(plan.actions, "annotated tag", "push immutable tag")
	case ReleasePhaseDirect:
		if plan.bump != nil {
			plan.actions = append(plan.actions, "version bump")
		}
	default:
		return nil, fmt.Errorf("unsupported release phase %q", plan.phase)
	}
	if plan.phase == ReleasePhaseDirect && req.TagOptions.Tag {
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
		if plan.phase == ReleasePhasePrepare {
			return nil, fmt.Errorf("provider publishing belongs to protected finalize, not prepare")
		}
		ref, err := newRemoteReleaseRef(req.TagName)
		if err != nil {
			return nil, err
		}
		if err := preflightProvider(req.Configuration); err != nil {
			return nil, err
		}
		providerCfg := providerConfig(req.Configuration)
		draft := true
		if providerCfg.Draft != nil {
			draft = *providerCfg.Draft
		}
		plan.publishTarget = strings.Join([]string{
			string(providerCfg.Type),
			providerCfg.BaseURL,
			providerCfg.Repo,
			fmt.Sprintf("draft=%t", draft),
		}, "|")
		plan.remoteRef = &ref
		plan.actions = append(plan.actions, "provider publish")
	}
	if req.Confluence || req.Trends || (req.Changelog && changelogDestination(req.Configuration) == "confluence") {
		if err := preflightConfluence(req.Configuration); err != nil {
			return nil, err
		}
	}
	if req.Confluence {
		confluenceCfg := resolveConfluenceConfig(req.Configuration)
		plan.confluenceTarget = strings.Join([]string{
			confluenceCfg.BaseURL,
			confluenceCfg.SpaceKey,
			confluenceCfg.ParentPageID,
		}, "|")
		plan.mutationTargets = append(plan.mutationTargets, "confluence:"+plan.confluenceTarget)
		plan.actions = append(plan.actions, "Confluence publish")
	}
	if req.Changelog {
		if err := preflightChangelog(repo, req.Configuration); err != nil {
			return nil, err
		}
		plan.mutationTargets = append(plan.mutationTargets, "changelog:"+changelogTargetIdentity(repo, req.Configuration))
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
		absoluteTarget, err := filepath.Abs(target.path)
		if err != nil {
			return nil, fmt.Errorf("resolve %s target: %w", target.name, err)
		}
		plan.mutationTargets = append(plan.mutationTargets, target.name+":"+absoluteTarget)
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
		plan.mutationTargets = append(plan.mutationTargets, "trend snapshot:"+plan.trendSnapshotPath)
		plan.actions = append(plan.actions, "trend snapshot write")
	}
	if req.Publish || req.Confluence || req.Changelog || req.OutputPath != "" {
		plan.renderedOutputHash = sha256Digest(req.RenderedOutput)
	}
	plan.fingerprint, err = fingerprintReleasePlan(plan)
	if err != nil {
		return nil, fmt.Errorf("fingerprint release plan: %w", err)
	}
	return plan, nil
}

func (p *ReleasePlan) Phase() ReleasePhase {
	if p == nil {
		return ""
	}
	return p.phase
}

func (p *ReleasePlan) Fingerprint() string {
	if p == nil {
		return ""
	}
	return p.fingerprint
}

func (p *ReleasePlan) RequireApproval(approved string) error {
	if p == nil {
		return fmt.Errorf("release plan is nil")
	}
	approved = strings.TrimSpace(approved)
	if approved == "" {
		return withHint(
			fmt.Errorf("release mutation requires approval of plan %s", p.fingerprint),
			fmt.Sprintf("review `patchlog release --dry-run`, then rerun the indicated command with `--approve %s`", p.fingerprint),
		)
	}
	if approved != p.fingerprint {
		return withHint(
			fmt.Errorf("approved plan %s does not match current plan %s", approved, p.fingerprint),
			"repository or release inputs changed; rerun `patchlog release --dry-run` and approve the new fingerprint",
		)
	}
	return nil
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
	switch p.phase {
	case ReleasePhasePrepare:
		if err := p.manager.ValidateProtectedPrepare(ctx, p.protectedBranch, p.releaseBranch); err != nil {
			return err
		}
	case ReleasePhaseFinalize:
		if err := p.manager.ValidateProtectedFinalize(ctx, p.protectedBranch, p.tagName); err != nil {
			return err
		}
	case ReleasePhaseDirect:
		if !p.tagOptions.Tag {
			return nil
		}
		if err := p.manager.ValidatePlan(ctx, p.tagName, p.tagOptions); err != nil {
			return err
		}
	}
	return nil
}

func (p *ReleasePlan) ApplyProtectedPrepare(ctx context.Context) (*gittag.Result, error) {
	if p == nil || p.phase != ReleasePhasePrepare || p.bump == nil {
		return nil, fmt.Errorf("protected prepare requires a prepared release plan")
	}
	if err := p.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("release plan changed before prepare: %w", err)
	}
	if err := p.manager.CreateBranch(ctx, p.releaseBranch); err != nil {
		return nil, fmt.Errorf("create release branch %s: %w", p.releaseBranch, err)
	}
	cleanupBeforeCommit := func(cause error) error {
		rollbackErr := p.bump.Rollback()
		switchErr := p.manager.SwitchBranch(ctx, p.protectedBranch)
		deleteErr := p.manager.DeleteBranch(ctx, p.releaseBranch)
		return errors.Join(cause, rollbackErr, switchErr, deleteErr)
	}
	if err := p.bump.Apply(); err != nil {
		_ = p.manager.SwitchBranch(ctx, p.protectedBranch)
		_ = p.manager.DeleteBranch(ctx, p.releaseBranch)
		return nil, fmt.Errorf("apply protected version bump: %w", err)
	}
	result := &gittag.Result{Files: p.bump.ChangedFiles(), Branch: p.releaseBranch}
	message := fmt.Sprintf("chore(release): prepare v%s\n\nPatchlog-Plan: %s", p.targetVersion, p.fingerprint)
	commit, err := p.manager.CommitFiles(ctx, message, p.bump.ChangedFiles())
	if err != nil {
		return result, fmt.Errorf("prepare release commit: %w", cleanupBeforeCommit(err))
	}
	result.Commit = commit
	if err := p.manager.PushBranch(ctx, p.releaseBranch); err != nil {
		return result, fmt.Errorf("release branch commit %s remains locally after push failure: %w", commit, err)
	}
	result.Pushed = true
	return result, nil
}

func (p *ReleasePlan) ApplyProtectedFinalize(ctx context.Context) (*gittag.Result, error) {
	if p == nil || p.phase != ReleasePhaseFinalize {
		return nil, fmt.Errorf("protected finalize requires a finalize release plan")
	}
	if err := p.Revalidate(ctx); err != nil {
		return nil, fmt.Errorf("release plan changed before finalize: %w", err)
	}
	result := &gittag.Result{Tag: p.tagName, Branch: p.protectedBranch}
	if err := p.manager.CreateTag(ctx, p.tagName, fmt.Sprintf("Release %s", p.tagName)); err != nil {
		return result, err
	}
	if err := p.manager.PushTag(ctx, p.tagName); err != nil {
		return result, fmt.Errorf("local immutable tag %s remains after push failure: %w", p.tagName, err)
	}
	result.Pushed = true
	return result, nil
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

func changelogTargetIdentity(repo string, cfg config.Config) string {
	switch changelogDestination(cfg) {
	case "md":
		path := cfg.Changelog.File
		if path == "" {
			path = "CHANGELOG.md"
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		absolute, err := filepath.Abs(path)
		if err == nil {
			return absolute
		}
		return filepath.Clean(path)
	case "wiki":
		return strings.Join([]string{cfg.Provider.BaseURL, cfg.Provider.Repo, cfg.Changelog.Slug}, "|")
	case "confluence":
		confluenceCfg := resolveConfluenceConfig(cfg)
		return strings.Join([]string{confluenceCfg.BaseURL, confluenceCfg.SpaceKey, cfg.Changelog.Title}, "|")
	default:
		return changelogDestination(cfg)
	}
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
