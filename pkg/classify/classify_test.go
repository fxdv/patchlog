package classify

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
)

func TestClassifyBreaking(t *testing.T) {
	c := commit.Commit{Type: "feat", Breaking: true}
	r := Classify(c, 1)
	if r.Level != Major {
		t.Errorf("expected Major, got %s", r.Level)
	}
	if r.Reason != "breaking change" {
		t.Errorf("unexpected reason: %s", r.Reason)
	}
}

func TestClassifyFeature(t *testing.T) {
	c := commit.Commit{Type: "feat", Breaking: false}
	r := Classify(c, 3)
	if r.Level != Minor {
		t.Errorf("expected Minor, got %s", r.Level)
	}
}

func TestClassifyLargeFeature(t *testing.T) {
	c := commit.Commit{Type: "feat", Breaking: false}
	r := Classify(c, 5)
	if r.Level != Major {
		t.Errorf("expected Major for large feature, got %s", r.Level)
	}
}

func TestClassifyFix(t *testing.T) {
	c := commit.Commit{Type: "fix", Breaking: false}
	r := Classify(c, 2)
	if r.Level != Patch {
		t.Errorf("expected Patch, got %s", r.Level)
	}
}

func TestClassifyLargeFix(t *testing.T) {
	c := commit.Commit{Type: "fix", Breaking: false}
	r := Classify(c, 5)
	if r.Level != Minor {
		t.Errorf("expected Minor for large fix, got %s", r.Level)
	}
}

func TestClassifyPerf(t *testing.T) {
	c := commit.Commit{Type: "perf"}
	r := Classify(c, 0)
	if r.Level != Minor {
		t.Errorf("expected Minor for perf, got %s", r.Level)
	}
}

func TestClassifyRefactor(t *testing.T) {
	c := commit.Commit{Type: "refactor"}
	r := Classify(c, 0)
	if r.Level != Patch {
		t.Errorf("expected Patch for refactor, got %s", r.Level)
	}
}

func TestClassifySkips(t *testing.T) {
	skipTypes := []string{"docs", "test", "style", "ci", "chore"}
	for _, typ := range skipTypes {
		c := commit.Commit{Type: typ}
		r := Classify(c, 0)
		if r.Level != Skip {
			t.Errorf("expected Skip for %s, got %s", typ, r.Level)
		}
	}
}

func TestClassifyUnknownSmall(t *testing.T) {
	c := commit.Commit{Type: "custom"}
	r := Classify(c, 2)
	if r.Level != Patch {
		t.Errorf("expected Patch for small unknown, got %s", r.Level)
	}
}

func TestClassifyUnknownLarge(t *testing.T) {
	c := commit.Commit{Type: "custom"}
	r := Classify(c, 3)
	if r.Level != Minor {
		t.Errorf("expected Minor for large unknown, got %s", r.Level)
	}
}

func TestLevelString(t *testing.T) {
	tests := map[Level]string{
		Skip:  "skip",
		Patch: "patch",
		Minor: "minor",
		Major: "major",
	}
	for lvl, want := range tests {
		if lvl.String() != want {
			t.Errorf("Level(%d).String() = %q, want %q", lvl, lvl.String(), want)
		}
	}
}

func TestClassifyChangedFilesRecorded(t *testing.T) {
	c := commit.Commit{Type: "fix"}
	r := Classify(c, 42)
	if r.Changed != 42 {
		t.Errorf("expected Changed=42, got %d", r.Changed)
	}
}
