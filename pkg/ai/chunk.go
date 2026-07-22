package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

const chunkThreshold = 4000

func GenerateProseStream(ctx context.Context, report render.Report, tone Tone, aiClient Client, onToken func(string)) (string, error) {
	return GenerateProseStreamLang(ctx, report, tone, aiClient, onToken, i18n.LangEN)
}

func GenerateProseStreamLang(ctx context.Context, report render.Report, tone Tone, aiClient Client, onToken func(string), lang i18n.Lang) (string, error) {
	if aiClient == nil {
		return generateTemplateFallback(report, tone), nil
	}

	prompt := BuildPromptLang(report, tone, lang)
	if prompt == "" {
		return generateTemplateFallback(report, tone), fmt.Errorf("prompt template error")
	}

	if len(prompt) <= chunkThreshold {
		text, err := aiClient.StreamGenerate(ctx, prompt, onToken)
		if err != nil {
			return generateTemplateFallback(report, tone), fmt.Errorf("AI generation failed, using template: %w", err)
		}
		if text == "" {
			return generateTemplateFallback(report, tone), fmt.Errorf("AI returned empty response, using template")
		}
		return text, nil
	}

	return generateChunkedLang(ctx, report, tone, aiClient, onToken, lang)
}

func generateChunkedLang(ctx context.Context, report render.Report, tone Tone, client Client, onToken func(string), lang i18n.Lang) (string, error) {
	breaking := report.Breaking
	sections := make([]templatedSection, 0, len(report.Sections))
	for _, s := range report.Sections {
		sections = append(sections, templatedSection{
			Heading: s.Heading,
			Items:   s.Items,
			Scopes:  s.Scopes,
		})
	}

	chunks := splitSectionsIntoChunks(sections, chunkThreshold)
	if len(chunks) == 0 {
		return generateTemplateFallback(report, tone), nil
	}

	var parts []string
	var chunkErrors []string

	if len(breaking) > 0 {
		chunkPrompt := BuildChunkPromptLang(report, tone, nil, breaking, 1, len(chunks)+1, lang)
		text, err := client.StreamGenerate(ctx, chunkPrompt, onToken)
		if err != nil {
			chunkErrors = append(chunkErrors, err.Error())
		} else if text != "" {
			parts = append(parts, text)
		}
		if onToken != nil {
			onToken("\n\n")
		}
	}

	breakingOffset := 0
	if len(breaking) > 0 {
		breakingOffset = 1
	}

	for i, chunk := range chunks {
		chunkPrompt := BuildChunkPromptLang(report, tone, chunk, nil, i+1+breakingOffset, len(chunks)+breakingOffset, lang)
		text, err := client.StreamGenerate(ctx, chunkPrompt, onToken)
		if err != nil {
			chunkErrors = append(chunkErrors, err.Error())
			continue
		}
		if text == "" {
			continue
		}
		parts = append(parts, text)
		if i < len(chunks)-1 && onToken != nil {
			onToken("\n\n")
		}
	}

	if len(parts) == 0 {
		err := fmt.Errorf("all %d chunks failed", len(chunkErrors))
		if len(chunkErrors) > 0 {
			err = fmt.Errorf("all chunks failed: %s", strings.Join(chunkErrors, "; "))
		}
		return generateTemplateFallback(report, tone), err
	}

	return strings.Join(parts, "\n\n"), nil
}

func splitSectionsIntoChunks(sections []templatedSection, threshold int) [][]templatedSection {
	if len(sections) == 0 {
		return nil
	}

	var chunks [][]templatedSection
	var current []templatedSection
	currentLen := 0

	for _, s := range sections {
		sectionLen := estimateSectionLen(s)

		if currentLen+sectionLen > threshold && len(current) > 0 {
			chunks = append(chunks, current)
			current = nil
			currentLen = 0
		}

		if sectionLen > threshold {
			subChunks := splitLargeSection(s, threshold)
			for _, sc := range subChunks {
				if currentLen+estimateSectionLen(sc) > threshold && len(current) > 0 {
					chunks = append(chunks, current)
					current = nil
					currentLen = 0
				}
				current = append(current, sc)
				currentLen += estimateSectionLen(sc)
			}
		} else {
			current = append(current, s)
			currentLen += sectionLen
		}
	}

	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

func splitLargeSection(s templatedSection, threshold int) []templatedSection {
	var chunks []templatedSection
	var items []render.Item

	currentLen := len(s.Heading) + 10

	for _, item := range s.Items {
		itemLen := len(item.Description) + len(item.Ref) + 20
		if currentLen+itemLen > threshold && len(items) > 0 {
			chunks = append(chunks, templatedSection{
				Heading: s.Heading,
				Items:   items,
			})
			items = nil
			currentLen = len(s.Heading) + 10
		}
		items = append(items, item)
		currentLen += itemLen
	}

	for _, sg := range s.Scopes {
		for _, item := range sg.Items {
			itemLen := len(item.Description) + len(sg.Name) + 20
			if currentLen+itemLen > threshold && len(items) > 0 {
				chunks = append(chunks, templatedSection{
					Heading: s.Heading,
					Items:   items,
				})
				items = nil
				currentLen = len(s.Heading) + 10
			}
			items = append(items, render.Item{
				Description: sg.Name + ": " + item.Description,
				Ref:         item.Ref,
				JiraIssues:  item.JiraIssues,
			})
			currentLen += itemLen
		}
	}

	if len(items) > 0 {
		chunks = append(chunks, templatedSection{
			Heading: s.Heading,
			Items:   items,
		})
	}

	return chunks
}

func estimateSectionLen(s templatedSection) int {
	n := len(s.Heading) + 10
	for _, item := range s.Items {
		n += len(item.Description) + len(item.Ref) + 20
	}
	for _, sg := range s.Scopes {
		n += len(sg.Name) + 5
		for _, item := range sg.Items {
			n += len(item.Description) + len(item.Ref) + 20
		}
	}
	return n
}
