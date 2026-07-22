package deps

import (
	"encoding/json"
	"fmt"
	"strings"
)

func FormatMarkdown(changes []Change) string {
	if len(changes) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("## Dependencies\n\n")
	for _, c := range changes {
		formatChangeMarkdown(&buf, c)
	}
	return buf.String()
}

func formatChangeMarkdown(buf *strings.Builder, c Change) {
	arrow := c.NewVersion
	if c.OldVersion != "" {
		arrow = fmt.Sprintf("%s → %s", c.OldVersion, c.NewVersion)
	}

	fmt.Fprintf(buf, "### %s %s\n\n", c.Name, arrow)
	fmt.Fprintf(buf, "_ecosystem: %s_ _manifest: %s_\n\n", c.Ecosystem, c.Manifest)

	if c.Changelog != "" {
		buf.WriteString("<details>\n<summary>Upstream changes</summary>\n\n")
		buf.WriteString(c.Changelog)
		buf.WriteString("\n\n</details>\n\n")
	} else if c.ChangelogURL != "" {
		fmt.Fprintf(buf, "[Upstream](%s)\n\n", c.ChangelogURL)
	}
}

func FormatJSON(changes []Change) ([]byte, error) {
	return json.MarshalIndent(changes, "", "  ")
}
