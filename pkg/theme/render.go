// Package theme uses AI to group commits into thematic sections instead of conventional commit types.
package theme

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/render"
)

func Markdown(tr ThemedReport) ([]byte, error) {
	var buf bytes.Buffer

	if tr.Version != "" {
		fmt.Fprintf(&buf, "# %s", tr.Version)
	}
	if tr.Date != "" {
		fmt.Fprintf(&buf, " (%s)", tr.Date)
	} else {
		fmt.Fprintf(&buf, " (%s)", time.Now().Format("2006-01-02"))
	}
	buf.WriteString("\n\n")

	if tr.CompareURL != "" {
		fmt.Fprintf(&buf, "[Full Changelog](%s)\n\n", tr.CompareURL)
	}

	if len(tr.Breaking) > 0 {
		fmt.Fprintf(&buf, "## %s\n\n", breakingHeading(tr.Emojis))
		for _, item := range tr.Breaking {
			writeItem(&buf, item, tr.ShowAuthor)
		}
		buf.WriteByte('\n')
	}

	for _, theme := range tr.Themes {
		if len(theme.Items) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "## %s\n\n", theme.Title)
		if theme.Narrative != "" {
			fmt.Fprintf(&buf, "*%s*\n\n", theme.Narrative)
		}
		for _, item := range theme.Items {
			writeItem(&buf, item, tr.ShowAuthor)
		}
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func writeItem(buf *bytes.Buffer, item render.Item, showAuthor bool) {
	buf.WriteString("- ")
	if item.Scope != "" {
		fmt.Fprintf(buf, "**%s**: ", item.Scope)
	}
	buf.WriteString(item.Description)
	if item.Ref != "" {
		fmt.Fprintf(buf, " (%s)", item.Ref)
	}
	for _, j := range item.JiraIssues {
		if j.URL != "" {
			fmt.Fprintf(buf, " [%s](%s)", j.Key, j.URL)
		} else {
			fmt.Fprintf(buf, " [%s]", j.Key)
		}
		if j.Summary != "" {
			fmt.Fprintf(buf, " %s", j.Summary)
		}
		if j.Status != "" {
			fmt.Fprintf(buf, " `%s`", j.Status)
		}
		if len(j.Components) > 0 {
			fmt.Fprintf(buf, " (%s)", strings.Join(j.Components, ", "))
		}
		if len(j.FixVersions) > 0 {
			fmt.Fprintf(buf, " → %s", strings.Join(j.FixVersions, ", "))
		}
	}
	if item.Author != "" && showAuthor {
		fmt.Fprintf(buf, " by @%s", item.Author)
	}
	buf.WriteByte('\n')
}

func breakingHeading(useEmoji bool) string {
	if useEmoji {
		return "⚠️ Breaking Changes"
	}
	return "Breaking Changes"
}
