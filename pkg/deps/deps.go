// Package deps detects dependency version bumps in manifest files and fetches upstream changelogs.
package deps

import "strings"

type Ecosystem string

const (
	EcosystemNPM   Ecosystem = "npm"
	EcosystemCargo Ecosystem = "cargo"
	EcosystemGo    Ecosystem = "go"
	EcosystemPyPI  Ecosystem = "pypi"
)

type Change struct {
	Name         string    `json:"name"`
	OldVersion   string    `json:"old_version"`
	NewVersion   string    `json:"new_version"`
	Ecosystem    Ecosystem `json:"ecosystem"`
	Manifest     string    `json:"manifest"`
	Changelog    string    `json:"changelog,omitempty"`
	ChangelogURL string    `json:"changelog_url,omitempty"`
}

type FetchOptions struct {
	MaxDependencies int
	GitHubReleases  bool
	NPMRegistry     string
	CratesRegistry  string
	PyPIRegistry    string
}

var manifestRegistry = []struct {
	files     []string
	ecosystem Ecosystem
}{
	{[]string{"package.json"}, EcosystemNPM},
	{[]string{"Cargo.toml"}, EcosystemCargo},
	{[]string{"go.mod"}, EcosystemGo},
	{[]string{"pyproject.toml", "requirements.txt"}, EcosystemPyPI},
}

func ManifestFiles() []string {
	var files []string
	for _, m := range manifestRegistry {
		files = append(files, m.files...)
	}
	return files
}

func IsManifestFile(path string) bool {
	base := baseName(path)
	for _, m := range manifestRegistry {
		for _, f := range m.files {
			if base == f {
				return true
			}
		}
	}
	return false
}

func ecosystemForFile(path string) (Ecosystem, bool) {
	base := baseName(path)
	for _, m := range manifestRegistry {
		for _, f := range m.files {
			if base == f {
				return m.ecosystem, true
			}
		}
	}
	return "", false
}

func baseName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func stripVersionPrefix(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimLeft(v, "^~>=<!")
	v = strings.TrimPrefix(v, "v")
	return v
}

func compareVersions(a, b string) int {
	a = stripVersionPrefix(a)
	b = stripVersionPrefix(b)
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			aNum = parseVersionSegment(aParts[i])
		}
		if i < len(bParts) {
			bNum = parseVersionSegment(bParts[i])
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}

func parseVersionSegment(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			n := 0
			for j := 0; j < i; j++ {
				n = n*10 + int(s[j]-'0')
			}
			return n
		}
	}
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func isVersionBetween(tag, oldVer, newVer string) bool {
	tagClean := stripVersionPrefix(tag)
	oldClean := stripVersionPrefix(oldVer)
	newClean := stripVersionPrefix(newVer)

	if oldClean != "" && compareVersions(tagClean, oldClean) <= 0 {
		return false
	}
	if newClean != "" && compareVersions(tagClean, newClean) > 0 {
		return false
	}
	return true
}
