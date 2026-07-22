package render

import (
	"bytes"
	"fmt"
	"strings"
)

var sectionEmojis = map[string]string{
	"feat":     "✨",
	"fix":      "🐛",
	"perf":     "⚡",
	"refactor": "🔨",
	"docs":     "📝",
	"test":     "🧪",
	"style":    "🎨",
	"ci":       "👷",
	"chore":    "🔧",
}

func emojiForType(typeKey string) string {
	if e, ok := sectionEmojis[typeKey]; ok {
		return e
	}
	return ""
}

func PrettyHeading(heading, typeKey string, useEmoji bool) string {
	if !useEmoji {
		return heading
	}
	if e := emojiForType(typeKey); e != "" {
		return e + " " + heading
	}
	return heading
}

func breakingHeading(useEmoji bool) string {
	if useEmoji {
		return "⚠️ Breaking Changes"
	}
	return "Breaking Changes"
}

func formatVersionHeading(version, date string) string {
	if date != "" {
		return fmt.Sprintf("## [%s] — %s", version, date)
	}
	return fmt.Sprintf("## [%s]", version)
}

func separator() string {
	return "\n---\n\n"
}

func changelogHeader(title string) string {
	return fmt.Sprintf("# %s\n\nAll notable changes to this project will be documented in this file.\n\n", title)
}

func MarkdownSection(report Report, useEmoji bool) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(formatVersionHeading(report.Version, report.DateStr()))
	buf.WriteString("\n\n")

	if report.CompareURL != "" {
		fmt.Fprintf(&buf, "[Full Changelog](%s)\n\n", report.CompareURL)
	}

	if len(report.Breaking) > 0 {
		fmt.Fprintf(&buf, "### %s\n\n", breakingHeading(useEmoji))
		for _, item := range report.Breaking {
			writeItem(&buf, item, report.ShowAuthor)
		}
		buf.WriteByte('\n')
	}

	for _, section := range report.Sections {
		if len(section.Items) == 0 && len(section.Scopes) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "### %s\n\n", PrettyHeading(section.Heading, section.Type, useEmoji))
		for _, item := range section.Items {
			writeItem(&buf, item, report.ShowAuthor)
		}
		if len(section.Items) > 0 && len(section.Scopes) > 0 {
			buf.WriteByte('\n')
		}
		for _, sg := range section.Scopes {
			fmt.Fprintf(&buf, "#### %s\n\n", sg.Name)
			for _, item := range sg.Items {
				writeScopedItem(&buf, item, report.ShowAuthor)
			}
		}
		buf.WriteByte('\n')
	}

	if len(report.Dependencies) > 0 {
		fmt.Fprintf(&buf, "### %s\n\n", dependenciesHeading(useEmoji))
		for _, dep := range report.Dependencies {
			writeDependency(&buf, dep)
		}
	}

	return buf.Bytes(), nil
}

func (r Report) DateStr() string {
	if r.Date != "" {
		return r.Date
	}
	return ""
}

func AccumulateMarkdown(existing []byte, newSection []byte, title string) []byte {
	if len(existing) == 0 {
		var buf bytes.Buffer
		buf.WriteString(changelogHeader(title))
		buf.Write(newSection)
		return buf.Bytes()
	}

	existingStr := string(existing)
	headerEnd := 0

	headerPrefix := "# " + title
	if strings.HasPrefix(existingStr, headerPrefix) {
		descMarker := "All notable changes"
		if idx := strings.Index(existingStr, descMarker); idx >= 0 {
			lineEnd := strings.Index(existingStr[idx:], "\n")
			if lineEnd >= 0 {
				headerEnd = idx + lineEnd + 1
				for headerEnd < len(existingStr) && existingStr[headerEnd] == '\n' {
					headerEnd++
				}
			}
		}
		if headerEnd == 0 {
			if idx := strings.Index(existingStr, "\n"); idx >= 0 {
				headerEnd = idx + 1
				for headerEnd < len(existingStr) && existingStr[headerEnd] == '\n' {
					headerEnd++
				}
			}
		}
	}

	if headerEnd == 0 && strings.HasPrefix(existingStr, "# ") {
		if idx := strings.Index(existingStr, "\n"); idx >= 0 {
			headerEnd = idx + 1
			for headerEnd < len(existingStr) && existingStr[headerEnd] == '\n' {
				headerEnd++
			}
		}
	}

	var buf bytes.Buffer
	if headerEnd > 0 {
		buf.WriteString(existingStr[:headerEnd])
		buf.WriteString(separator())
	} else {
		buf.WriteString(changelogHeader(title))
		buf.WriteString(separator())
	}

	buf.Write(newSection)

	if headerEnd > 0 && headerEnd < len(existingStr) {
		rest := existingStr[headerEnd:]
		rest = strings.TrimLeft(rest, "\n")
		if strings.HasPrefix(rest, "---") {
			rest = strings.TrimLeft(rest[3:], "\n")
		}
		if rest != "" {
			buf.WriteString(separator())
			buf.WriteString(rest)
		}
	}

	return buf.Bytes()
}
