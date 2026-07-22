// Package significance defines significance levels (skip, patch, minor, major) for commit classification.
package significance

import "fmt"

type Level int

const (
	Skip Level = iota
	Patch
	Minor
	Major
)

func (l Level) String() string {
	switch l {
	case Skip:
		return "skip"
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
	case "skip":
		return Skip, nil
	case "patch":
		return Patch, nil
	case "minor":
		return Minor, nil
	case "major":
		return Major, nil
	default:
		return Patch, fmt.Errorf("unknown significance level %q", s)
	}
}
