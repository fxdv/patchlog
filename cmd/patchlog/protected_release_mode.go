package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/fxdv/patchlog/pkg/bump"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/gitlog"
	"github.com/fxdv/patchlog/pkg/gittag"
)

func resolveReleasePhase(
	ctx context.Context,
	requested string,
	dryRun bool,
	repo string,
	cfg config.Config,
	fetcher *gitlog.Fetcher,
) (ReleasePhase, string, string, error) {
	switch requested {
	case "prepare":
		return ReleasePhasePrepare, "", "", nil
	case "finalize":
		current, err := detectRepositoryVersion(repo, cfg)
		return ReleasePhaseFinalize, current, "", err
	case "direct":
		return ReleasePhaseDirect, "", "", nil
	case "":
		if !dryRun {
			return "", "", "", withHint(
				fmt.Errorf("protected releases require an immutable plan before mutation"),
				"run `patchlog release --dry-run`, then use the exact prepare or finalize command it prints",
			)
		}
	default:
		return "", "", "", fmt.Errorf("unsupported release action %q", requested)
	}

	current, err := detectRepositoryVersion(repo, cfg)
	if err != nil {
		return "", "", "", err
	}
	latestTag, err := fetcher.LatestStableTag(ctx, cfg.Release.TagPrefix)
	if err != nil {
		if !strings.Contains(err.Error(), "no stable release tags found") {
			return "", "", "", fmt.Errorf("detect latest release tag: %w", err)
		}
		// A repository with no tags is a bootstrap finalize: VERSION already
		// declares the first release identity, so no artificial bump PR is
		// required before the first annotated tag.
		return ReleasePhaseFinalize, current, "", nil
	}
	prefix := gittag.DetectPrefix(latestTag)
	tagVersion := strings.TrimPrefix(latestTag, prefix)
	cmp, err := compareSemanticVersions(current, tagVersion)
	if err != nil {
		return "", "", "", fmt.Errorf("compare VERSION %s with latest tag %s: %w", current, latestTag, err)
	}
	switch {
	case cmp == 0:
		return ReleasePhasePrepare, current, latestTag, nil
	case cmp > 0:
		return ReleasePhaseFinalize, current, latestTag, nil
	default:
		return "", "", "", fmt.Errorf("VERSION %s is behind latest release tag %s", current, latestTag)
	}
}

func detectRepositoryVersion(repo string, cfg config.Config) (string, error) {
	plan, err := bump.CreatePlan(repo, bump.Patch, cfg.Bump.Files, cfg.Bump.AutoDetect)
	if err != nil {
		return "", fmt.Errorf("detect repository version: %w", err)
	}
	return plan.CurrentVersion, nil
}

func compareSemanticVersions(left, right string) (int, error) {
	parse := func(raw string) ([3]int, error) {
		var result [3]int
		core := strings.SplitN(strings.TrimSpace(raw), "-", 2)[0]
		parts := strings.Split(core, ".")
		if len(parts) != 3 {
			return result, fmt.Errorf("%q is not MAJOR.MINOR.PATCH", raw)
		}
		for i, part := range parts {
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				return result, fmt.Errorf("%q contains an invalid numeric component", raw)
			}
			result[i] = value
		}
		return result, nil
	}
	a, err := parse(left)
	if err != nil {
		return 0, err
	}
	b, err := parse(right)
	if err != nil {
		return 0, err
	}
	for i := range a {
		if a[i] < b[i] {
			return -1, nil
		}
		if a[i] > b[i] {
			return 1, nil
		}
	}
	return 0, nil
}
