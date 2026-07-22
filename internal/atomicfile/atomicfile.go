// Package atomicfile provides recoverable same-directory file replacement.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write writes data to a same-directory temporary file and then replaces path.
func Write(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".patchlog-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set temporary permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := Replace(tempPath, path); err != nil {
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}

// Replace installs oldpath at newpath. The normal path is an atomic rename.
// The backup fallback supports platforms that cannot rename over an existing
// file and restores the original destination if installation fails.
func Replace(oldpath, newpath string) error {
	initialErr := os.Rename(oldpath, newpath)
	if initialErr == nil {
		return nil
	}
	if _, err := os.Lstat(newpath); err != nil {
		return initialErr
	}

	backup, err := os.CreateTemp(filepath.Dir(newpath), ".patchlog-backup-*")
	if err != nil {
		return initialErr
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return initialErr
	}
	if err := os.Remove(backupPath); err != nil {
		return initialErr
	}
	if err := os.Rename(newpath, backupPath); err != nil {
		return initialErr
	}
	if err := os.Rename(oldpath, newpath); err != nil {
		if restoreErr := os.Rename(backupPath, newpath); restoreErr != nil {
			return fmt.Errorf("install replacement: %w; restore original: %v", err, restoreErr)
		}
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("replacement installed but old-file backup cleanup failed: %w", err)
	}
	return nil
}
