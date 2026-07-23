package htmlreport

import (
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/metrics"
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

func TestGenerateKeepsDPIAndHealthBehindLabs(t *testing.T) {
	data := ReportData{
		Commits: []commit.Commit{{Author: "Ada", Type: "feat"}},
		Metrics: metrics.ReportMetrics{
			Authors: []metrics.AuthorStat{{Name: "Ada", Commits: 1}},
		},
	}

	withoutLabs := Generate(data)
	for _, experimental := range []string{"Developer Productivity Index", "Team Health Signals"} {
		if strings.Contains(withoutLabs, experimental) {
			t.Fatalf("experimental section %q rendered without Labs", experimental)
		}
	}

	data.Labs = true
	withLabs := Generate(data)
	for _, experimental := range []string{"Developer Productivity Index", "Team Health Signals"} {
		if !strings.Contains(withLabs, experimental) {
			t.Fatalf("experimental section %q missing with Labs enabled", experimental)
		}
	}
}
