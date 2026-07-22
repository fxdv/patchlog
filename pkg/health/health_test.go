package health

import (
	"strings"
	"testing"
)

func TestFormatHTMLEscapesSignals(t *testing.T) {
	out := FormatHTML(Report{
		OverallScore:  90,
		OverallStatus: "Healthy",
		Signals: []Signal{{
			Name:   `<script>alert(1)</script>`,
			Status: `" onclick="alert(1)`,
			Value:  `<b>unsafe</b>`,
			Detail: `<img src=x onerror=alert(1)>`,
		}},
	})
	for _, unsafe := range []string{"<script", "<b>", "<img", `onclick="alert`} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("unsafe health HTML remained: %q", unsafe)
		}
	}
}
