package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesContentsAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "output.txt")
	if err := Write(path, []byte("first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("second\n"), 0640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second\n" {
		t.Fatalf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}
