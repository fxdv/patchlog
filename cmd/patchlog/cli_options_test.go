package main

import (
	"io"
	"strings"
	"testing"
)

func TestParseCLIReleaseDryRunAppliesSafeDefaults(t *testing.T) {
	opts, args, err := parseCLI([]string{"release", "--dry-run"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want none", args)
	}
	if !opts.releaseMode || !opts.dryRun {
		t.Fatalf("releaseMode=%v dryRun=%v, want both true", opts.releaseMode, opts.dryRun)
	}
	if opts.bumpLevel != "auto" || opts.tag || opts.push {
		t.Fatalf("safe release defaults = bump %q, tag=%v, push=%v", opts.bumpLevel, opts.tag, opts.push)
	}
	if opts.publish || opts.confluence || opts.aiEnhance || opts.metrics || opts.labs {
		t.Fatal("safe release defaults enabled an optional extension")
	}
}

func TestParseCLIReleaseWithoutPhaseRetainsProtectedPrepareDefaults(t *testing.T) {
	opts, _, err := parseCLI([]string{"release"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.bumpLevel != "auto" || opts.tag || opts.push {
		t.Fatalf("safe release defaults = bump %q, tag=%v, push=%v", opts.bumpLevel, opts.tag, opts.push)
	}
}

func TestParseCLIProtectedFinalizeDoesNotPlanAnotherBump(t *testing.T) {
	opts, _, err := parseCLI([]string{"release", "finalize", "--approve", "sha256:abc"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.releaseAction != "finalize" || opts.bumpLevel != "" || !opts.tag || !opts.push {
		t.Fatalf("finalize = action %q, bump %q, tag=%v, push=%v", opts.releaseAction, opts.bumpLevel, opts.tag, opts.push)
	}
	if opts.approvePlan != "sha256:abc" {
		t.Fatalf("approve = %q", opts.approvePlan)
	}
}

func TestParseCLIExplicitDirectModeRetainsDirectDefaults(t *testing.T) {
	opts, _, err := parseCLI([]string{"release", "direct"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.releaseAction != "direct" || opts.bumpLevel != "auto" || !opts.tag || !opts.push {
		t.Fatalf("direct = action %q, bump %q, tag=%v, push=%v", opts.releaseAction, opts.bumpLevel, opts.tag, opts.push)
	}
}

func TestParseCLIManualBumpRequiresProtectedPrepare(t *testing.T) {
	_, _, err := parseCLI([]string{"release", "--bump", "minor"}, io.Discard)
	if err == nil || !containsAll(err.Error(), "prepare", "--bump minor", "--dry-run") {
		t.Fatalf("manual bump error = %v", err)
	}

	opts, _, err := parseCLI([]string{"release", "prepare", "--bump", "minor", "--dry-run"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.releaseAction != "prepare" || opts.bumpLevel != "minor" || opts.tag || opts.push {
		t.Fatalf("protected prepare = action %q, bump %q, tag=%v, push=%v", opts.releaseAction, opts.bumpLevel, opts.tag, opts.push)
	}
}

func TestParseCLIDirectOnlyFlagsRequireExplicitDirectMode(t *testing.T) {
	for _, flag := range []string{"--tag", "--push", "--force", "--changelog"} {
		_, _, err := parseCLI([]string{"release", flag}, io.Discard)
		if err == nil || !containsAll(err.Error(), "explicit compatibility mode", "release direct") {
			t.Errorf("%s error = %v", flag, err)
		}
	}
}

func TestParseCLIPublishRequiresExplicitFinalize(t *testing.T) {
	_, _, err := parseCLI([]string{"release", "--publish"}, io.Discard)
	if err == nil || !containsAll(err.Error(), "finalize", "--publish", "--dry-run") {
		t.Fatalf("publish error = %v", err)
	}
	opts, _, err := parseCLI([]string{"release", "finalize", "--publish", "--dry-run"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.publish || !opts.tag || !opts.push {
		t.Fatalf("finalize publish = publish=%v tag=%v push=%v", opts.publish, opts.tag, opts.push)
	}
}

func TestParseCLIPlanJSONIsProtectedDryRunOnly(t *testing.T) {
	for _, args := range [][]string{
		{"--plan-json"},
		{"release", "--plan-json"},
		{"release", "direct", "--dry-run", "--plan-json"},
	} {
		if _, _, err := parseCLI(args, io.Discard); err == nil {
			t.Fatalf("%v unexpectedly accepted --plan-json", args)
		}
	}
	opts, _, err := parseCLI([]string{"release", "--dry-run", "--plan-json"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.planJSON || !opts.dryRun || !opts.releaseMode {
		t.Fatalf("plan json options = %#v", opts)
	}
}

func TestParseCLIExtensionFlagsRequireDedicatedSubcommands(t *testing.T) {
	if _, _, err := parseCLI([]string{"--metrics"}, io.Discard); err == nil {
		t.Fatal("global --metrics unexpectedly remained available")
	}
	opts, _, err := parseCLI([]string{"metrics"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.extensionMode != "metrics" || !opts.metrics {
		t.Fatalf("metrics scope = %q, enabled=%v", opts.extensionMode, opts.metrics)
	}
}

func TestParseCLINoteGenerationDoesNotApplyReleaseDefaults(t *testing.T) {
	opts, _, err := parseCLI(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.releaseMode || opts.bumpLevel != "" || opts.tag || opts.push {
		t.Fatalf("read-only defaults changed: release=%v, bump=%q, tag=%v, push=%v", opts.releaseMode, opts.bumpLevel, opts.tag, opts.push)
	}
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
