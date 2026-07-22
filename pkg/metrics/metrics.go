package metrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/render"
)

// ReportMetrics holds computed metrics for a release.
type ReportMetrics struct {
	Version                          string
	TotalCommits                     int
	TotalAuthors                     int
	Authors                          []AuthorStat
	BreakingChanges                  int
	SignificanceCounts               map[string]int
	TypeCounts                       map[string]int
	JiraTicketsLinked                int
	CommitsWithJira                  int
	AvgCommitBodyLen                 float64
	ConventionalRatio                float64
	ScopeUsage                       map[string]int
	DateRange                        string
	GeneratedAt                      time.Time
	CommitQuality                    CommitQuality
	Velocity                         VelocityStats
	OwnershipConcentration           float64
	OwnershipEntropy                 float64
	ReleaseContributionConcentration int
	RevertRate                       float64
	CommitSizeDist                   CommitSizeDistribution
	ReviewLoad                       float64
	CommitMsgQuality                 float64
	PeakHourSpread                   float64
	RhythmConsistency                float64
	LanguageMix                      float64
}

// CommitQuality measures conventional commit discipline.
type CommitQuality struct {
	AvgHeaderLen     float64
	CommitsWithBody  int
	CommitsWithScope int
	BodyRatio        float64
	ScopeRatio       float64
	BreakingCount    int
	RevertCount      int
}

// VelocityStats measures development pace and rhythm.
type VelocityStats struct {
	CommitsPerDay          float64
	AvgHoursBetween        float64
	MostActiveDay          string
	MostActiveDayCount     int
	WeekendRatio           float64
	ReleaseCommitSpanHours float64
	ReleaseAgeHours        float64
	BatchFactor            float64
}

// AuthorStat holds per-author commit counts.
type AuthorStat struct {
	Name    string
	Commits int
}

// CodeStats holds file-level code change statistics.
type CodeStats struct {
	TotalFiles          int
	TotalAdditions      int
	TotalDeletions      int
	NetLines            int
	TotalFileChanges    int
	FilesByType         map[string]int
	LargestChange       string
	LargestLines        int
	Hotspots            []FileHotspot
	TestFiles           int
	SourceFiles         int
	APIFilesChanged     int
	CrossCuttingCommits int
	AvgLinesPerCommit   float64
	AvgFilesPerCommit   float64
	DeleteRatio         float64
	AuthorOverlap       float64
	DirectoryBreadth    float64
	ConfigChurnRate     float64
	CodeFreshness       float64
}

// FileHotspot identifies frequently changed files.
type FileHotspot struct {
	Path    string
	Changes int
	Lines   int
}

// CommitSizeDistribution counts commits by file-change bucket.
type CommitSizeDistribution struct {
	Small  int
	Medium int
	Large  int
	Huge   int
}

func ComputeReportMetrics(report render.Report, commits []commit.Commit) ReportMetrics {
	m := ReportMetrics{
		Version:            report.Version,
		GeneratedAt:        time.Now(),
		SignificanceCounts: make(map[string]int),
		TypeCounts:         make(map[string]int),
		ScopeUsage:         make(map[string]int),
	}

	m.TotalCommits = len(commits)
	m.BreakingChanges = len(report.Breaking)

	authorMap := make(map[string]int)
	totalBodyLen := 0
	conventionalCount := 0
	commitsWithJira := 0
	commitsWithBody := 0
	commitsWithScope := 0
	totalHeaderLen := 0
	revertCount := 0

	for _, c := range commits {
		authorMap[c.Author]++
		if c.Type != "other" {
			conventionalCount++
		}
		if len(c.JiraKeys) > 0 {
			commitsWithJira++
		}
		if c.Body != "" {
			commitsWithBody++
		}
		if c.Scope != "" {
			commitsWithScope++
		}
		if c.Breaking {
			m.CommitQuality.BreakingCount++
		}
		if c.Type == "revert" {
			revertCount++
		}
		totalBodyLen += len(c.Body)
		totalHeaderLen += len(c.Header)
		if c.Significance != "" && c.Significance != "skip" {
			m.SignificanceCounts[c.Significance]++
		}
		m.TypeCounts[c.Type]++
		if c.Scope != "" {
			m.ScopeUsage[c.Scope]++
		}
	}

	m.TotalAuthors = len(authorMap)
	for name, count := range authorMap {
		m.Authors = append(m.Authors, AuthorStat{Name: name, Commits: count})
	}
	sort.Slice(m.Authors, func(i, j int) bool {
		if m.Authors[i].Commits != m.Authors[j].Commits {
			return m.Authors[i].Commits > m.Authors[j].Commits
		}
		return m.Authors[i].Name < m.Authors[j].Name
	})

	if m.TotalCommits > 0 {
		var hhi float64
		for _, a := range m.Authors {
			share := float64(a.Commits) / float64(m.TotalCommits)
			hhi += share * share
		}
		m.OwnershipConcentration = hhi * 100

		if m.TotalAuthors > 1 {
			var entropy float64
			for _, a := range m.Authors {
				p := float64(a.Commits) / float64(m.TotalCommits)
				if p > 0 {
					entropy -= p * math.Log2(p)
				}
			}
			m.OwnershipEntropy = entropy / math.Log2(float64(m.TotalAuthors))
		} else {
			m.OwnershipEntropy = 0
		}

		threshold := 0.8 * float64(m.TotalCommits)
		cumulative := 0
		for _, a := range m.Authors {
			m.ReleaseContributionConcentration++
			cumulative += a.Commits
			if float64(cumulative) >= threshold {
				break
			}
		}
	}

	if m.TotalCommits > 0 {
		m.AvgCommitBodyLen = float64(totalBodyLen) / float64(m.TotalCommits)
		m.ConventionalRatio = float64(conventionalCount) / float64(m.TotalCommits)
		m.CommitQuality.AvgHeaderLen = float64(totalHeaderLen) / float64(m.TotalCommits)
		m.CommitQuality.CommitsWithBody = commitsWithBody
		m.CommitQuality.CommitsWithScope = commitsWithScope
		m.CommitQuality.BodyRatio = float64(commitsWithBody) / float64(m.TotalCommits)
		m.CommitQuality.ScopeRatio = float64(commitsWithScope) / float64(m.TotalCommits)
		m.CommitQuality.RevertCount = revertCount
		m.RevertRate = float64(revertCount) / float64(m.TotalCommits) * 100
	}

	report.ForEachItem(func(item *render.Item) {
		m.JiraTicketsLinked += len(item.JiraIssues)
	})
	m.CommitsWithJira = commitsWithJira

	if len(commits) > 0 {
		earliest := commits[0].Timestamp
		latest := commits[0].Timestamp
		for _, c := range commits {
			if c.Timestamp.Before(earliest) {
				earliest = c.Timestamp
			}
			if c.Timestamp.After(latest) {
				latest = c.Timestamp
			}
		}
		if !earliest.IsZero() && !latest.IsZero() {
			m.DateRange = fmt.Sprintf("%s to %s", earliest.Format("2006-01-02"), latest.Format("2006-01-02"))
			m.Velocity = computeVelocity(commits)
		}
	}

	computeExtendedMetrics(&m, commits)

	return m
}

func computeVelocity(commits []commit.Commit) VelocityStats {
	var v VelocityStats
	if len(commits) == 0 {
		return v
	}

	earliest := commits[0].Timestamp
	latest := commits[0].Timestamp
	for _, c := range commits {
		if c.Timestamp.Before(earliest) {
			earliest = c.Timestamp
		}
		if c.Timestamp.After(latest) {
			latest = c.Timestamp
		}
	}
	duration := latest.Sub(earliest)
	days := duration.Hours() / 24
	v.ReleaseCommitSpanHours = duration.Hours()
	v.ReleaseAgeHours = time.Since(earliest).Hours()
	if days > 0 {
		v.CommitsPerDay = float64(len(commits)) / days
	}

	if len(commits) > 1 {
		var totalGap float64
		gaps := make([]float64, 0, len(commits)-1)
		for i := 1; i < len(commits); i++ {
			gap := commits[i].Timestamp.Sub(commits[i-1].Timestamp).Hours()
			gaps = append(gaps, gap)
			totalGap += gap
		}
		mean := totalGap / float64(len(commits)-1)
		v.AvgHoursBetween = mean

		if mean > 0 {
			var sumSqDiff float64
			for _, g := range gaps {
				diff := g - mean
				sumSqDiff += diff * diff
			}
			stddev := math.Sqrt(sumSqDiff / float64(len(gaps)))
			v.BatchFactor = stddev / mean
		}
	}

	dayCounts := make(map[string]int)
	weekendCount := 0
	for _, c := range commits {
		dayName := c.Timestamp.Format("Mon")
		dayCounts[dayName]++
		if c.Timestamp.Weekday() == time.Saturday || c.Timestamp.Weekday() == time.Sunday {
			weekendCount++
		}
	}

	maxCount := 0
	for day, count := range dayCounts {
		if count > maxCount {
			maxCount = count
			v.MostActiveDay = day
			v.MostActiveDayCount = count
		}
	}

	if len(commits) > 0 {
		v.WeekendRatio = float64(weekendCount) / float64(len(commits))
	}

	return v
}

func ComputeCodeStats(ctx context.Context, fetcher *gitlog.Fetcher, commits []commit.Commit) CodeStats {
	var cs CodeStats
	cs.FilesByType = make(map[string]int)
	seenFiles := make(map[string]bool)
	fileChurn := make(map[string]int)
	fileLines := make(map[string]int)
	dirSet := make(map[string]bool)
	configFilesChanged := 0
	fileAuthors := make(map[string]map[string]bool)

	// Batch fetch all diff stats in a single git call for performance.
	// Falls back to per-commit GetDiffStat if batch fails.
	batchStats, batchErr := fetcher.BatchDiffStats(ctx, "", "")
	batchMode := batchErr == nil && len(batchStats) > 0

	for _, c := range commits {
		var stat *gitlog.DiffStat
		if batchMode {
			stat = batchStats[c.Hash]
		}
		if stat == nil {
			var err error
			stat, err = fetcher.GetDiffStat(ctx, c.Hash)
			if err != nil {
				continue
			}
		}
		cs.TotalAdditions += stat.Insertions
		cs.TotalDeletions += stat.Deletions
		lines := stat.Insertions + stat.Deletions
		if lines > cs.LargestLines {
			cs.LargestLines = lines
			cs.LargestChange = c.Header
		}
		commitCategories := make(map[string]bool)
		for _, f := range stat.Files {
			if !seenFiles[f] {
				seenFiles[f] = true
				cs.TotalFiles++
				ext := fileExtension(f)
				cs.FilesByType[ext]++
				if isTestFile(f) {
					cs.TestFiles++
				} else {
					cs.SourceFiles++
				}
				if isAPIFile(f) {
					cs.APIFilesChanged++
				}
			}
			fileChurn[f]++
			fileLines[f] += lines
			cs.TotalFileChanges++
			commitCategories[fileCategory(f)] = true
			dirSet[topDirectory(f)] = true
			if isConfigFile(f) {
				configFilesChanged++
			}
			if fileAuthors[f] == nil {
				fileAuthors[f] = make(map[string]bool)
			}
			fileAuthors[f][c.Author] = true
		}
		if len(commitCategories) > 1 {
			cs.CrossCuttingCommits++
		}
	}

	cs.NetLines = cs.TotalAdditions - cs.TotalDeletions

	cs.Hotspots = computeHotspots(fileChurn, fileLines, 10)

	if len(commits) > 0 {
		totalLines := cs.TotalAdditions + cs.TotalDeletions
		cs.AvgLinesPerCommit = float64(totalLines) / float64(len(commits))
		cs.AvgFilesPerCommit = float64(cs.TotalFileChanges) / float64(len(commits))
	}

	// --- Delete Ratio ---
	totalChurn := cs.TotalAdditions + cs.TotalDeletions
	if totalChurn > 0 {
		cs.DeleteRatio = float64(cs.TotalDeletions) / float64(totalChurn)
	}

	// --- Author Overlap ---
	if len(fileAuthors) > 0 {
		overlapCount := 0
		for _, authors := range fileAuthors {
			if len(authors) > 1 {
				overlapCount++
			}
		}
		cs.AuthorOverlap = float64(overlapCount) / float64(len(fileAuthors)) * 100
	}

	// --- Directory Breadth ---
	if cs.TotalFiles > 0 && len(dirSet) > 0 {
		cs.DirectoryBreadth = float64(len(dirSet)) / float64(cs.TotalFiles) * 100
	}

	// --- Config Churn Rate ---
	if cs.TotalFileChanges > 0 {
		cs.ConfigChurnRate = float64(configFilesChanged) / float64(cs.TotalFileChanges) * 100
	}

	// --- Code Freshness ---
	if len(commits) > 0 {
		earliest := commits[0].Timestamp
		for _, c := range commits {
			if c.Timestamp.Before(earliest) {
				earliest = c.Timestamp
			}
		}
		cs.CodeFreshness = time.Since(earliest).Hours() / 24
	}

	return cs
}

func computeHotspots(fileChurn map[string]int, fileLines map[string]int, limit int) []FileHotspot {
	type entry struct {
		path  string
		count int
		lines int
	}
	var entries []entry
	for path, count := range fileChurn {
		entries = append(entries, entry{path: path, count: count, lines: fileLines[path]})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	var hotspots []FileHotspot
	for _, e := range entries {
		hotspots = append(hotspots, FileHotspot{Path: e.path, Changes: e.count, Lines: e.lines})
	}
	return hotspots
}

func fileExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return "other"
	}
	return path[idx:]
}

func isTestFile(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, "_test.go") ||
		strings.HasSuffix(p, "_test.py") ||
		strings.HasSuffix(p, ".test.js") ||
		strings.HasSuffix(p, ".test.ts") ||
		strings.HasSuffix(p, ".test.tsx") ||
		strings.HasSuffix(p, ".spec.js") ||
		strings.HasSuffix(p, ".spec.ts") ||
		strings.HasSuffix(p, ".spec.tsx") ||
		strings.Contains(p, "/test/") ||
		strings.Contains(p, "/tests/") ||
		strings.Contains(p, "/__tests__/")
}

func isAPIFile(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "openapi.yaml") ||
		strings.Contains(p, "openapi.json") ||
		strings.Contains(p, "swagger.") ||
		strings.HasSuffix(p, ".proto") ||
		strings.HasSuffix(p, ".graphql") ||
		strings.Contains(p, "/api/v1/") ||
		strings.Contains(p, "/api/v2/") ||
		strings.Contains(p, "/public/api/")
}

func fileCategory(path string) string {
	p := strings.ToLower(path)
	if isTestFile(path) {
		return "test"
	}
	if strings.Contains(p, "/client/") || strings.Contains(p, "/frontend/") {
		return "client"
	}
	if strings.Contains(p, "/server/") || strings.Contains(p, "/backend/") {
		return "server"
	}
	if strings.Contains(p, "/worker/") {
		return "worker"
	}
	if strings.Contains(p, "migration") || strings.HasSuffix(p, ".sql") {
		return "database"
	}
	if strings.HasSuffix(p, ".md") || strings.Contains(p, "/docs/") {
		return "docs"
	}
	return "other"
}

func FormatMetricsMarkdown(m ReportMetrics, cs CodeStats) string {
	var buf strings.Builder

	buf.WriteString("## 📊 Release Metrics\n\n")

	buf.WriteString("### Overview\n\n")
	fmt.Fprintf(&buf, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&buf, "| Total commits | %d |\n", m.TotalCommits)
	fmt.Fprintf(&buf, "| Unique authors | %d |\n", m.TotalAuthors)
	fmt.Fprintf(&buf, "| Breaking changes | %d |\n", m.BreakingChanges)
	if m.DateRange != "" {
		fmt.Fprintf(&buf, "| Date range | %s |\n", m.DateRange)
	}
	fmt.Fprintf(&buf, "| Conventional commit ratio | %.0f%% |\n", m.ConventionalRatio*100)
	fmt.Fprintf(&buf, "| Commits with Jira refs | %d |\n", m.CommitsWithJira)
	fmt.Fprintf(&buf, "| Jira tickets linked | %d |\n", m.JiraTicketsLinked)
	fmt.Fprintf(&buf, "| Avg commit body length | %.0f chars |\n\n", m.AvgCommitBodyLen)

	if m.TotalCommits > 0 {
		buf.WriteString("### Commit Quality\n\n")
		fmt.Fprintf(&buf, "| Metric | Value |\n|--------|-------|\n")
		fmt.Fprintf(&buf, "| Avg header length | %.0f chars |\n", m.CommitQuality.AvgHeaderLen)
		fmt.Fprintf(&buf, "| Commits with body | %d (%.0f%%) |\n", m.CommitQuality.CommitsWithBody, m.CommitQuality.BodyRatio*100)
		fmt.Fprintf(&buf, "| Commits with scope | %d (%.0f%%) |\n", m.CommitQuality.CommitsWithScope, m.CommitQuality.ScopeRatio*100)
		fmt.Fprintf(&buf, "| Breaking changes | %d |\n", m.CommitQuality.BreakingCount)
		fmt.Fprintf(&buf, "| Reverts | %d |\n\n", m.CommitQuality.RevertCount)
	}

	if m.Velocity.CommitsPerDay > 0 {
		buf.WriteString("### Velocity\n\n")
		fmt.Fprintf(&buf, "| Metric | Value |\n|--------|-------|\n")
		fmt.Fprintf(&buf, "| Commits per day | %.1f |\n", m.Velocity.CommitsPerDay)
		fmt.Fprintf(&buf, "| Avg hours between commits | %.1f |\n", m.Velocity.AvgHoursBetween)
		if m.Velocity.MostActiveDay != "" {
			fmt.Fprintf(&buf, "| Most active day | %s (%d commits) |\n", m.Velocity.MostActiveDay, m.Velocity.MostActiveDayCount)
		}
		fmt.Fprintf(&buf, "| Weekend commit ratio | %.0f%% |\n\n", m.Velocity.WeekendRatio*100)
	}

	if len(m.Authors) > 0 {
		buf.WriteString("### Contributors\n\n")
		buf.WriteString("| Author | Commits |\n|--------|---------|\n")
		for _, a := range m.Authors {
			fmt.Fprintf(&buf, "| %s | %d |\n", a.Name, a.Commits)
		}
		buf.WriteString("\n")
	}

	if len(m.TypeCounts) > 0 {
		buf.WriteString("### Commit Types\n\n")
		buf.WriteString("| Type | Count |\n|------|-------|\n")
		for _, t := range []string{"feat", "fix", "perf", "refactor", "revert", "docs", "test", "style", "ci", "chore", "other"} {
			if c, ok := m.TypeCounts[t]; ok && c > 0 {
				fmt.Fprintf(&buf, "| %s | %d |\n", t, c)
			}
		}
		buf.WriteString("\n")
	}

	if len(m.SignificanceCounts) > 0 {
		buf.WriteString("### Impact Distribution\n\n")
		buf.WriteString("| Level | Count |\n|-------|-------|\n")
		for _, lvl := range []string{"major", "minor", "patch"} {
			if c, ok := m.SignificanceCounts[lvl]; ok && c > 0 {
				fmt.Fprintf(&buf, "| %s | %d |\n", lvl, c)
			}
		}
		buf.WriteString("\n")
	}

	if len(m.ScopeUsage) > 0 {
		buf.WriteString("### Scope Usage\n\n")
		buf.WriteString("| Scope | Count |\n|-------|-------|\n")
		scopes := make([]string, 0, len(m.ScopeUsage))
		for scope := range m.ScopeUsage {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		for _, scope := range scopes {
			fmt.Fprintf(&buf, "| %s | %d |\n", scope, m.ScopeUsage[scope])
		}
		buf.WriteString("\n")
	}

	buf.WriteString("### Code Changes\n\n")
	fmt.Fprintf(&buf, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&buf, "| Files touched | %d |\n", cs.TotalFiles)
	fmt.Fprintf(&buf, "| Lines added | %d |\n", cs.TotalAdditions)
	fmt.Fprintf(&buf, "| Lines deleted | %d |\n", cs.TotalDeletions)
	fmt.Fprintf(&buf, "| Net lines | %+d |\n", cs.NetLines)
	if cs.LargestLines > 0 {
		fmt.Fprintf(&buf, "| Largest change | %s (%d lines) |\n", cs.LargestChange, cs.LargestLines)
	}

	if len(cs.FilesByType) > 0 {
		buf.WriteString("\n### Files by Type\n\n")
		buf.WriteString("| Extension | Count |\n|-----------|-------|\n")
		exts := make([]string, 0, len(cs.FilesByType))
		for ext := range cs.FilesByType {
			exts = append(exts, ext)
		}
		sort.Strings(exts)
		for _, ext := range exts {
			fmt.Fprintf(&buf, "| %s | %d |\n", ext, cs.FilesByType[ext])
		}
	}

	if len(cs.Hotspots) > 0 {
		buf.WriteString("\n### File Hotspots (Most Changed)\n\n")
		buf.WriteString("| File | Changes | Lines Touched |\n|------|---------|---------------|\n")
		for _, h := range cs.Hotspots {
			fmt.Fprintf(&buf, "| %s | %d | %d |\n", h.Path, h.Changes, h.Lines)
		}
	}

	return buf.String()
}

func computeExtendedMetrics(m *ReportMetrics, commits []commit.Commit) {
	if len(commits) == 0 {
		return
	}

	// --- Commit Size Distribution ---
	var commitSizes []int
	var largestCommitSize int
	for _, c := range commits {
		size := c.ChangedFiles
		commitSizes = append(commitSizes, size)
		if size > largestCommitSize {
			largestCommitSize = size
		}
	}
	for _, size := range commitSizes {
		switch {
		case size < 3:
			m.CommitSizeDist.Small++
		case size < 10:
			m.CommitSizeDist.Medium++
		case size < 30:
			m.CommitSizeDist.Large++
		default:
			m.CommitSizeDist.Huge++
		}
	}

	// --- Review Load ---
	avgSize := 0.0
	for _, s := range commitSizes {
		avgSize += float64(s)
	}
	avgSize /= float64(len(commitSizes))
	if avgSize > 0 {
		m.ReviewLoad = float64(largestCommitSize) / avgSize
	}

	// --- Commit Message Quality ---
	bodyScore := m.CommitQuality.BodyRatio * 25
	scopeScore := m.CommitQuality.ScopeRatio * 25
	convScore := m.ConventionalRatio * 25
	avgLenScore := 0.0
	if m.CommitQuality.AvgHeaderLen > 0 {
		switch {
		case m.CommitQuality.AvgHeaderLen >= 30:
			avgLenScore = 25
		case m.CommitQuality.AvgHeaderLen >= 15:
			avgLenScore = 15
		default:
			avgLenScore = 5
		}
	}
	m.CommitMsgQuality = bodyScore + scopeScore + convScore + avgLenScore

	// --- Peak Hour Spread ---
	var hours []float64
	for _, c := range commits {
		hours = append(hours, float64(c.Timestamp.Hour()))
	}
	if len(hours) > 1 {
		var sum float64
		for _, h := range hours {
			sum += h
		}
		mean := sum / float64(len(hours))
		var sqDiff float64
		for _, h := range hours {
			sqDiff += (h - mean) * (h - mean)
		}
		m.PeakHourSpread = math.Sqrt(sqDiff / float64(len(hours)))
	}

	// --- Language Mix ---
	cyrillicCount := 0
	for _, c := range commits {
		for _, r := range c.Header {
			if r >= '\u0400' && r <= '\u04FF' {
				cyrillicCount++
				break
			}
		}
	}
	m.LanguageMix = float64(cyrillicCount) / float64(len(commits))

	// --- Rhythm Consistency ---
	if len(commits) > 2 {
		var gaps []float64
		for i := 1; i < len(commits); i++ {
			gaps = append(gaps, commits[i].Timestamp.Sub(commits[i-1].Timestamp).Hours())
		}
		if len(gaps) > 0 {
			var sum float64
			for _, g := range gaps {
				sum += g
			}
			mean := sum / float64(len(gaps))
			if mean > 0 {
				var sqDiff float64
				for _, g := range gaps {
					sqDiff += (g - mean) * (g - mean)
				}
				stddev := math.Sqrt(sqDiff / float64(len(gaps)))
				m.RhythmConsistency = math.Max(0, 1-stddev/mean)
			}
		}
	}
}

func topDirectory(path string) string {
	if idx := strings.Index(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

func isConfigFile(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".yaml") ||
		strings.HasSuffix(p, ".yml") ||
		strings.HasSuffix(p, ".toml") ||
		strings.HasSuffix(p, ".ini") ||
		strings.HasSuffix(p, ".env") ||
		(strings.HasSuffix(p, ".json") && (strings.Contains(p, "config") ||
			strings.Contains(p, "settings") || strings.Contains(p, "tsconfig")))
}
