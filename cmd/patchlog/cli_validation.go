package main

import "fmt"

func validateCLIContract(opts cliOptions, args []string) error {
	if opts.push && !opts.tag {
		return fmt.Errorf("--push requires --tag")
	}
	if opts.tag && opts.bumpLevel == "" && opts.releaseAction != "finalize" {
		return fmt.Errorf("--tag requires --bump")
	}
	if opts.gamify && !opts.labs {
		return fmt.Errorf("--gamify is experimental and requires --labs")
	}
	if !opts.releaseMode &&
		(opts.bumpLevel != "" || opts.tag || opts.push || opts.force || opts.publish || opts.changelog ||
			(opts.confluence && opts.extensionMode != "confluence") ||
			(opts.trends && opts.extensionMode != "confluence")) {
		return fmt.Errorf("release mutations require the focused 'patchlog release' subcommand")
	}
	if opts.releaseMode && len(args) > 0 {
		return fmt.Errorf("unexpected release argument %q", args[0])
	}
	return nil
}
