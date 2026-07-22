package ignore

import (
	"testing"
)

func TestCompileEmpty(t *testing.T) {
	re := Compile(nil)
	if re != nil {
		t.Error("expected nil for nil input")
	}
	re = Compile([]string{})
	if re != nil {
		t.Error("expected nil for empty input")
	}
}

func TestCompileValid(t *testing.T) {
	re := Compile([]string{"^Merge", "^chore\\(deps\\)"})
	if re == nil {
		t.Fatal("expected non-nil regex")
	}
	if !re.MatchString("Merge branch foo") {
		t.Error("should match ^Merge")
	}
	if !re.MatchString("chore(deps): bump foo") {
		t.Error("should match ^chore(deps)")
	}
	if re.MatchString("feat: add thing") {
		t.Error("should not match feat commit")
	}
}

func TestCompileInvalidSkipped(t *testing.T) {
	re := Compile([]string{"[invalid", "^Merge"})
	if re == nil {
		t.Fatal("expected non-nil regex (invalid pattern skipped)")
	}
	if !re.MatchString("Merge branch") {
		t.Error("should match valid pattern")
	}
}

func TestCompileAllInvalid(t *testing.T) {
	re := Compile([]string{"[invalid", "(unclosed"})
	if re != nil {
		t.Error("expected nil when all patterns invalid")
	}
}

func TestCompileSinglePattern(t *testing.T) {
	re := Compile([]string{"^WIP"})
	if re == nil {
		t.Fatal("expected non-nil regex")
	}
	if !re.MatchString("WIP: work in progress") {
		t.Error("should match WIP")
	}
}
