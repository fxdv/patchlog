package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type hintedError struct {
	err  error
	hint string
}

func (e *hintedError) Error() string { return e.err.Error() }

func (e *hintedError) Unwrap() error { return e.err }

func withHint(err error, hint string) error {
	if err == nil {
		return nil
	}
	return &hintedError{err: err, hint: hint}
}

func errorHint(err error) string {
	var hinted *hintedError
	if errors.As(err, &hinted) {
		return hinted.hint
	}
	return ""
}

func writeConfigError(out io.Writer, path string, err error) {
	fmt.Fprintf(out, "Configuration error in %s:\n  %v\n", path, err)
	fmt.Fprintln(out, "Next: run `patchlog init` to create a valid configuration, or fix the field above and retry `patchlog release --dry-run`.")
}

func writeReleasePlanError(out io.Writer, err error) {
	fmt.Fprintf(out, "Release plan error:\n  %v\n", err)
	fmt.Fprintf(out, "Preflight rejection: %s\n", preflightRejectionCategory(err))
	hint := errorHint(err)
	if hint == "" {
		hint = "fix the error above, then rerun `patchlog release --dry-run` before applying"
	}
	fmt.Fprintf(out, "Next: %s.\n", hint)
}

// preflightRejectionCategory emits a stable, content-free diagnostic key for
// opt-in workflow measurement. The full error remains for humans, while the
// category can be aggregated without collecting repository paths or source.
func preflightRejectionCategory(err error) string {
	message := strings.ToLower(err.Error())
	containsAny := func(fragments ...string) bool {
		for _, fragment := range fragments {
			if strings.Contains(message, fragment) {
				return true
			}
		}
		return false
	}
	switch {
	case containsAny("does not match current plan", "changed after planning"):
		return "stale_fingerprint"
	case containsAny("clean worktree", "working tree", "worktree; found"):
		return "dirty_worktree"
	case containsAny("protected branch is not current", "green protected commit mismatch"):
		return "stale_protected_branch"
	case strings.Contains(message, "release branch") && strings.Contains(message, "already exists"):
		return "occupied_release_branch"
	case containsAny("does not match target version", "behind latest release tag"):
		return "version_tag_mismatch"
	case strings.Contains(message, "tag") && strings.Contains(message, "already exists"):
		return "remote_tag_collision"
	case containsAny("missing required configuration", "configuration is incomplete"):
		return "missing_configuration"
	case containsAny("no version file found", "version field not found", "version mismatch"):
		return "version_detection"
	case containsAny("detached head"):
		return "detached_head"
	}
	return "other"
}
