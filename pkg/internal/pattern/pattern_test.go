package pattern

import "testing"

func TestExtractKeys(t *testing.T) {
	text := "PROJ-123 fix bug in ENG-456 and PROJ-123 again"
	keys := ExtractKeys(text)
	if len(keys) != 2 {
		t.Fatalf("expected 2 unique keys, got %d: %v", len(keys), keys)
	}
	if keys[0] != "PROJ-123" {
		t.Errorf("expected PROJ-123 first, got %s", keys[0])
	}
	if keys[1] != "ENG-456" {
		t.Errorf("expected ENG-456 second, got %s", keys[1])
	}
}

func TestExtractKeysNone(t *testing.T) {
	keys := ExtractKeys("no keys here")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestExtractKeysEmpty(t *testing.T) {
	keys := ExtractKeys("")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for empty string, got %d", len(keys))
	}
}

func TestExtractKeysDedup(t *testing.T) {
	text := "PROJ-1 PROJ-1 PROJ-1"
	keys := ExtractKeys(text)
	if len(keys) != 1 {
		t.Errorf("expected 1 unique key, got %d", len(keys))
	}
}

func TestExtractKeysMultiDigit(t *testing.T) {
	keys := ExtractKeys("ABC-12345")
	if len(keys) != 1 || keys[0] != "ABC-12345" {
		t.Errorf("expected ABC-12345, got %v", keys)
	}
}

func TestExtractKeysAlphanumericProject(t *testing.T) {
	keys := ExtractKeys("ABC1-42 DEF-99")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d: %v", len(keys), keys)
	}
}
