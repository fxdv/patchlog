package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

var reNumberPrefix = regexp.MustCompile(`^\d+\.\s*`)

func GenerateProse(ctx context.Context, report render.Report, tone Tone, aiClient Client) (string, error) {
	return GenerateProseStream(ctx, report, tone, aiClient, nil)
}

func GenerateProseLang(ctx context.Context, report render.Report, tone Tone, aiClient Client, lang i18n.Lang) (string, error) {
	return GenerateProseStreamLang(ctx, report, tone, aiClient, nil, lang)
}

func GenerateSummary(ctx context.Context, report render.Report, aiClient Client, sm SummaryMetrics) (string, error) {
	return GenerateSummaryLang(ctx, report, aiClient, sm, i18n.LangRU)
}

func GenerateSummaryLang(ctx context.Context, report render.Report, aiClient Client, sm SummaryMetrics, lang i18n.Lang) (string, error) {
	if aiClient == nil {
		return "", nil
	}

	prompt := BuildSummaryPrompt(report, sm, lang)
	if prompt == "" {
		return "", nil
	}

	text, err := aiClient.StreamGenerate(ctx, prompt, nil)
	if err != nil {
		return "", fmt.Errorf("AI summary failed: %w", err)
	}
	return strings.TrimSpace(text), nil
}

func EnhanceReport(ctx context.Context, report render.Report, aiClient Client) (render.Report, error) {
	if aiClient == nil {
		return report, nil
	}

	var errs []string
	enhanced := 0

	for i := range report.Sections {
		section := &report.Sections[i]
		if len(section.Items) == 0 {
			continue
		}

		prompt := buildEnhanceSectionPrompt(section)
		if prompt == "" {
			continue
		}

		text, err := aiClient.StreamGenerate(ctx, prompt, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", section.Heading, err.Error()))
			continue
		}
		if text == "" {
			continue
		}

		descriptions := parseEnhancedDescriptions(text, len(section.Items))
		for j, desc := range descriptions {
			if j < len(section.Items) && desc != "" {
				section.Items[j].Description = desc
			}
		}
		enhanced++
	}

	if len(errs) > 0 && enhanced == 0 {
		return report, fmt.Errorf("AI enhancement failed for all sections: %s", strings.Join(errs, "; "))
	}

	return report, nil
}

func buildEnhanceSectionPrompt(section *render.Section) string {
	var buf strings.Builder
	buf.WriteString("Rewrite each changelog entry below to be more descriptive and user-friendly.\n")
	buf.WriteString("Keep each entry on a single line. Return ONLY the rewritten entries, one per line, in the same order.\n")
	buf.WriteString("Do not add numbering, bullet points, or extra commentary.\n\n")
	fmt.Fprintf(&buf, "Section: %s\n\n", section.Heading)
	for i, item := range section.Items {
		desc := item.Description
		if item.Scope != "" {
			desc = fmt.Sprintf("[%s] %s", item.Scope, desc)
		}
		fmt.Fprintf(&buf, "%d. %s\n", i+1, desc)
	}
	return buf.String()
}

func parseEnhancedDescriptions(text string, count int) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		trimmed := reNumberPrefix.ReplaceAllString(line, "")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			result = append(result, trimmed)
		}
		if len(result) >= count {
			break
		}
	}
	return result
}
