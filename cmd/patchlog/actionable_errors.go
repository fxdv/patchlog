package main

import (
	"errors"
	"fmt"
	"io"
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
	hint := errorHint(err)
	if hint == "" {
		hint = "fix the error above, then rerun `patchlog release --dry-run` before applying"
	}
	fmt.Fprintf(out, "Next: %s.\n", hint)
}
