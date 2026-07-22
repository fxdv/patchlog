package truncate

import "testing"

func TestStringShortEnough(t *testing.T) {
	got := String("hello", 10)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestStringExactLength(t *testing.T) {
	got := String("hello", 5)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestStringTruncated(t *testing.T) {
	got := String("hello world", 5)
	if got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
}

func TestStringEmpty(t *testing.T) {
	got := String("", 10)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStringZeroMax(t *testing.T) {
	got := String("hello", 0)
	if got != "..." {
		t.Errorf("expected '...', got %q", got)
	}
}

func TestStringUnicode(t *testing.T) {
	got := String("привет мир", 5)
	if got != "приве..." {
		t.Errorf("expected 'приве...', got %q", got)
	}
}
