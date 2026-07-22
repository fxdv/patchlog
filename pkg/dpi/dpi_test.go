package dpi

import (
	"strings"
	"testing"
)

func TestFormatHTMLEscapesContributorData(t *testing.T) {
	out := FormatHTML([]DeveloperDPI{{
		Name:       `<img src=x onerror="alert(1)">`,
		Grade:      `A`,
		Strengths:  []string{`<script>alert(1)</script>`},
		Weaknesses: []string{`" onclick="alert(1)`},
	}})
	for _, unsafe := range []string{"<img", "<script", `onclick="alert`} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("unsafe contributor HTML remained: %q", unsafe)
		}
	}
}
