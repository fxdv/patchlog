package curate

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/console"
	"github.com/fxdv/patchlog/pkg/render"
)

const (
	iconIncluded = "◉"
	iconExcluded = "◯"
	iconMergeSrc = "◉⟶"
)

func (s *CuratorState) Render() string {
	var buf strings.Builder

	if s.ShowPreview {
		left := s.renderBrowsePane(s.Width/2, s.Height)
		right := s.renderPreviewPane(s.Width/2, s.Height)
		buf.WriteString(left)
		buf.WriteString("│")
		buf.WriteString(right)
		return buf.String()
	}

	if s.Mode == ModeHelp {
		return s.renderHelp()
	}

	buf.WriteString(s.renderBrowsePane(s.Width, s.Height))
	return buf.String()
}

func (s *CuratorState) renderBrowsePane(width, height int) string {
	var buf strings.Builder

	frameWidth := width - 2
	if frameWidth < 20 {
		frameWidth = 20
	}
	title := fmt.Sprintf(" patchlog curate · %s · %d items · %d shown ",
		s.Report.Version, s.TotalItems(), s.VisibleItemCount())
	if len(title) > frameWidth {
		title = title[:frameWidth]
	}
	pad := frameWidth - len(title)
	if pad < 0 {
		pad = 0
	}

	buf.WriteString("┌─")
	buf.WriteString(title)
	buf.WriteString(strings.Repeat("─", pad))
	buf.WriteString("─┐\n")

	contentLines := s.renderContent(frameWidth)
	maxContentLines := height - 4
	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
	}

	for _, line := range contentLines {
		displayWidth := displayWidth(line)
		if displayWidth < frameWidth {
			line = line + strings.Repeat(" ", frameWidth-displayWidth)
		}
		buf.WriteString("│ ")
		buf.WriteString(line)
		buf.WriteString(" │\n")
	}

	for i := len(contentLines); i < maxContentLines; i++ {
		buf.WriteString("│ ")
		buf.WriteString(strings.Repeat(" ", frameWidth))
		buf.WriteString(" │\n")
	}

	buf.WriteString("├")
	buf.WriteString(strings.Repeat("─", frameWidth+2))
	buf.WriteString("┤\n")

	statusBar := s.renderStatusBar(frameWidth)
	buf.WriteString("│ ")
	buf.WriteString(statusBar)
	sbPad := frameWidth - displayWidth(statusBar)
	if sbPad < 0 {
		sbPad = 0
	}
	buf.WriteString(strings.Repeat(" ", sbPad))
	buf.WriteString(" │\n")

	buf.WriteString("└")
	buf.WriteString(strings.Repeat("─", frameWidth+2))
	buf.WriteString("┘")

	return buf.String()
}

func (s *CuratorState) renderContent(width int) []string {
	var lines []string

	if s.Mode == ModeEdit {
		lines = append(lines, s.renderEditLine(width))
		return lines
	}

	if s.Mode == ModeMove {
		lines = append(lines, s.renderMovePicker(width))
		return lines
	}

	if s.Mode == ModeMerge && s.MergeSource != nil {
		lines = append(lines, s.renderMergeDialog(width))
		return lines
	}

	for i, section := range s.Report.Sections {
		if len(section.Items) == 0 {
			continue
		}
		heading := fmt.Sprintf("  %s  [%2d]", section.Heading, len(section.Items))
		lines = append(lines, heading)

		for j, item := range section.Items {
			isCurrent := (i == s.Cursor.Section && j == s.Cursor.Item)
			isMergeSrc := s.MergeSource != nil && s.MergeSource.Section == i && s.MergeSource.Item == j
			excluded := s.IsExcluded(i, j)

			icon := iconIncluded
			if excluded {
				icon = iconExcluded
			}
			if isMergeSrc {
				icon = iconMergeSrc
			}

			desc := item.Description
			maxDescWidth := width - len(icon) - 5
			if maxDescWidth < 10 {
				maxDescWidth = 10
			}
			if len(desc) > maxDescWidth {
				desc = desc[:maxDescWidth-1] + "…"
			}

			author := ""
			if item.Author != "" {
				author = " @" + item.Author
			}

			line := fmt.Sprintf("    %s %s%s", icon, desc, author)
			if len(line) > width {
				line = line[:width-1] + "…"
			}

			if isCurrent {
				line = console.CyanText(line)
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	return lines
}

func (s *CuratorState) renderEditLine(width int) string {
	prompt := "edit: " + s.EditBuffer + "_"
	if len(prompt) > width {
		prompt = prompt[:width-1] + "…"
	}
	return console.CyanText(prompt)
}

func (s *CuratorState) renderMovePicker(width int) string {
	var sections []string
	for i, section := range s.Report.Sections {
		marker := " "
		if i == s.Cursor.Section {
			marker = "▶"
		}
		sections = append(sections, fmt.Sprintf("%s %s", marker, section.Heading))
	}
	line := "move to: " + strings.Join(sections, "  ") + "  ↵ esc"
	if len(line) > width {
		line = line[:width-1] + "…"
	}
	return console.CyanText(line)
}

func (s *CuratorState) renderMergeDialog(width int) string {
	line := "merge: select target item, press x to confirm, esc to cancel"
	if len(line) > width {
		line = line[:width-1] + "…"
	}
	return console.CyanText(line)
}

func (s *CuratorState) renderStatusBar(width int) string {
	if s.Mode == ModeEdit {
		return "↵ save   esc cancel"
	}
	if s.Mode == ModeMove {
		return "←→ select section   ↵ confirm   esc cancel"
	}
	if s.Mode == ModeMerge {
		return "↑↓ select target   x confirm   esc cancel"
	}

	keys := "↑↓ navigate   space toggle   e edit   m move   x merge   p preview   u undo   ? help   q quit"
	if s.ShowSearch {
		keys = "search: " + s.SearchQuery + "_   ↵ done   esc cancel"
	}
	if len(keys) > width {
		keys = keys[:width-1] + "…"
	}
	return console.DimText(keys)
}

func (s *CuratorState) renderPreviewPane(width, height int) string {
	var buf strings.Builder

	filtered := s.FilteredReport()
	md, _ := render.Markdown(filtered)
	mdStr := string(md)

	lines := strings.Split(mdStr, "\n")
	maxLines := height - 2
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	for _, line := range lines {
		if len(line) > width {
			line = line[:width-1] + "…"
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return buf.String()
}

func (s *CuratorState) renderHelp() string {
	var buf strings.Builder

	helpText := []struct {
		key  string
		desc string
	}{
		{"j / ↓", "Move cursor down"},
		{"k / ↑", "Move cursor up"},
		{"J / K", "Move to next/previous section"},
		{"g / G", "Jump to top / bottom"},
		{"space", "Toggle include/exclude"},
		{"e", "Edit description (inline)"},
		{"m", "Move item to another section"},
		{"x", "Mark for merge (select 2 items)"},
		{"u", "Undo last action"},
		{"/", "Search/filter items"},
		{"p", "Toggle preview pane"},
		{"?", "Toggle this help"},
		{"q", "Quit without publishing"},
		{"↵", "Confirm and publish"},
		{"Ctrl+S", "Save to file without publishing"},
	}

	width := s.Width - 2
	buf.WriteString("┌─")
	buf.WriteString(" Help ")
	buf.WriteString(strings.Repeat("─", width-7))
	buf.WriteString("─┐\n")

	for _, h := range helpText {
		line := fmt.Sprintf("  %-12s %s", h.key, h.desc)
		if len(line) > width {
			line = line[:width-1] + "…"
		}
		buf.WriteString("│ ")
		buf.WriteString(line)
		pad := width - len(line)
		if pad > 0 {
			buf.WriteString(strings.Repeat(" ", pad))
		}
		buf.WriteString(" │\n")
	}

	buf.WriteString("└")
	buf.WriteString(strings.Repeat("─", width+2))
	buf.WriteString("┘")

	return buf.String()
}

func displayWidth(s string) int {
	n := 0
	inEscape := false
	for _, c := range s {
		if c == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if c == 'm' {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}
