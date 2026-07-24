package main

import (
	"flag"
	"fmt"
	"io"
)

// printCLIUsage keeps the primary surface centered on safe release
// coordination. Compatibility and analytical workflows remain available but
// do not compete with the protected golden path.
func printCLIUsage(fs *flag.FlagSet, out io.Writer) {
	printBanner()
	fmt.Fprintln(out, "\nUsage:")
	fmt.Fprintln(out, "  patchlog release --dry-run")
	fmt.Fprintln(out, "  patchlog release prepare --approve HASH")
	fmt.Fprintln(out, "  patchlog release finalize --approve HASH")
	fmt.Fprintln(out, "  patchlog [flags]  Generate release notes without mutation")
	fmt.Fprintln(out, "\nSafe release coordination:")
	fmt.Fprintln(out, "  release           Produce the next immutable protected-release plan")
	fmt.Fprintln(out, "  release prepare   Push an approved version-bump branch")
	fmt.Fprintln(out, "  release finalize  Verify policy and tag the exact protected commit")
	fmt.Fprintln(out, "\nOptional extensions:")
	fmt.Fprintln(out, "  ai           AI-assisted release-note workflows")
	fmt.Fprintln(out, "  confluence   Confluence publication")
	fmt.Fprintln(out, "  metrics      Diagnostic repository-level metrics")
	fmt.Fprintln(out, "  labs         Experimental health, DPI, and gamification")
	fmt.Fprintln(out, "\nCompatibility:")
	fmt.Fprintln(out, "  release direct   Explicit legacy direct commit/tag/push workflow")
	fmt.Fprintln(out, "\nCore flags:")
	printCoreCLIFlags(fs, out)
	fmt.Fprintln(out, "\nGolden path:")
	fmt.Fprintln(out, "  patchlog release --dry-run")
	fmt.Fprintln(out, "  patchlog release prepare --approve sha256:<fingerprint>")
	fmt.Fprintln(out, "  patchlog release finalize --dry-run")
	fmt.Fprintln(out, "  patchlog release finalize --approve sha256:<fingerprint>")
	fmt.Fprintln(out, "\nAdvanced command reference: https://github.com/fxdv/patchlog/blob/main/docs/REFERENCE.md")
}

func printCoreCLIFlags(fs *flag.FlagSet, out io.Writer) {
	for _, name := range []string{
		"approve",
		"config",
		"dry-run",
		"first",
		"format",
		"from",
		"lang",
		"no-cache",
		"out",
		"plan-json",
		"quiet",
		"release-branch",
		"repo",
		"to",
		"version",
	} {
		entry := fs.Lookup(name)
		if entry == nil {
			continue
		}
		defaultValue := ""
		if entry.DefValue != "" && entry.DefValue != "false" {
			defaultValue = fmt.Sprintf(" (default %s)", entry.DefValue)
		}
		fmt.Fprintf(out, "  --%-16s %s%s\n", entry.Name, entry.Usage, defaultValue)
	}
}
