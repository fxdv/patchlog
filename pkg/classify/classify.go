// Package classify assigns significance levels (skip/patch/minor/major) to commits based on diff analysis.
package classify

import (
	"strconv"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/significance"
)

type Level = significance.Level

const (
	Skip  = significance.Skip
	Patch = significance.Patch
	Minor = significance.Minor
	Major = significance.Major
)

type Result struct {
	Level   Level
	Reason  string
	Changed int
}

type Thresholds struct {
	LargeFeatureFiles int
	LargeFixFiles     int
	LargeUnknownFiles int
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		LargeFeatureFiles: 5,
		LargeFixFiles:     5,
		LargeUnknownFiles: 3,
	}
}

func Classify(c commit.Commit, changedFiles int) Result {
	return ClassifyWithThresholds(c, changedFiles, DefaultThresholds())
}

// ClassifyWithThresholds determines the significance level of a commit.
// Classification is always recomputed from git data — it must never be cached,
// as git is the source of truth for commit metadata.
func ClassifyWithThresholds(c commit.Commit, changedFiles int, th Thresholds) Result {
	if c.Breaking {
		return Result{Major, "breaking change", changedFiles}
	}

	switch c.Type {
	case "feat":
		if changedFiles >= th.LargeFeatureFiles {
			return Result{Major, "large feature (" + plural(changedFiles, "file") + ")", changedFiles}
		}
		return Result{Minor, "new feature", changedFiles}
	case "fix":
		if changedFiles >= th.LargeFixFiles {
			return Result{Minor, "large fix (" + plural(changedFiles, "file") + ")", changedFiles}
		}
		return Result{Patch, "bug fix", changedFiles}
	case "perf":
		return Result{Minor, "performance improvement", changedFiles}
	case "refactor":
		return Result{Patch, "code refactoring", changedFiles}
	case "docs":
		return Result{Skip, "documentation", changedFiles}
	case "test":
		return Result{Skip, "tests", changedFiles}
	case "style":
		return Result{Skip, "style/formatting", changedFiles}
	case "ci":
		return Result{Skip, "CI/build", changedFiles}
	case "chore":
		return Result{Skip, "maintenance", changedFiles}
	default:
		if changedFiles >= th.LargeUnknownFiles {
			return Result{Minor, "significant change (" + plural(changedFiles, "file") + ")", changedFiles}
		}
		return Result{Patch, "other change", changedFiles}
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
