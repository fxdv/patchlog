package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteConfigErrorIncludesRecoveryCommand(t *testing.T) {
	var out bytes.Buffer
	writeConfigError(&out, "patchlog.yaml", errors.New("field provider.tokne not found"))
	got := out.String()
	for _, want := range []string{"patchlog.yaml", "provider.tokne", "patchlog init", "patchlog release --dry-run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error output %q does not contain %q", got, want)
		}
	}
}

func TestWriteReleasePlanErrorUsesSpecificHint(t *testing.T) {
	var out bytes.Buffer
	err := withHint(errors.New("missing provider.token"), "set `provider.token` and rerun `patchlog release --dry-run`")
	writeReleasePlanError(&out, err)
	got := out.String()
	for _, want := range []string{"missing provider.token", "set `provider.token`", "patchlog release --dry-run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error output %q does not contain %q", got, want)
		}
	}
}
