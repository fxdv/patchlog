package pattern

import "regexp"

var JiraKeyRe = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

func ExtractKeys(text string) []string {
	matches := JiraKeyRe.FindAllString(text, -1)
	seen := make(map[string]bool)
	var keys []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			keys = append(keys, m)
		}
	}
	return keys
}
