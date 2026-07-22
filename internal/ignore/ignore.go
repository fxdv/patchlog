package ignore

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func Compile(patterns []string) *regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	var valid []string
	for _, p := range patterns {
		_, err := regexp.Compile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid ignore pattern %q: %v\n", p, err)
			continue
		}
		valid = append(valid, p)
	}
	if len(valid) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("^(?:")
	for i, p := range valid {
		if i > 0 {
			sb.WriteByte('|')
		}
		sb.WriteString("(?:")
		sb.WriteString(p)
		sb.WriteByte(')')
	}
	sb.WriteByte(')')
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil
	}
	return re
}
