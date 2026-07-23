package main

import (
	"io"
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
	if opts.bumpLevel != "auto" || !opts.tag || !opts.push {
		t.Fatalf("safe release defaults = bump %q, tag=%v, push=%v", opts.bumpLevel, opts.tag, opts.push)
	}
	if opts.publish || opts.confluence || opts.aiEnhance || opts.metrics || opts.labs {
		t.Fatal("safe release defaults enabled an optional extension")
	}
}

func TestParseCLIReleaseApplyAppliesSafeDefaults(t *testing.T) {
	opts, _, err := parseCLI([]string{"release"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.bumpLevel != "auto" || !opts.tag || !opts.push {
		t.Fatalf("safe release defaults = bump %q, tag=%v, push=%v", opts.bumpLevel, opts.tag, opts.push)
	}
}

func TestParseCLIExplicitReleaseActionPreservesUserSelection(t *testing.T) {
	opts, _, err := parseCLI([]string{"release", "--bump", "minor"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.bumpLevel != "minor" || opts.tag || opts.push {
		t.Fatalf("explicit release selection changed: bump %q, tag=%v, push=%v", opts.bumpLevel, opts.tag, opts.push)
	}
}

func TestParseCLIForceModifiesSafeDefaultsInsteadOfSelectingAnEmptyWorkflow(t *testing.T) {
	opts, _, err := parseCLI([]string{"release", "--force"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.bumpLevel != "auto" || !opts.tag || !opts.push || !opts.force {
		t.Fatalf("forced safe release = bump %q, tag=%v, push=%v, force=%v", opts.bumpLevel, opts.tag, opts.push, opts.force)
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
