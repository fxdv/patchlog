package gamify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/metrics"
	"github.com/fxdv/patchlog/pkg/trends"
)

type Badge struct {
	Emoji  string
	Name   string
	Reason string
}

type ContributorResult struct {
	Name      string
	Commits   int
	Badges    []Badge
	Level     int
	LevelName string
}

type Options struct {
	ShowLevels    bool
	ShowStreaks   bool
	NightOwlHour  int
	EarlyBirdHour int
}

func Compute(commits []commit.Commit, reportMetrics metrics.ReportMetrics, snapshots []trends.Snapshot, opts Options) []ContributorResult {
	if len(reportMetrics.Authors) == 0 {
		return nil
	}

	nightOwl := opts.NightOwlHour
	if nightOwl == 0 {
		nightOwl = 22
	}
	earlyBird := opts.EarlyBirdHour
	if earlyBird == 0 {
		earlyBird = 8
	}

	authorTypes := make(map[string]map[string]int)
	authorHours := make(map[string][]int)
	authorCommits := make(map[string]int)

	for _, a := range reportMetrics.Authors {
		authorTypes[a.Name] = make(map[string]int)
	}

	for _, c := range commits {
		if _, ok := authorTypes[c.Author]; ok {
			authorTypes[c.Author][c.Type]++
			authorHours[c.Author] = append(authorHours[c.Author], c.Timestamp.Hour())
		}
	}
	for _, a := range reportMetrics.Authors {
		authorCommits[a.Name] = a.Commits
	}

	totalCommits := make(map[string]int)
	for _, snap := range snapshots {
		for _, c := range snap.TopContributors {
			totalCommits[c.Name] += c.Commits
		}
	}
	for _, a := range reportMetrics.Authors {
		totalCommits[a.Name] += a.Commits
	}

	prevReleases := make(map[string]int)
	for _, snap := range snapshots {
		for _, c := range snap.TopContributors {
			prevReleases[c.Name]++
		}
	}

	maxC := 0
	for _, a := range reportMetrics.Authors {
		if a.Commits > maxC {
			maxC = a.Commits
		}
	}

	var results []ContributorResult
	for _, a := range reportMetrics.Authors {
		result := ContributorResult{
			Name:    a.Name,
			Commits: a.Commits,
		}

		types := authorTypes[a.Name]
		hours := authorHours[a.Name]

		if a.Commits == maxC && maxC > 0 {
			result.Badges = append(result.Badges, Badge{Emoji: "🏆", Name: "Release Hero", Reason: "Most commits"})
		}
		if types["feat"] >= 5 {
			result.Badges = append(result.Badges, Badge{Emoji: "🚀", Name: "Feature Machine", Reason: fmt.Sprintf("%d feat commits", types["feat"])})
		}
		if types["fix"] >= 5 {
			result.Badges = append(result.Badges, Badge{Emoji: "🐛", Name: "Bug Slayer", Reason: fmt.Sprintf("%d fix commits", types["fix"])})
		}
		if types["refactor"] >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "🧹", Name: "Code Cleaner", Reason: fmt.Sprintf("%d refactor commits", types["refactor"])})
		}
		if types["docs"] >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "📝", Name: "Documentation Sage", Reason: fmt.Sprintf("%d docs commits", types["docs"])})
		}
		if types["perf"] > 0 {
			result.Badges = append(result.Badges, Badge{Emoji: "⚡", Name: "Performance Tuner", Reason: "perf optimization"})
		}
		if prevReleases[a.Name] >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "🔥", Name: "Streak Keeper", Reason: fmt.Sprintf("%d consecutive releases", prevReleases[a.Name]+1)})
		}
		if prevReleases[a.Name] == 0 {
			result.Badges = append(result.Badges, Badge{Emoji: "🌱", Name: "First Blood", Reason: "First contribution"})
		}
		if types["test"] >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "🧪", Name: "Test Champion", Reason: fmt.Sprintf("%d test commits", types["test"])})
		}
		nightCount := 0
		earlyCount := 0
		for _, h := range hours {
			if h >= nightOwl {
				nightCount++
			}
			if h < earlyBird {
				earlyCount++
			}
		}
		if nightCount >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "⏰", Name: "Night Owl", Reason: fmt.Sprintf("%d commits after %d:00", nightCount, nightOwl)})
		}
		if earlyCount >= 3 {
			result.Badges = append(result.Badges, Badge{Emoji: "🌅", Name: "Early Bird", Reason: fmt.Sprintf("%d commits before %d:00", earlyCount, earlyBird)})
		}

		tc := totalCommits[a.Name]
		switch {
		case tc >= 100:
			result.Level = 5
			result.LevelName = "Legend"
		case tc >= 51:
			result.Level = 4
			result.LevelName = "Veteran"
		case tc >= 21:
			result.Level = 3
			result.LevelName = "Regular"
		case tc >= 6:
			result.Level = 2
			result.LevelName = "Contributor"
		default:
			result.Level = 1
			result.LevelName = "Newcomer"
		}

		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Commits != results[j].Commits {
			return results[i].Commits > results[j].Commits
		}
		return results[i].Name < results[j].Name
	})

	return results
}

func FormatMarkdown(results []ContributorResult) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 🎮 Contributor Achievements\n\n")
	sb.WriteString("| Contributor | Commits | Badges | Level |\n")
	sb.WriteString("|-------------|---------|--------|-------|\n")
	for _, r := range results {
		badges := ""
		for _, b := range r.Badges {
			badges += b.Emoji + " "
		}
		badges = strings.TrimSpace(badges)
		if badges == "" {
			badges = "—"
		}
		fmt.Fprintf(&sb, "| %s | %d | %s | %s (L%d) |\n", r.Name, r.Commits, badges, r.LevelName, r.Level)
	}
	sb.WriteString("\n")

	var highlights []string
	for _, r := range results {
		for _, b := range r.Badges {
			if b.Name == "Release Hero" || b.Name == "Feature Machine" || b.Name == "First Blood" {
				highlights = append(highlights, fmt.Sprintf("%s **%s**: %s (%s)", b.Emoji, b.Name, r.Name, b.Reason))
			}
		}
	}
	if len(highlights) > 0 {
		sb.WriteString("### Badge Highlights\n")
		for _, h := range highlights {
			fmt.Fprintf(&sb, "%s\n", h)
		}
	}

	return sb.String()
}
