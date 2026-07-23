package main

import (
	"os"
	"strings"
	"testing"
)

func TestApprovalCommandResolvesUniversalPhaseAndPreservesInputs(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{
		"patchlog", "release", "--dry-run", "--repo", "/tmp/repo with space",
		"--release-branch", "release/custom", "--publish",
	}

	got := approvalCommand(ReleasePhaseFinalize, "sha256:abc")
	want := "patchlog release finalize --repo '/tmp/repo with space' --release-branch release/custom --publish --approve sha256:abc"
	if got != want {
		t.Fatalf("approvalCommand() = %q, want %q", got, want)
	}
}

func TestApprovalCommandReplacesStaleApprovalAndKeepsDirectFlags(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{
		"patchlog", "release", "direct", "--bump=minor", "--tag", "--push",
		"--dry-run=true", "--approve", "sha256:old",
	}

	got := approvalCommand(ReleasePhaseDirect, "sha256:new")
	if strings.Contains(got, "old") || strings.Contains(got, "dry-run") {
		t.Fatalf("approvalCommand() retained stale planning flags: %q", got)
	}
	want := "patchlog release direct --bump=minor --tag --push --approve sha256:new"
	if got != want {
		t.Fatalf("approvalCommand() = %q, want %q", got, want)
	}
}
