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
	for _, want := range []string{"missing provider.token", "Preflight rejection: other", "set `provider.token`", "patchlog release --dry-run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error output %q does not contain %q", got, want)
		}
	}
}

func TestPreflightRejectionCategoryDoesNotExposeDetails(t *testing.T) {
	cases := map[string]string{
		"protected branch is not current: local main is abc, origin/main is def":           "stale_protected_branch",
		"local release branch release/v1.2.3 already exists":                               "occupied_release_branch",
		"approved plan sha256:old does not match current plan sha256:new":                  "stale_fingerprint",
		"publish preflight missing required configuration: provider.token":                 "missing_configuration",
		"detect repository version: detect version in Cargo.toml: version field not found": "version_detection",
	}
	for message, want := range cases {
		if got := preflightRejectionCategory(errors.New(message)); got != want {
			t.Errorf("preflightRejectionCategory(%q) = %q, want %q", message, got, want)
		}
	}
}
