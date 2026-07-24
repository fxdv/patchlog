package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/console"
)

type releaseDryRunView struct {
	Plan              *ReleasePlan
	Phase             ReleasePhase
	Bump              *bump.Plan
	ProtectedBranch   string
	ReleaseBranch     string
	TagName           string
	TagRequested      bool
	PushRequested     bool
	PublishRequested  bool
	Confluence        bool
	Changelog         bool
	HTML              bool
	TrendSnapshotPath string
	Quiet             bool
}

func (v releaseDryRunView) Render(out io.Writer) error {
	if v.Plan == nil {
		return fmt.Errorf("release dry-run output requires a plan")
	}
	if !v.Quiet {
		switch v.Phase {
		case ReleasePhasePrepare:
			console.Step(fmt.Sprintf("[dry-run] Would create release branch: %s from origin/%s", v.ReleaseBranch, v.ProtectedBranch))
			console.Step(fmt.Sprintf("[dry-run] Would bump %s → %s in: %s", v.Bump.CurrentVersion, v.Bump.NewVersion, strings.Join(v.Bump.ChangedFiles(), ", ")))
			console.Step(fmt.Sprintf("[dry-run] Would commit only: %s", strings.Join(v.Bump.ChangedFiles(), ", ")))
			console.Step(fmt.Sprintf("[dry-run] Would push release branch: origin/%s", v.ReleaseBranch))
		case ReleasePhaseFinalize:
			console.Step(fmt.Sprintf("[dry-run] Would tag exact verified origin/%s commit %s as %s", v.ProtectedBranch, v.Plan.head, v.TagName))
			console.Step(fmt.Sprintf("[dry-run] Would push only immutable tag: %s", v.TagName))
		case ReleasePhaseDirect:
			if v.Bump != nil {
				console.Step(fmt.Sprintf("[dry-run] Would bump %s → %s in: %s", v.Bump.CurrentVersion, v.Bump.NewVersion, strings.Join(v.Bump.ChangedFiles(), ", ")))
			}
			if v.TagRequested {
				console.Step(fmt.Sprintf("[dry-run] Would commit only: %s", strings.Join(v.Bump.ChangedFiles(), ", ")))
				console.Step(fmt.Sprintf("[dry-run] Would create tag: %s", v.TagName))
				if v.PushRequested {
					console.Step("[dry-run] Would atomically push branch and tag")
				}
			}
		}
		if v.PublishRequested {
			console.Step("[dry-run] Would publish release draft to provider")
		}
		if v.Confluence {
			console.Step("[dry-run] Would publish to Confluence")
		}
		if v.Changelog {
			console.Step("[dry-run] Would accumulate changelog")
		}
		if v.HTML {
			console.Step("[dry-run] Would write an HTML report")
		}
		if v.TrendSnapshotPath != "" {
			console.Step(fmt.Sprintf("[dry-run] Would write trend snapshot: %s", v.TrendSnapshotPath))
		}
	}

	fmt.Fprintf(out, "Plan fingerprint: %s\n", v.Plan.Fingerprint())
	switch v.Phase {
	case ReleasePhasePrepare:
		fmt.Fprintf(out, "Next: %s\n", approvalCommand(v.Phase, v.Plan.Fingerprint()))
	case ReleasePhaseFinalize:
		fmt.Fprintf(out, "Next: required provider checks are green; run `%s`\n", approvalCommand(v.Phase, v.Plan.Fingerprint()))
	case ReleasePhaseDirect:
		fmt.Fprintf(out, "Next: %s\n", approvalCommand(v.Phase, v.Plan.Fingerprint()))
	}
	return nil
}
