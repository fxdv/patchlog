package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/confluence"
	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/gitwiki"
	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/provider"
	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/theme"
	"github.com/fxdv/patchlog/pkg/trends"
)

func resolveConfluenceConfig(cfg config.Config) config.ConfluenceConfig {
	c := cfg.Confluence
	if c.BaseURL == "" && cfg.Jira.BaseURL != "" {
		c.BaseURL = cfg.Jira.BaseURL
	}
	if c.Email == "" && cfg.Jira.Email != "" {
		c.Email = cfg.Jira.Email
	}
	if c.APIToken == "" && cfg.Jira.APIToken != "" {
		c.APIToken = cfg.Jira.APIToken
	}
	return c
}

func renderProseLang(ctx context.Context, report render.Report, tone ai.Tone, cfg config.Config, quiet bool, lang i18n.Lang) ([]byte, error) {
	client := newAIClientOrWarn(cfg, "prose", quiet)

	var streamCallback func(string)
	if client != nil {
		if !quiet {
			console.Step("Generating AI prose (streaming)...")
		}
		streamCallback = func(token string) {
			fmt.Fprint(os.Stderr, token)
		}
	}

	text, err := ai.GenerateProseStreamLang(ctx, report, tone, client, streamCallback, lang)
	if streamCallback != nil {
		fmt.Fprint(os.Stderr, "\n")
	}
	if err != nil {
		if text != "" {
			return []byte(text), err
		}
		return nil, err
	}
	return []byte(text), nil
}

func providerConfig(cfg config.Config) provider.Config {
	provCfg := provider.Config{
		Type:    provider.Type(cfg.Provider.Type),
		Token:   cfg.Provider.Token,
		Repo:    cfg.Provider.Repo,
		BaseURL: cfg.Provider.BaseURL,
		Draft:   cfg.Provider.Draft,
	}
	if provCfg.Repo == "" {
		provCfg.Repo = cfg.Repo
	}
	return provCfg
}

func runPublish(ctx context.Context, releaseRef RemoteReleaseRef, cfg config.Config, notes string) (string, error) {
	provCfg := providerConfig(cfg)
	if provCfg.Type == "" {
		return "", fmt.Errorf("publish requires provider config (type, token, repo)")
	}
	prov, err := provider.New(provCfg)
	if err != nil {
		return "", fmt.Errorf("provider setup: %w", err)
	}
	release, err := prov.CreateRelease(ctx, releaseRef.Tag(), notes)
	if err != nil {
		return "", fmt.Errorf("creating release %s: %w", releaseRef.Tag(), err)
	}
	return release.URL, nil
}

func runConfluencePublish(ctx context.Context, report render.Report, cfg config.Config, fileCache *cache.Cache, aiSummary string, sm ai.SummaryMetrics, command string, interpretations map[string]string, metricsNarrative string, themedReport *theme.ThemedReport, lang i18n.Lang) (string, bool, error) {
	confluenceCfg := resolveConfluenceConfig(cfg)
	confluence.SetConfluenceLabels(i18n.ConfluenceLabelsFor(lang))

	client := confluence.NewClient(confluence.Config{
		BaseURL:         confluenceCfg.BaseURL,
		Email:           confluenceCfg.Email,
		APIToken:        confluenceCfg.APIToken,
		SpaceKey:        confluenceCfg.SpaceKey,
		ParentPageID:    confluenceCfg.ParentPageID,
		Labels:          confluenceCfg.Labels,
		ViewRestriction: confluenceCfg.ViewRestriction,
		EditRestriction: confluenceCfg.EditRestriction,
		Template:        confluenceCfg.Template,
	})
	client.SetCache(fileCache)
	if !client.Configured() {
		return "", false, fmt.Errorf("confluence requires base_url, api_token, and space_key in config")
	}

	reportDate := report.Date
	if reportDate == "" {
		reportDate = time.Now().Format("2006-01-02")
	}
	title := fmt.Sprintf("Release Notes — %s (%s)", report.Version, reportDate)

	var releaseNotes string
	if themedReport != nil {
		releaseNotes = confluence.RenderThemedStorageFormat(*themedReport)
	} else {
		releaseNotes = confluence.RenderStorageFormat(report)
	}
	analyticsPanel := ""
	if sm.TotalCommits > 0 {
		ad := confluence.AnalyticsData{
			TotalCommits:                     sm.TotalCommits,
			TotalAuthors:                     sm.TotalAuthors,
			BreakingChanges:                  sm.BreakingChanges,
			SignificanceCounts:               sm.SignificanceCounts,
			TypeCounts:                       sm.TypeCounts,
			DateRange:                        sm.DateRange,
			CommitsPerDay:                    sm.CommitsPerDay,
			MostActiveDay:                    sm.MostActiveDay,
			MostActiveDayCount:               sm.MostActiveDayCount,
			FilesTouched:                     sm.FilesTouched,
			LinesAdded:                       sm.LinesAdded,
			LinesDeleted:                     sm.LinesDeleted,
			NetLines:                         sm.NetLines,
			JiraTicketsLinked:                sm.JiraTicketsLinked,
			HotspotDensity:                   sm.HotspotDensity,
			ChurnFactor:                      sm.ChurnFactor,
			ComplexityPerFeat:                sm.ComplexityPerFeat,
			ReleaseCommitSpanHours:           sm.ReleaseCommitSpanHours,
			ReleaseAgeHours:                  sm.ReleaseAgeHours,
			OwnershipConc:                    sm.OwnershipConc,
			ReleaseContributionConcentration: sm.ReleaseContributionConcentration,
			FixToFeatureRatio:                sm.FixToFeatureRatio,
			TestToSourceRatio:                sm.TestToSourceRatio,
			RefactoringRatio:                 sm.RefactoringRatio,
			APISurfaceChange:                 sm.APISurfaceChange,
			ReleaseRiskScore:                 sm.ReleaseRiskScore,
			BatchFactor:                      sm.BatchFactor,
			RevertRate:                       sm.RevertRate,
			ScopeIsolation:                   sm.ScopeIsolation,
			CrossCuttingPct:                  sm.CrossCuttingPct,
			FileVolatility:                   sm.FileVolatility,
			Interpretations:                  interpretations,
			MetricsNarrative:                 metricsNarrative,
			ChangeComplexityProxy:            sm.ChangeComplexityProxy,
			CrossCuttingChangeRisk:           sm.CrossCuttingChangeRisk,
			TechnicalDebtUSD:                 sm.TechnicalDebtUSD,
			TouchedTestFileRatio:             sm.TouchedTestFileRatio,
			OwnershipEntropy:                 sm.OwnershipEntropy,
			HotspotScore:                     sm.HotspotScore,
		}
		for _, c := range sm.TopContributors {
			ad.TopContributors = append(ad.TopContributors, confluence.ContributorEntry{
				Name:    c.Name,
				Commits: c.Commits,
			})
		}
		analyticsPanel = confluence.RenderAnalyticsPanel(ad)
		if analyticsPanel != "" {
			analyticsPanel = confluence.Spacer() + analyticsPanel
		}
	}

	var summaryPanel string
	if aiSummary != "" {
		summaryPanel = confluence.RenderSummaryPanel(aiSummary) + confluence.Spacer()
	}

	var pageProperties string
	if sm.TotalCommits > 0 {
		ad := confluence.AnalyticsData{
			TotalCommits:     sm.TotalCommits,
			TotalAuthors:     sm.TotalAuthors,
			BreakingChanges:  sm.BreakingChanges,
			ReleaseRiskScore: sm.ReleaseRiskScore,
		}
		pageProperties = confluence.RenderPageProperties(report, ad)
	}

	epicsPanel := confluence.RenderEpicsPanel(report)
	if epicsPanel != "" {
		epicsPanel = confluence.Spacer() + epicsPanel
	}

	var prevNextNav string
	siblings, _ := client.FindSiblingPages(ctx, "Release Notes")
	if len(siblings) > 1 {
		var prev, next *confluence.SiblingPage
		for i := range siblings {
			s := siblings[i]
			if s.Title == title {
				if i > 0 {
					prev = &siblings[i-1]
				}
				if i < len(siblings)-1 {
					next = &siblings[i+1]
				}
				break
			}
		}
		if prev == nil && len(siblings) > 0 && siblings[0].Title != title {
			next = &siblings[0]
		}
		if prev != nil || next != nil {
			prevNextNav = confluence.RenderPrevNextNav(prev, next)
		}
	}

	sections := confluence.PageSections{
		PageProperties: pageProperties,
		AISummary:      summaryPanel,
		Analytics:      analyticsPanel,
		ReleaseNotes:   releaseNotes,
		EpicsPanel:     epicsPanel,
		PrevNextNav:    prevNextNav,
		CommandFooter:  confluence.RenderCommandFooter(command),
	}

	body, err := confluence.RenderPageWithTemplate(sections, confluenceCfg.Template)
	if err != nil {
		return "", false, fmt.Errorf("confluence render: %w", err)
	}

	page, updated, err := client.PublishOrUpdate(ctx, title, body)
	if err != nil {
		return "", false, err
	}

	if err := configureConfluencePage(ctx, client, page.ID, confluenceCfg); err != nil {
		return page.URL, updated, fmt.Errorf("Confluence page %s was published, but its labels or restrictions are incomplete: %w", page.URL, err)
	}

	return page.URL, updated, nil
}

func runChangelogAccumulate(ctx context.Context, report render.Report, cfg config.Config, notes string, repoPath string, fileCache *cache.Cache) (string, error) {
	destination := cfg.Changelog.Destination
	if destination == "" {
		destination = "md"
	}

	useEmoji := true
	if cfg.Changelog.Emojis != nil {
		useEmoji = *cfg.Changelog.Emojis
	}
	report.Emojis = useEmoji

	title := cfg.Changelog.Title
	if title == "" {
		title = "Changelog"
	}

	switch destination {
	case "md":
		return accumulateMarkdown(ctx, report, cfg, repoPath, useEmoji)
	case "wiki":
		return accumulateWiki(ctx, report, cfg, useEmoji, title)
	case "confluence":
		return accumulateConfluence(ctx, report, cfg, fileCache, useEmoji, title)
	default:
		return "", fmt.Errorf("unknown changelog destination %q (use md, wiki, or confluence)", destination)
	}
}

func accumulateMarkdown(_ context.Context, report render.Report, cfg config.Config, repoPath string, useEmoji bool) (string, error) {
	filePath := cfg.Changelog.File
	if filePath == "" {
		filePath = "CHANGELOG.md"
	}
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(repoPath, filePath)
	}

	section, err := render.MarkdownSection(report, useEmoji)
	if err != nil {
		return "", err
	}

	existing, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read existing changelog: %w", err)
	}
	combined := render.AccumulateMarkdown(existing, section, cfg.Changelog.Title)

	if err := atomicWriteFile(filePath, combined, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

func accumulateWiki(ctx context.Context, report render.Report, cfg config.Config, useEmoji bool, title string) (string, error) {
	wikiCfg := gitwiki.Config{
		Token:   cfg.Provider.Token,
		Repo:    cfg.Provider.Repo,
		BaseURL: cfg.Provider.BaseURL,
		Slug:    cfg.Changelog.Slug,
	}
	if wikiCfg.Repo == "" {
		wikiCfg.Repo = cfg.Repo
	}

	client := gitwiki.NewClient(wikiCfg)
	if !client.Configured() {
		return "", fmt.Errorf("wiki accumulation requires provider config (type: gitlab, token, repo)")
	}

	section, err := render.MarkdownSection(report, useEmoji)
	if err != nil {
		return "", err
	}

	page, _, err := client.AccumulatePage(ctx, title, string(section))
	if err != nil {
		return "", err
	}

	return page.URL, nil
}

func accumulateConfluence(ctx context.Context, report render.Report, cfg config.Config, fileCache *cache.Cache, useEmoji bool, title string) (string, error) {
	confluenceCfg := resolveConfluenceConfig(cfg)

	client := confluence.NewClient(confluence.Config{
		BaseURL:         confluenceCfg.BaseURL,
		Email:           confluenceCfg.Email,
		APIToken:        confluenceCfg.APIToken,
		SpaceKey:        confluenceCfg.SpaceKey,
		ParentPageID:    confluenceCfg.ParentPageID,
		Labels:          confluenceCfg.Labels,
		ViewRestriction: confluenceCfg.ViewRestriction,
		EditRestriction: confluenceCfg.EditRestriction,
		Template:        confluenceCfg.Template,
	})
	client.SetCache(fileCache)
	if !client.Configured() {
		return "", fmt.Errorf("confluence accumulation requires base_url, api_token, and space_key in config")
	}

	report.Emojis = useEmoji
	sectionBody := confluence.RenderSection(report)

	page, updated, err := client.AccumulatePage(ctx, title, sectionBody)
	if err != nil {
		return "", err
	}

	if err := configureConfluencePage(ctx, client, page.ID, confluenceCfg); err != nil {
		action := "created"
		if updated {
			action = "updated"
		}
		return page.URL, fmt.Errorf("Confluence changelog page %s was %s, but its labels or restrictions are incomplete: %w", page.URL, action, err)
	}

	return page.URL, nil
}

func runTrendsConfluencePublish(ctx context.Context, snapshots []trends.Snapshot, cfg config.Config, fileCache *cache.Cache) (string, bool, error) {
	return runTrendsConfluencePublishWithGamification(ctx, snapshots, cfg, fileCache, "")
}

func runTrendsConfluencePublishWithGamification(ctx context.Context, snapshots []trends.Snapshot, cfg config.Config, fileCache *cache.Cache, gamificationHTML string) (string, bool, error) {
	confluenceCfg := resolveConfluenceConfig(cfg)

	client := confluence.NewClient(confluence.Config{
		BaseURL:         confluenceCfg.BaseURL,
		Email:           confluenceCfg.Email,
		APIToken:        confluenceCfg.APIToken,
		SpaceKey:        confluenceCfg.SpaceKey,
		ParentPageID:    confluenceCfg.ParentPageID,
		Labels:          confluenceCfg.Labels,
		ViewRestriction: confluenceCfg.ViewRestriction,
		EditRestriction: confluenceCfg.EditRestriction,
	})
	client.SetCache(fileCache)
	if !client.Configured() {
		return "", false, fmt.Errorf("confluence requires base_url, api_token, and space_key in config")
	}

	title := cfg.Trends.Title
	if title == "" {
		title = "Release Trends"
	}

	var body string
	if gamificationHTML != "" {
		body = confluence.RenderTrendsPageWithGamification(snapshots, confluence.TrendsThresholds{
			ReleaseCommitSpanWarning:            cfg.Trends.Thresholds.ReleaseCommitSpanWarning,
			ReleaseCommitSpanCritical:           cfg.Trends.Thresholds.ReleaseCommitSpanCritical,
			TechDebtWarning:                     cfg.Trends.Thresholds.TechDebtWarning,
			TechDebtCritical:                    cfg.Trends.Thresholds.TechDebtCritical,
			ReleaseContributionConcentrationMin: cfg.Trends.Thresholds.ReleaseContributionConcentrationMin,
		}, gamificationHTML)
	} else {
		body = confluence.RenderTrendsPage(snapshots, confluence.TrendsThresholds{
			ReleaseCommitSpanWarning:            cfg.Trends.Thresholds.ReleaseCommitSpanWarning,
			ReleaseCommitSpanCritical:           cfg.Trends.Thresholds.ReleaseCommitSpanCritical,
			TechDebtWarning:                     cfg.Trends.Thresholds.TechDebtWarning,
			TechDebtCritical:                    cfg.Trends.Thresholds.TechDebtCritical,
			ReleaseContributionConcentrationMin: cfg.Trends.Thresholds.ReleaseContributionConcentrationMin,
		})
	}

	page, updated, err := client.PublishOrUpdate(ctx, title, body)
	if err != nil {
		return "", false, err
	}

	if err := configureConfluencePage(ctx, client, page.ID, confluenceCfg); err != nil {
		return page.URL, updated, fmt.Errorf("Confluence trends page %s was published, but its labels or restrictions are incomplete: %w", page.URL, err)
	}

	return page.URL, updated, nil
}

func configureConfluencePage(ctx context.Context, client *confluence.Client, pageID string, cfg config.ConfluenceConfig) error {
	var errs []error
	if err := client.AddLabels(ctx, pageID, cfg.Labels); err != nil {
		errs = append(errs, fmt.Errorf("labels: %w", err))
	}
	if err := client.SetRestrictions(ctx, pageID, cfg.ViewRestriction, cfg.EditRestriction); err != nil {
		errs = append(errs, fmt.Errorf("restrictions: %w", err))
	}
	return errors.Join(errs...)
}
