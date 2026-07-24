package main

import (
	"fmt"
	"os"
)

// dispatchAdvancedCommand keeps maintenance and analytical workflows out of
// the primary release coordinator. They remain available for compatibility
// and focused use without expanding the golden-path help surface.
func dispatchAdvancedCommand(args []string, repo string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "init":
		cmdInit()
	case "lint":
		runLint(args[1:])
	case "audit":
		runAudit(args[1:])
	case "multi":
		runMultiRepo(args[1:])
	case "recover":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: patchlog recover <json-file>")
			os.Exit(2)
		}
		cmdRecover(args[1])
	case "cache":
		if len(args) < 2 || args[1] != "clear" {
			fmt.Fprintln(os.Stderr, "Usage: patchlog cache clear")
			os.Exit(2)
		}
		cmdCacheClear(repo)
	case "trends":
		runTrends(args[1:], repo)
	case "curate":
		runCurate(args[1:], repo)
	case "postmortem":
		runPostmortem(args[1:], repo)
	default:
		return false
	}
	return true
}
