package significance

import "testing"

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{Skip, "skip"},
		{Patch, "patch"},
		{Minor, "minor"},
		{Major, "major"},
	}
	for _, tc := range tests {
		got := tc.level.String()
		if got != tc.want {
			t.Errorf("Level(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestLevelStringDefault(t *testing.T) {
	l := Level(999)
	if l.String() != "patch" {
		t.Errorf("unknown level should default to patch, got %q", l.String())
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"skip", Skip},
		{"patch", Patch},
		{"minor", Minor},
		{"major", Major},
	}
	for _, tc := range tests {
		got, err := ParseLevel(tc.input)
		if err != nil {
			t.Fatalf("ParseLevel(%q) error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseLevelInvalid(t *testing.T) {
	_, err := ParseLevel("bogus")
	if err == nil {
		t.Error("expected error for invalid level")
	}
}
