package main

import (
	"fmt"
	"os"
	"strings"
)

func printBanner() {
	w := 50

	displayWidth := func(s string) int {
		n := 0
		for _, r := range s {
			n++
			if r >= 0x1100 && (r <= 0x115F || r == 0x2329 || r == 0x232A ||
				(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
				(r >= 0xAC00 && r <= 0xD7A3) ||
				(r >= 0xF900 && r <= 0xFAFF) ||
				(r >= 0xFE10 && r <= 0xFE19) ||
				(r >= 0xFE30 && r <= 0xFE6F) ||
				(r >= 0xFF01 && r <= 0xFF60) ||
				(r >= 0xFFE0 && r <= 0xFFE6) ||
				(r >= 0x1F300 && r <= 0x1F64F) ||
				(r >= 0x1F900 && r <= 0x1F9FF)) {
				n++
			}
		}
		return n
	}

	stripV := func(s string) string {
		if len(s) > 1 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9' {
			return s[1:]
		}
		return s
	}

	center := func(s string) string {
		dw := displayWidth(s)
		if dw >= w {
			return s
		}
		total := w - dw
		left := total / 2
		right := total - left
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
	}

	b := &strings.Builder{}
	frame := strings.Repeat("─", w)
	b.WriteString("┌" + frame + "┐\n")
	b.WriteString("│" + center("⚡ patchlog v"+stripV(currentVersion())) + "│\n")
	b.WriteString("│" + center("") + "│\n")
	b.WriteString("│" + center("auto-generate release notes") + "│\n")
	b.WriteString("│" + center("from git history") + "│\n")
	b.WriteString("└" + frame + "┘\n")

	fmt.Fprint(os.Stderr, b.String())
}
