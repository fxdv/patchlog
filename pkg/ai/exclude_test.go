package ai

import "testing"

func TestPathExcluded(t *testing.T) {
	patterns := []string{".env*", "*.pem", "**/generated/**", "vendor/**"}
	for _, file := range []string{".env.production", "keys/server.pem", "pkg/generated/client.go", "generated/client.go", "vendor/lib/code.go"} {
		if !PathExcluded(file, patterns) {
			t.Errorf("expected %q to be excluded", file)
		}
	}
	if PathExcluded("pkg/service.go", patterns) {
		t.Fatal("ordinary source file should not be excluded")
	}
}
