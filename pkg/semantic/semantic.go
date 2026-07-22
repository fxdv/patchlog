// Package semantic generates AI-powered semantic summaries of actual code diffs per release section.
package semantic

import (
	"context"
	"strings"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/render"
)

type Options struct {
	Aggregate    bool
	MaxDiffChars int
}

type SectionSummary struct {
	SectionHeading string
	Summary        string
}

func SummarizeSections(ctx context.Context, report render.Report, diffs map[string]string, client ai.Client, opts Options) map[string]string {
	if client == nil || len(diffs) == 0 {
		return nil
	}

	maxChars := opts.MaxDiffChars
	if maxChars <= 0 {
		maxChars = 4000
	}

	result := make(map[string]string)

	for _, section := range report.Sections {
		if len(section.Items) == 0 {
			continue
		}

		if !opts.Aggregate {
			var summaries []string
			for _, item := range section.Items {
				diff, ok := diffs[item.Hash]
				if !ok || diff == "" {
					continue
				}
				prompt := buildPrompt(section.Heading, []string{item.Description}, truncateDiff(diff, maxChars))
				summary, err := client.Generate(ctx, prompt)
				if err == nil && strings.TrimSpace(summary) != "" {
					summaries = append(summaries, "- "+item.Description+": "+cleanSummary(summary))
				}
			}
			if len(summaries) > 0 {
				result[section.Heading] = strings.Join(summaries, "\n")
			}
			continue
		}

		var combinedDiff strings.Builder
		var itemDescs []string
		for _, item := range section.Items {
			itemDescs = append(itemDescs, item.Description)
			if diff, ok := diffs[item.Hash]; ok {
				truncated := truncateDiff(diff, maxChars)
				combinedDiff.WriteString(truncated)
				combinedDiff.WriteString("\n\n---\n\n")
			}
		}

		if combinedDiff.Len() == 0 {
			continue
		}

		prompt := buildPrompt(section.Heading, itemDescs, combinedDiff.String())
		summary, err := client.Generate(ctx, prompt)
		if err != nil || summary == "" {
			continue
		}

		summary = cleanSummary(summary)

		if summary != "" {
			result[section.Heading] = summary
		}
	}

	return result
}

func cleanSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	summary = strings.TrimPrefix(summary, "```")
	summary = strings.TrimSuffix(summary, "```")
	return strings.TrimSpace(summary)
}

func buildPrompt(heading string, descs []string, diff string) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following code diff and write a concise semantic summary of WHAT changed in the code.\n")
	sb.WriteString("Focus on the actual code changes, not the commit messages.\n")
	sb.WriteString("Describe: what was added, removed, refactored, or fixed.\n")
	sb.WriteString("Keep it to 2-3 sentences. No heading, no markdown.\n\n")
	sb.WriteString("Section: " + heading + "\n\n")
	sb.WriteString("Commit descriptions:\n")
	for _, d := range descs {
		sb.WriteString("- " + d + "\n")
	}
	sb.WriteString("\nCode diff:\n")
	sb.WriteString(diff)
	sb.WriteString("\n\nSemantic summary:")
	return sb.String()
}

func truncateDiff(diff string, maxChars int) string {
	if len(diff) <= maxChars {
		return diff
	}

	lines := strings.Split(diff, "\n")
	var sb strings.Builder
	for _, line := range lines {
		if sb.Len()+len(line)+1 > maxChars {
			sb.WriteString("\n... (diff truncated)")
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func FormatMarkdown(summaries map[string]string) string {
	if len(summaries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Semantic Summary\n\n")
	for heading, summary := range summaries {
		sb.WriteString("### " + heading + "\n\n")
		sb.WriteString("*" + summary + "*\n\n")
	}
	return sb.String()
}
