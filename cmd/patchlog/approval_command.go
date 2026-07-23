package main

import (
	"os"
	"strings"
)

// approvalCommand preserves every planning input while replacing the
// side-effect-free universal release command with its resolved phase. This
// keeps custom repository, configuration, bump, branch, and publish options in
// the exact command used to approve the fingerprint.
func approvalCommand(phase ReleasePhase, fingerprint string) string {
	args := append([]string(nil), os.Args[1:]...)
	filtered := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run", strings.HasPrefix(arg, "--dry-run="):
			continue
		case arg == "--approve":
			i++
			continue
		case strings.HasPrefix(arg, "--approve="):
			continue
		default:
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) > 0 && filtered[0] == "release" {
		if len(filtered) > 1 && isReleasePhaseArgument(filtered[1]) {
			filtered[1] = string(phase)
		} else {
			filtered = append(filtered[:1], append([]string{string(phase)}, filtered[1:]...)...)
		}
	}
	filtered = append(filtered, "--approve", fingerprint)
	command := make([]string, 0, len(filtered)+1)
	command = append(command, "patchlog")
	for _, arg := range filtered {
		command = append(command, shellQuote(arg))
	}
	return strings.Join(command, " ")
}

func isReleasePhaseArgument(value string) bool {
	switch ReleasePhase(value) {
	case ReleasePhasePrepare, ReleasePhaseFinalize, ReleasePhaseDirect:
		return true
	default:
		return false
	}
}

func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n'\"`$\\;&|<>()[]{}*?!#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
