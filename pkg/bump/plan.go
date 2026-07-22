package bump

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fxdv/patchlog/internal/atomicfile"
)

// FileChange is an exact file mutation produced by a bump plan. Path is
// relative to RepoPath so it can be passed directly to Git without consulting
// the worktree status.
type FileChange struct {
	Path   string
	Before []byte
	After  []byte
	Mode   os.FileMode

	absolutePath string
}

// Plan is a side-effect-free description of a version bump. Creating a Plan
// only reads files; Apply is the mutation boundary.
type Plan struct {
	RepoPath       string
	CurrentVersion string
	NewVersion     string
	Changes        []FileChange
}

// CreatePlan inspects all supported manifests and returns the exact mutations
// required for a bump. When autoDetect is false, only extraFiles are planned.
func CreatePlan(repoPath string, level Level, extraFiles []string, autoDetect bool) (*Plan, error) {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repository path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect repository path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository path %q is not a directory", root)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize repository path %q: %w", repoPath, err)
	}

	type candidate struct {
		path string
		kind string
	}
	var candidates []candidate
	if autoDetect {
		for _, detector := range detectors {
			for _, name := range detector.Files {
				unresolvedPath := filepath.Join(root, name)
				fileInfo, statErr := os.Lstat(unresolvedPath)
				if os.IsNotExist(statErr) {
					continue
				}
				if statErr != nil {
					return nil, fmt.Errorf("inspect version file %q: %w", name, statErr)
				}
				if fileInfo.IsDir() {
					continue
				}
				resolvedPath, resolveErr := resolveTarget(root, name)
				if resolveErr != nil {
					return nil, resolveErr
				}
				candidates = append(candidates, candidate{path: resolvedPath, kind: detector.Name})
				break
			}
		}
	}
	for _, name := range extraFiles {
		path, resolveErr := resolveTarget(root, name)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if fileInfo, statErr := os.Stat(path); statErr != nil {
			return nil, fmt.Errorf("inspect version file %q: %w", name, statErr)
		} else if fileInfo.IsDir() {
			return nil, fmt.Errorf("version file %q is a directory", name)
		}
		candidates = append(candidates, candidate{path: path, kind: "version-file"})
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := candidates[:0]
	for _, item := range candidates {
		if _, ok := seen[item.path]; ok {
			continue
		}
		seen[item.path] = struct{}{}
		unique = append(unique, item)
	}
	candidates = unique
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no version file found in %s (looked for package.json, Cargo.toml, pyproject.toml, VERSION)", root)
	}

	contents := make([][]byte, len(candidates))
	versions := make([]string, len(candidates))
	var currentVersion string
	for i, item := range candidates {
		data, readErr := os.ReadFile(item.path)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", item.path, readErr)
		}
		version, detectErr := detectVersion(item.kind, data)
		if detectErr != nil {
			return nil, fmt.Errorf("detect version in %s: %w", item.path, detectErr)
		}
		if currentVersion == "" {
			currentVersion = version
		} else if version != currentVersion {
			return nil, fmt.Errorf("version mismatch: %s contains %s, expected %s", item.path, version, currentVersion)
		}
		contents[i] = data
		versions[i] = version
	}

	newVersion, err := bumpVersion(currentVersion, level)
	if err != nil {
		return nil, err
	}
	plan := &Plan{RepoPath: root, CurrentVersion: currentVersion, NewVersion: newVersion}
	for i, item := range candidates {
		after, renderErr := renderVersion(item.kind, contents[i], versions[i], newVersion)
		if renderErr != nil {
			return nil, fmt.Errorf("plan bump for %s: %w", item.path, renderErr)
		}
		if bytes.Equal(contents[i], after) {
			continue
		}
		fileInfo, statErr := os.Stat(item.path)
		if statErr != nil {
			return nil, fmt.Errorf("inspect %s: %w", item.path, statErr)
		}
		relativePath, relErr := filepath.Rel(root, item.path)
		if relErr != nil {
			return nil, fmt.Errorf("resolve relative path for %s: %w", item.path, relErr)
		}
		plan.Changes = append(plan.Changes, FileChange{
			Path:         filepath.ToSlash(relativePath),
			Before:       append([]byte(nil), contents[i]...),
			After:        append([]byte(nil), after...),
			Mode:         fileInfo.Mode().Perm(),
			absolutePath: item.path,
		})
	}
	return plan, nil
}

func resolveTarget(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("version file must be relative to the repository: %q", name)
	}
	path := filepath.Clean(filepath.Join(root, name))
	relativePath, err := filepath.Rel(root, path)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("version file escapes repository: %q", name)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve version file %q: %w", name, err)
	}
	resolvedRelative, err := filepath.Rel(root, resolved)
	if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("version file resolves outside repository: %q", name)
	}
	return resolved, nil
}

func detectVersion(kind string, data []byte) (string, error) {
	var matches [][]byte
	switch kind {
	case "npm":
		matches = reNPMVersion.FindSubmatch(data)
	case "cargo", "python":
		for _, line := range bytes.Split(data, []byte("\n")) {
			if match := reTOMLVersion.FindSubmatch(line); match != nil {
				matches = match
				break
			}
		}
	case "version-file":
		version := strings.TrimSpace(string(data))
		if version == "" {
			return "", errors.New("version is empty")
		}
		return version, nil
	default:
		return "", fmt.Errorf("unsupported version file kind %q", kind)
	}
	if len(matches) < 2 {
		return "", errors.New("version field not found")
	}
	return string(matches[1]), nil
}

func renderVersion(kind string, data []byte, oldVersion, newVersion string) ([]byte, error) {
	switch kind {
	case "npm":
		re := regexp.MustCompile(`("version"\s*:\s*")` + regexp.QuoteMeta(oldVersion) + `(")`)
		if !re.Match(data) {
			return nil, errors.New("planned npm version is no longer present")
		}
		return re.ReplaceAll(data, []byte(`${1}`+newVersion+`${2}`)), nil
	case "cargo", "python":
		re := regexp.MustCompile(`(?m)^(version\s*=\s*")` + regexp.QuoteMeta(oldVersion) + `(")`)
		if !re.Match(data) {
			return nil, errors.New("planned TOML version is no longer present")
		}
		return re.ReplaceAll(data, []byte(`${1}`+newVersion+`${2}`)), nil
	case "version-file":
		return []byte(newVersion + "\n"), nil
	default:
		return nil, fmt.Errorf("unsupported version file kind %q", kind)
	}
}

// ChangedFiles returns the exact repository-relative files Apply will mutate.
func (p *Plan) ChangedFiles() []string {
	files := make([]string, len(p.Changes))
	for i, change := range p.Changes {
		files[i] = change.Path
	}
	return files
}

// Apply transactionally writes all planned changes. It verifies every source
// file before the first write and restores committed files if a later rename
// fails.
func (p *Plan) Apply() error {
	return p.applyWithRename(atomicfile.Replace)
}

func (p *Plan) applyWithRename(rename func(string, string) error) error {
	if p == nil {
		return errors.New("bump plan is nil")
	}
	for _, change := range p.Changes {
		current, err := os.ReadFile(change.absolutePath)
		if err != nil {
			return fmt.Errorf("preflight %s: %w", change.Path, err)
		}
		if !bytes.Equal(current, change.Before) {
			return fmt.Errorf("preflight %s: file changed after planning", change.Path)
		}
	}

	temps := make([]string, len(p.Changes))
	cleanup := func() {
		for _, path := range temps {
			if path != "" {
				_ = os.Remove(path)
			}
		}
	}
	defer cleanup()
	for i, change := range p.Changes {
		temp, err := os.CreateTemp(filepath.Dir(change.absolutePath), ".patchlog-bump-*")
		if err != nil {
			return fmt.Errorf("prepare %s: %w", change.Path, err)
		}
		temps[i] = temp.Name()
		if err := temp.Chmod(change.Mode); err != nil {
			_ = temp.Close()
			return fmt.Errorf("prepare permissions for %s: %w", change.Path, err)
		}
		if _, err := temp.Write(change.After); err != nil {
			_ = temp.Close()
			return fmt.Errorf("prepare contents for %s: %w", change.Path, err)
		}
		if err := temp.Sync(); err != nil {
			_ = temp.Close()
			return fmt.Errorf("sync temporary %s: %w", change.Path, err)
		}
		if err := temp.Close(); err != nil {
			return fmt.Errorf("close temporary %s: %w", change.Path, err)
		}
	}

	committed := 0
	for i, change := range p.Changes {
		if err := rename(temps[i], change.absolutePath); err != nil {
			rollbackErr := p.rollback(committed)
			if rollbackErr != nil {
				return fmt.Errorf("apply %s: %w; rollback failed: %v", change.Path, err, rollbackErr)
			}
			return fmt.Errorf("apply %s: %w (all earlier changes rolled back)", change.Path, err)
		}
		temps[i] = ""
		committed++
	}
	return nil
}

func (p *Plan) rollback(count int) error {
	var rollbackErrors []error
	for i := count - 1; i >= 0; i-- {
		change := p.Changes[i]
		if err := atomicfile.Write(change.absolutePath, change.Before, change.Mode); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("%s: %w", change.Path, err))
		}
	}
	return errors.Join(rollbackErrors...)
}
