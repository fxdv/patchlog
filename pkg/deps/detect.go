package deps

import (
	"regexp"
	"sort"
	"strings"
)

type depEntry struct {
	name    string
	version string
}

type lineParser func(line string) (depEntry, bool)

func Detect(filename, diff string) []Change {
	eco, ok := ecosystemForFile(filename)
	if !ok {
		return nil
	}

	parser := parserForEcosystem(eco)
	if parser == nil {
		return nil
	}

	removals, additions := splitDiffLines(diff)

	oldDeps := extractDeps(removals, parser)
	newDeps := extractDeps(additions, parser)

	var changes []Change

	for name, newVer := range newDeps {
		oldVer, existed := oldDeps[name]
		if !existed {
			changes = append(changes, Change{
				Name:       name,
				OldVersion: "",
				NewVersion: newVer,
				Ecosystem:  eco,
				Manifest:   filename,
			})
		} else if oldVer != newVer {
			changes = append(changes, Change{
				Name:       name,
				OldVersion: oldVer,
				NewVersion: newVer,
				Ecosystem:  eco,
				Manifest:   filename,
			})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})
	return changes
}

func DetectAll(diffs map[string]string) []Change {
	var filenames []string
	for f := range diffs {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	var changes []Change
	for _, f := range filenames {
		changes = append(changes, Detect(f, diffs[f])...)
	}
	return changes
}

func splitDiffLines(diff string) (removals, additions []string) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			removals = append(removals, line[1:])
		} else if strings.HasPrefix(line, "+") {
			additions = append(additions, line[1:])
		}
	}
	return
}

func extractDeps(lines []string, parse lineParser) map[string]string {
	deps := make(map[string]string)
	for _, line := range lines {
		if d, ok := parse(line); ok {
			deps[d.name] = d.version
		}
	}
	return deps
}

func parserForEcosystem(eco Ecosystem) lineParser {
	switch eco {
	case EcosystemNPM:
		return parseNPMLine
	case EcosystemCargo, EcosystemPyPI:
		return parseTOMLLine
	case EcosystemGo:
		return parseGoModLine
	default:
		return nil
	}
}

var (
	npmDepRe  = regexp.MustCompile(`["']([^"']+)["']\s*:\s*["']([^"']+)["']`)
	versionRe = regexp.MustCompile(`^[\^~>=<]*\d+\.\d+`)
	tomlSimRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*=\s*"([^"]+)"`)
	tomlTabRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*=\s*\{[^}]*version\s*=\s*"([^"]+)"[^}]*\}`)
	goModRe   = regexp.MustCompile(`^\s*(?:require\s+)?(\S+)\s+(v\S+)`)
	requireRe = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)\s*(==|>=|~=|<=|>|<)\s*([^\s;#]+)`)
)

func looksLikeVersion(v string) bool {
	return versionRe.MatchString(v)
}

var npmNonDepKeys = map[string]bool{
	"name": true, "version": true, "description": true, "main": true,
	"author": true, "license": true, "type": true, "engines": true,
	"bin": true, "types": true, "typings": true, "module": true,
	"exports": true, "files": true, "keywords": true, "homepage": true,
	"repository": true, "bugs": true, "scripts": true, "eslintConfig": true,
	"babel": true, "browserslist": true, "sideEffects": true, "private": true,
	"workspaces": true, "packageManager": true, "node": true, "npm": true,
	"gitHead": true, "readme": true, "directories": true, "publishConfig": true,
	"lint-staged": true, "husky": true, "jest": true, "mocha": true,
	"prettier": true, "vite": true, "webpack": true, "rollup": true,
	"ts-node": true, "peer-dependencies": true, "bundleDependencies": true,
	"optionalDependencies": true, "overrides": true, "resolutions": true,
}

func parseNPMLine(line string) (depEntry, bool) {
	m := npmDepRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return depEntry{}, false
	}
	name := m[1]
	if npmNonDepKeys[name] {
		return depEntry{}, false
	}
	if !looksLikeVersion(m[2]) {
		return depEntry{}, false
	}
	return depEntry{name: name, version: m[2]}, true
}

var tomlNonDepKeys = map[string]bool{
	"name": true, "version": true, "edition": true, "authors": true,
	"description": true, "license": true, "readme": true, "homepage": true,
	"repository": true, "keywords": true, "categories": true,
	"rust-version": true, "build": true, "path": true, "include": true,
	"exclude": true, "workspace": true, "default-run": true,
	"autobins": true, "autoexamples": true, "autotests": true,
	"autobenches": true, "documentation": true, "links": true,
}

func parseTOMLLine(line string) (depEntry, bool) {
	line = strings.TrimSpace(line)
	if m := tomlTabRe.FindStringSubmatch(line); m != nil {
		if tomlNonDepKeys[m[1]] {
			return depEntry{}, false
		}
		return depEntry{name: m[1], version: m[2]}, true
	}
	if m := tomlSimRe.FindStringSubmatch(line); m != nil {
		if tomlNonDepKeys[m[1]] {
			return depEntry{}, false
		}
		if !looksLikeVersion(m[2]) {
			return depEntry{}, false
		}
		return depEntry{name: m[1], version: m[2]}, true
	}
	if m := requireRe.FindStringSubmatch(line); m != nil {
		return depEntry{name: m[1], version: m[3]}, true
	}
	return depEntry{}, false
}

func parseGoModLine(line string) (depEntry, bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "go ") ||
		strings.HasPrefix(line, "toolchain ") || strings.HasPrefix(line, "require (") ||
		strings.HasPrefix(line, ")") || strings.HasPrefix(line, "//") {
		return depEntry{}, false
	}
	line = strings.TrimPrefix(line, "require ")
	m := goModRe.FindStringSubmatch(line)
	if m == nil {
		return depEntry{}, false
	}
	name := m[1]
	if name == "go" || name == "module" || name == "toolchain" {
		return depEntry{}, false
	}
	return depEntry{name: name, version: m[2]}, true
}
