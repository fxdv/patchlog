// Package gamify computes contributor achievements, badges, and levels for release gamification.
package gamify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/trends"
)

type Achievement struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Rarity      string `json:"rarity"`
	Emoji       string `json:"emoji"`
}

type ContributorAchievements struct {
	Name         string
	Commits      int
	Level        int
	LevelName    string
	Badges       []Badge
	Achievements []Achievement
	TotalCommits int
	Streak       int
}

type ContributorStats struct {
	Name          string   `json:"name"`
	Commits       int      `json:"commits"`
	Level         int      `json:"level"`
	LevelName     string   `json:"level_name"`
	FeatCount     int      `json:"feat_count"`
	FixCount      int      `json:"fix_count"`
	RefactorCount int      `json:"refactor_count"`
	TotalCommits  int      `json:"total_commits"`
	Streak        int      `json:"streak"`
	Badges        []string `json:"badges"`
}

func GenerateAchievements(ctx context.Context, results []ContributorResult, snapshots []trends.Snapshot, client ai.Client, lang string) []ContributorAchievements {
	if client == nil || len(results) == 0 {
		return fallbackAchievements(results, snapshots)
	}

	stats := buildContributorStats(results, snapshots)
	prompt := buildAchievementPrompt(stats, lang)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return fallbackAchievements(results, snapshots)
	}

	parsed := parseAchievementResponse(resp)
	if len(parsed) == 0 {
		return fallbackAchievements(results, snapshots)
	}

	output := make([]ContributorAchievements, len(results))
	for i, r := range results {
		output[i] = ContributorAchievements{
			Name:         r.Name,
			Commits:      r.Commits,
			Level:        r.Level,
			LevelName:    r.LevelName,
			Badges:       r.Badges,
			TotalCommits: stats[r.Name].TotalCommits,
			Streak:       stats[r.Name].Streak,
		}
		if achs, ok := parsed[r.Name]; ok {
			output[i].Achievements = achs
		}
	}

	return output
}

func buildContributorStats(results []ContributorResult, snapshots []trends.Snapshot) map[string]*ContributorStats {
	stats := make(map[string]*ContributorStats)

	totalCommitsMap := make(map[string]int)
	streakMap := make(map[string]int)
	for _, snap := range snapshots {
		for _, c := range snap.TopContributors {
			totalCommitsMap[c.Name] += c.Commits
			streakMap[c.Name]++
		}
	}

	for _, r := range results {
		totalCommitsMap[r.Name] += r.Commits
		streakMap[r.Name]++

		s := &ContributorStats{
			Name:         r.Name,
			Commits:      r.Commits,
			Level:        r.Level,
			LevelName:    r.LevelName,
			TotalCommits: totalCommitsMap[r.Name],
			Streak:       streakMap[r.Name],
		}
		for _, b := range r.Badges {
			s.Badges = append(s.Badges, b.Name)
		}
		stats[r.Name] = s
	}

	return stats
}

func buildAchievementPrompt(stats map[string]*ContributorStats, lang string) string {
	langInstruction := "Respond in English."
	if lang == "ru" {
		langInstruction = "Отвечай на русском языке."
	} else if lang == "zh" {
		langInstruction = "请用中文回答。"
	}

	var sb strings.Builder
	sb.WriteString("You are a game designer creating fun, personalized achievements for software developers.\n")
	sb.WriteString(langInstruction + "\n\n")
	sb.WriteString("For each contributor, generate 1-3 creative achievements based on their stats.\n")
	sb.WriteString("Achievements should be:\n")
	sb.WriteString("- Witty and specific to what they actually did\n")
	sb.WriteString("- Like game achievements (e.g. 'Архитектор Пресетов', 'Покоритель Recharts')\n")
	sb.WriteString("- Include a fun description explaining WHY they earned it\n")
	sb.WriteString("\nRarity levels: legendary, epic, rare, common\n")
	sb.WriteString("- legendary: outstanding contribution (most commits + multiple badges)\n")
	sb.WriteString("- epic: significant achievement (high commit count or notable pattern)\n")
	sb.WriteString("- rare: interesting pattern or milestone\n")
	sb.WriteString("- common: basic participation\n")
	sb.WriteString("\nEmoji: pick a fitting emoji for each achievement.\n\n")
	sb.WriteString("Respond with ONLY a JSON object. Keys are contributor names, values are arrays:\n")
	sb.WriteString(`{"Contributor Name": [{"title": "...", "description": "...", "rarity": "legendary", "emoji": "🏆"}]}` + "\n\n")
	sb.WriteString("Contributors:\n\n")

	for _, s := range stats {
		fmt.Fprintf(&sb, "Name: %s\n", s.Name)
		fmt.Fprintf(&sb, "  Commits this release: %d\n", s.Commits)
		fmt.Fprintf(&sb, "  Total commits (all releases): %d\n", s.TotalCommits)
		fmt.Fprintf(&sb, "  Level: %d (%s)\n", s.Level, s.LevelName)
		fmt.Fprintf(&sb, "  Streak: %d releases\n", s.Streak)
		if len(s.Badges) > 0 {
			fmt.Fprintf(&sb, "  Badges: %s\n", strings.Join(s.Badges, ", "))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with ONLY the JSON object, no other text.")
	return sb.String()
}

func parseAchievementResponse(resp string) map[string][]Achievement {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start < 0 || end < 0 || end <= start {
		return nil
	}

	var parsed map[string][]Achievement
	if err := json.Unmarshal([]byte(resp[start:end+1]), &parsed); err != nil {
		return nil
	}

	validRarities := map[string]bool{"legendary": true, "epic": true, "rare": true, "common": true}
	for name, achs := range parsed {
		for i := range achs {
			if !validRarities[achs[i].Rarity] {
				achs[i].Rarity = "common"
			}
		}
		parsed[name] = achs
	}

	return parsed
}

func fallbackAchievements(results []ContributorResult, snapshots []trends.Snapshot) []ContributorAchievements {
	output := make([]ContributorAchievements, len(results))
	for i, r := range results {
		var achs []Achievement
		for _, b := range r.Badges {
			rarity := "common"
			if b.Name == "Release Hero" {
				rarity = "legendary"
			} else if b.Name == "Feature Machine" || b.Name == "Bug Slayer" {
				rarity = "epic"
			} else if b.Name == "First Blood" || b.Name == "Streak Keeper" {
				rarity = "rare"
			}
			achs = append(achs, Achievement{
				Title:       b.Name,
				Description: b.Reason,
				Rarity:      rarity,
				Emoji:       b.Emoji,
			})
		}
		output[i] = ContributorAchievements{
			Name:         r.Name,
			Commits:      r.Commits,
			Level:        r.Level,
			LevelName:    r.LevelName,
			Badges:       r.Badges,
			Achievements: achs,
		}
	}
	return output
}
