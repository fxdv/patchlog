package timeline

import (
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/trends"
)

func TestFormatHTMLEscapesSnapshotText(t *testing.T) {
	out := FormatHTML([]trends.Snapshot{{
		Version: `<script>alert(1)</script>`,
		Date:    `<img src=x onerror=alert(1)>`,
		TopContributors: []trends.ContributorSnap{{
			Name:    `" onclick="alert(1)`,
			Commits: 1,
		}},
	}})
	for _, unsafe := range []string{"<script", "<img", `onclick="alert`} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("unsafe timeline HTML remained: %q", unsafe)
		}
	}
}
