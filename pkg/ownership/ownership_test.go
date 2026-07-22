package ownership

import (
	"strings"
	"testing"
)

func TestFormatHTMLEscapesDirectoriesAndAuthors(t *testing.T) {
	dirs := []DirectoryOwnership{{
		Dir:     `<img src=x onerror=alert(1)>`,
		Files:   1,
		Authors: map[string]int{`<script>alert(1)</script>`: 1},
	}}
	out := FormatHTML(nil, dirs)
	for _, unsafe := range []string{"<img", "<script"} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("unsafe ownership HTML remained: %q", unsafe)
		}
	}
}
