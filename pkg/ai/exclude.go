package ai

import (
	"path"
	"strings"
)

// PathExcluded applies slash-separated glob patterns, including **, to a
// repository-relative path.
func PathExcluded(file string, patterns []string) bool {
	file = strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./")
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(strings.ReplaceAll(pattern, "\\", "/"), "./")
		if matchDoubleStar(pattern, file) {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if matched, _ := path.Match(pattern, path.Base(file)); matched {
				return true
			}
		}
	}
	return false
}

func matchDoubleStar(pattern, file string) bool {
	patternParts := splitPath(pattern)
	fileParts := splitPath(file)
	type state struct{ pattern, file int }
	memo := make(map[state]bool)
	visited := make(map[state]bool)

	var match func(int, int) bool
	match = func(patternIndex, fileIndex int) bool {
		key := state{pattern: patternIndex, file: fileIndex}
		if visited[key] {
			return memo[key]
		}
		visited[key] = true

		var result bool
		switch {
		case patternIndex == len(patternParts):
			result = fileIndex == len(fileParts)
		case patternParts[patternIndex] == "**":
			result = match(patternIndex+1, fileIndex) ||
				(fileIndex < len(fileParts) && match(patternIndex, fileIndex+1))
		case fileIndex < len(fileParts):
			segmentMatches, err := path.Match(patternParts[patternIndex], fileParts[fileIndex])
			result = err == nil && segmentMatches && match(patternIndex+1, fileIndex+1)
		}
		memo[key] = result
		return result
	}

	return match(0, 0)
}

func splitPath(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}
