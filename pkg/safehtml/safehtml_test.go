package safehtml

import (
	"strings"
	"testing"
)

func TestTextEscapesMarkupAndQuotes(t *testing.T) {
	got := Text(`<img src=x onerror="alert(1)">&'`)
	for _, unsafe := range []string{"<img", "onerror=\"", "&'"} {
		if strings.Contains(got, unsafe) {
			t.Fatalf("unsafe fragment %q remained in %q", unsafe, got)
		}
	}
}
