// Package bump handles semantic version bumping in package.json, Cargo.toml, pyproject.toml, and plain text version files.
package bump

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Level int

const (
	Patch Level = iota + 1
	Minor
	Major
)

func (l Level) String() string {
	switch l {
	case Patch:
		return "patch"
	case Minor:
		return "minor"
	case Major:
		return "major"
	default:
		return "patch"
	}
}

func ParseLevel(s string) (Level, error) {
	switch s {
	case "patch":
		return Patch, nil
	case "minor":
		return Minor, nil
	case "major":
		return Major, nil
	default:
		return Patch, fmt.Errorf("unknown bump level %q, use patch/minor/major", s)
	}
}

type Detector struct {
	Files []string
	Name  string
}

var versionFileDetector = Detector{
	Files: []string{"VERSION", "version.txt", "version"},
	Name:  "version-file",
}

var detectors = []Detector{
	{
		Files: []string{"package.json"},
		Name:  "npm",
	},
	{
		Files: []string{"Cargo.toml"},
		Name:  "cargo",
	},
	{
		Files: []string{"pyproject.toml"},
		Name:  "python",
	},
	versionFileDetector,
}

var (
	reNPMVersion  = regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)
	reTOMLVersion = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)
)

func bumpVersion(v string, level Level) (string, error) {
	core := v
	pre := ""
	if idx := strings.Index(v, "-"); idx >= 0 {
		core = v[:idx]
		pre = v[idx:]
	}

	parts := strings.Split(core, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("cannot bump version %q: need at least X.Y", v)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("cannot bump version %q: invalid major part %q", v, parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("cannot bump version %q: invalid minor part %q", v, parts[1])
	}
	patch := 0
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return "", fmt.Errorf("cannot bump version %q: invalid patch part %q", v, parts[2])
		}
	}

	switch level {
	case Major:
		major++
		minor = 0
		patch = 0
		pre = ""
	case Minor:
		minor++
		patch = 0
		pre = ""
	case Patch:
		patch++
		pre = ""
	default:
		return "", fmt.Errorf("invalid bump level %d", level)
	}

	result := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if pre != "" {
		result += pre
	}
	return result, nil
}

// Result reports the exact files changed by a completed bump operation.
type Result struct {
	CurrentVersion string
	NewVersion     string
	ChangedFiles   []string
}

func Run(repoPath string, level Level, extraFiles []string) (Result, error) {
	plan, err := CreatePlan(repoPath, level, extraFiles, true)
	if err != nil {
		return Result{}, err
	}
	if err := plan.Apply(); err != nil {
		return Result{}, err
	}
	return Result{
		CurrentVersion: plan.CurrentVersion,
		NewVersion:     plan.NewVersion,
		ChangedFiles:   plan.ChangedFiles(),
	}, nil
}
