package htmlreport

import (
	"strings"
	"testing"
)

func TestGenerateEscapesHeaderAndSummary(t *testing.T) {
	out := Generate(ReportData{
		Version:   `<script>alert(1)</script>`,
		Date:      `<img src=x onerror=alert(1)>`,
		AISummary: `" onclick="alert(1)`,
	})
	for _, unsafe := range []string{"<script", "<img", `onclick="alert`} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("unsafe report HTML remained: %q", unsafe)
		}
	}
}
