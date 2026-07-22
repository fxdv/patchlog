package main

import (
	"context"
	"time"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/classify"
	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/gittag"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/render"
	"github.com/fxdv/patchlog/pkg/theme"
	"github.com/fxdv/patchlog/pkg/trends"
)

// PipelineState holds all shared state across pipeline stages.
type PipelineState struct {
	Ctx           context.Context
	Cfg           config.Config
	Fetcher       *gitlog.Fetcher
	Repo          string
	Quiet         bool
	DryRun        bool
	Cache         *cache.Cache
	Tone          ai.Tone
	RangeFrom     string
	To            string
	Report        render.Report
	ParsedCommits []commit.Commit
	ReportMetrics metrics.ReportMetrics
	CodeStats     metrics.CodeStats
	MetricsReady  bool
	AISummary     string
	BumpLevel     string
	TagResult     *gittag.Result
	ThemedReport  *theme.ThemedReport
	Output        []byte
	Flags         map[string]bool
}

// thresholdsFromConfig builds classify thresholds from config, applying overrides.
func thresholdsFromConfig(cfg config.Config) classify.Thresholds {
	t := classify.DefaultThresholds()
	if cfg.Classify.LargeFeatureFiles > 0 {
		t.LargeFeatureFiles = cfg.Classify.LargeFeatureFiles
	}
	if cfg.Classify.LargeFixFiles > 0 {
		t.LargeFixFiles = cfg.Classify.LargeFixFiles
	}
	if cfg.Classify.LargeUnknownFiles > 0 {
		t.LargeUnknownFiles = cfg.Classify.LargeUnknownFiles
	}
	return t
}

// snapshotFromMetrics builds a trends Snapshot from computed metrics.
func snapshotFromMetrics(version string, rm metrics.ReportMetrics, cs metrics.CodeStats, sm ai.SummaryMetrics) trends.Snapshot {
	snap := trends.Snapshot{
		Version:                          version,
		Date:                             time.Now().Format("2006-01-02"),
		TotalCommits:                     sm.TotalCommits,
		TotalAuthors:                     sm.TotalAuthors,
		BreakingChanges:                  sm.BreakingChanges,
		ReleaseContributionConcentration: sm.ReleaseContributionConcentration,
		ReleaseCommitSpanHours:           sm.ReleaseCommitSpanHours,
		ReleaseAgeHours:                  sm.ReleaseAgeHours,
		CommitsPerDay:                    sm.CommitsPerDay,
		ConventionalRatio:                rm.ConventionalRatio,
		OwnershipEntropy:                 sm.OwnershipEntropy,
		OwnershipConc:                    sm.OwnershipConc,
		TechDebtUSD:                      sm.TechnicalDebtUSD,
		HotspotScore:                     sm.HotspotScore,
		HotspotDensity:                   sm.HotspotDensity,
		ReleaseRiskScore:                 sm.ReleaseRiskScore,
		NetLines:                         sm.NetLines,
		LinesAdded:                       sm.LinesAdded,
		LinesDeleted:                     sm.LinesDeleted,
		FilesTouched:                     sm.FilesTouched,
		JiraTickets:                      sm.JiraTicketsLinked,
		ChurnFactor:                      sm.ChurnFactor,
		ComplexityPerFeat:                sm.ComplexityPerFeat,
		FixToFeatureRatio:                sm.FixToFeatureRatio,
		TestToSourceRatio:                sm.TestToSourceRatio,
		RefactoringRatio:                 sm.RefactoringRatio,
		APISurfaceChange:                 sm.APISurfaceChange,
		BatchFactor:                      sm.BatchFactor,
		RevertRate:                       sm.RevertRate,
		ScopeIsolation:                   sm.ScopeIsolation,
		CrossCuttingPct:                  sm.CrossCuttingPct,
		FileVolatility:                   sm.FileVolatility,
		ChangeComplexityProxy:            sm.ChangeComplexityProxy,
		CrossCuttingChangeRisk:           sm.CrossCuttingChangeRisk,
		TouchedTestFileRatio:             sm.TouchedTestFileRatio,
		CommitSizeSmall:                  sm.CommitSizeSmall,
		CommitSizeMedium:                 sm.CommitSizeMedium,
		CommitSizeLarge:                  sm.CommitSizeLarge,
		CommitSizeHuge:                   sm.CommitSizeHuge,
		ReviewLoad:                       sm.ReviewLoad,
		DeleteRatio:                      sm.DeleteRatio,
		CommitMsgQuality:                 sm.CommitMsgQuality,
		AuthorOverlap:                    sm.AuthorOverlap,
		PeakHourSpread:                   sm.PeakHourSpread,
		DirectoryBreadth:                 sm.DirectoryBreadth,
		ConfigChurnRate:                  sm.ConfigChurnRate,
		CodeFreshness:                    sm.CodeFreshness,
		RhythmConsistency:                sm.RhythmConsistency,
		LanguageMix:                      sm.LanguageMix,
		TypeCounts:                       sm.TypeCounts,
		SignificanceCounts:               sm.SignificanceCounts,
	}

	limit := 5
	if len(rm.Authors) < limit {
		limit = len(rm.Authors)
	}
	for i := 0; i < limit; i++ {
		snap.TopContributors = append(snap.TopContributors, trends.ContributorSnap{
			Name:    rm.Authors[i].Name,
			Commits: rm.Authors[i].Commits,
		})
	}

	return snap
}

// computeMetrics is a lazy evaluator for report metrics.
func (s *PipelineState) ComputeMetrics() {
	if s.MetricsReady {
		return
	}
	s.ReportMetrics = metrics.ComputeReportMetrics(s.Report, s.ParsedCommits)
	s.CodeStats = metrics.ComputeCodeStats(s.Ctx, s.Fetcher, s.ParsedCommits)
	s.MetricsReady = true
}

// SummaryMetrics returns the computed summary metrics, computing if needed.
func (s *PipelineState) SummaryMetrics() ai.SummaryMetrics {
	s.ComputeMetrics()
	return buildSummaryMetrics(s.ReportMetrics, s.CodeStats)
}
