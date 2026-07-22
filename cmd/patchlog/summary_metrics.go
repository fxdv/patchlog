package main

import (
	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/metrics"
)

func buildSummaryMetrics(m metrics.ReportMetrics, cs metrics.CodeStats) ai.SummaryMetrics {
	sm := ai.SummaryMetrics{
		TotalCommits:       m.TotalCommits,
		TotalAuthors:       m.TotalAuthors,
		BreakingChanges:    m.BreakingChanges,
		SignificanceCounts: m.SignificanceCounts,
		TypeCounts:         m.TypeCounts,
		DateRange:          m.DateRange,
		CommitsPerDay:      m.Velocity.CommitsPerDay,
		MostActiveDay:      m.Velocity.MostActiveDay,
		MostActiveDayCount: m.Velocity.MostActiveDayCount,
		FilesTouched:       cs.TotalFiles,
		LinesAdded:         cs.TotalAdditions,
		LinesDeleted:       cs.TotalDeletions,
		NetLines:           cs.NetLines,
		JiraTicketsLinked:  m.JiraTicketsLinked,
	}
	limit := 5
	if len(m.Authors) < limit {
		limit = len(m.Authors)
	}
	for i := 0; i < limit; i++ {
		sm.TopContributors = append(sm.TopContributors, ai.ContributorStat{
			Name:    m.Authors[i].Name,
			Commits: m.Authors[i].Commits,
		})
	}

	totalChurn := cs.TotalAdditions + cs.TotalDeletions

	if cs.TotalFileChanges > 0 && len(cs.Hotspots) > 0 {
		topN := 5
		if len(cs.Hotspots) < topN {
			topN = len(cs.Hotspots)
		}
		topChanges := 0
		for i := 0; i < topN; i++ {
			topChanges += cs.Hotspots[i].Changes
		}
		sm.HotspotDensity = float64(topChanges) / float64(cs.TotalFileChanges) * 100
	}

	netAbs := cs.NetLines
	if netAbs < 0 {
		netAbs = -netAbs
	}
	if netAbs > 0 {
		sm.ChurnFactor = float64(totalChurn) / float64(netAbs)
	}

	featCommits := m.TypeCounts["feat"]
	if featCommits > 0 {
		sm.ComplexityPerFeat = float64(totalChurn) / float64(featCommits)
	}

	sm.ReleaseCommitSpanHours = m.Velocity.ReleaseCommitSpanHours
	sm.ReleaseAgeHours = m.Velocity.ReleaseAgeHours
	sm.OwnershipConc = m.OwnershipConcentration
	sm.OwnershipEntropy = m.OwnershipEntropy
	sm.ReleaseContributionConcentration = m.ReleaseContributionConcentration
	sm.BatchFactor = m.Velocity.BatchFactor
	sm.RevertRate = m.RevertRate
	sm.ScopeIsolation = m.CommitQuality.ScopeRatio * 100

	if m.TotalCommits > 0 {
		sm.CrossCuttingPct = float64(cs.CrossCuttingCommits) / float64(m.TotalCommits) * 100
	}

	featCommits = m.TypeCounts["feat"]
	fixCommits := m.TypeCounts["fix"]
	refactorCommits := m.TypeCounts["refactor"]

	if featCommits > 0 {
		sm.FixToFeatureRatio = float64(fixCommits) / float64(featCommits)
	}

	if cs.SourceFiles > 0 {
		sm.TestToSourceRatio = float64(cs.TestFiles) / float64(cs.SourceFiles) * 100
	}

	if m.TotalCommits > 0 {
		sm.RefactoringRatio = float64(refactorCommits+fixCommits) / float64(m.TotalCommits) * 100
	}

	sm.APISurfaceChange = cs.APIFilesChanged

	if cs.TotalFiles > 0 {
		sm.FileVolatility = float64(cs.TotalFileChanges) / float64(cs.TotalFiles)
	}

	sm.ChangeComplexityProxy = computeChangeComplexityProxy(cs, sm.ChurnFactor)
	sm.CrossCuttingChangeRisk = computeCrossCuttingChangeRisk(sm)
	sm.TechnicalDebtUSD = computeTechnicalDebt(totalChurn, featCommits, fixCommits, refactorCommits, sm.ChangeComplexityProxy)
	sm.HotspotScore = computeHotspotScore(sm)

	totalTouchedFiles := cs.TestFiles + cs.SourceFiles
	if totalTouchedFiles > 0 {
		sm.TouchedTestFileRatio = float64(cs.TestFiles) / float64(totalTouchedFiles) * 100
	}

	// Release risk score: breaking changes + hotspot density + ownership concentration
	breakingPoints := float64(m.BreakingChanges) * riskBreakingWeight
	if breakingPoints > riskBreakingMax {
		breakingPoints = riskBreakingMax
	}
	hotspotPoints := sm.HotspotDensity * riskHotspotWeight
	if hotspotPoints > riskHotspotMax {
		hotspotPoints = riskHotspotMax
	}
	ownershipPoints := sm.OwnershipConc * riskOwnershipWeight
	if ownershipPoints > riskOwnershipMax {
		ownershipPoints = riskOwnershipMax
	}
	sm.ReleaseRiskScore = breakingPoints + hotspotPoints + ownershipPoints

	// Extended metrics
	sm.CommitSizeSmall = m.CommitSizeDist.Small
	sm.CommitSizeMedium = m.CommitSizeDist.Medium
	sm.CommitSizeLarge = m.CommitSizeDist.Large
	sm.CommitSizeHuge = m.CommitSizeDist.Huge
	sm.ReviewLoad = m.ReviewLoad
	sm.DeleteRatio = cs.DeleteRatio
	sm.CommitMsgQuality = m.CommitMsgQuality
	sm.AuthorOverlap = cs.AuthorOverlap
	sm.PeakHourSpread = m.PeakHourSpread
	sm.DirectoryBreadth = cs.DirectoryBreadth
	sm.ConfigChurnRate = cs.ConfigChurnRate
	sm.CodeFreshness = cs.CodeFreshness
	sm.RhythmConsistency = m.RhythmConsistency
	sm.LanguageMix = m.LanguageMix

	return sm
}

func computeChangeComplexityProxy(cs metrics.CodeStats, churnFactor float64) float64 {
	if cs.AvgLinesPerCommit <= 0 {
		return 0
	}
	complexity := (cs.AvgLinesPerCommit / changeComplexityLinesDivisor) *
		(1 + cs.AvgFilesPerCommit/changeComplexityFilesDivisor) *
		(1 + churnFactor/changeComplexityChurnDivisor)
	return complexity
}

func computeCrossCuttingChangeRisk(sm ai.SummaryMetrics) float64 {
	risk := sm.CrossCuttingPct * crossCuttingChangeWeight
	risk += (1 - sm.ScopeIsolation/100) * crossCuttingScopeWeight
	apiComponent := float64(sm.APISurfaceChange) * crossCuttingAPIWeight
	if apiComponent > crossCuttingAPIMax {
		apiComponent = crossCuttingAPIMax
	}
	risk += apiComponent
	if risk > 100 {
		risk = 100
	}
	return risk
}

func computeTechnicalDebt(totalChurn int, featCommits, fixCommits, refactorCommits int, changeComplexityProxy float64) float64 {
	base := float64(totalChurn) * debtChurnCost
	base += float64(refactorCommits) * debtRefactorCost
	base += float64(fixCommits) * debtFixCost
	multiplier := 1 + changeComplexityProxy/debtComplexityDivisor
	return base * multiplier
}

func computeHotspotScore(sm ai.SummaryMetrics) float64 {
	score := sm.HotspotDensity * hotspotDensityWeight
	churnComponent := sm.ChurnFactor * hotspotChurnWeight
	if churnComponent > hotspotChurnMax {
		churnComponent = hotspotChurnMax
	}
	score += churnComponent
	volatilityComponent := sm.FileVolatility * hotspotVolatilityWeight
	if volatilityComponent > hotspotVolatilityMax {
		volatilityComponent = hotspotVolatilityMax
	}
	score += volatilityComponent
	if score > 100 {
		score = 100
	}
	return score
}
