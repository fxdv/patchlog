# Quick start

## Requirements

- Git
- Go 1.22 or newer when installing from source
- A clean worktree for release mutations, unless `--force` is explicitly chosen

## Installation

```bash
go install github.com/fxdv/patchlog/cmd/patchlog@latest
patchlog --version
```

Alternatively, download the archive for your platform and verify it against `SHA256SUMS` from the same GitHub release.

## Generate release notes

```bash
patchlog
patchlog --from v1.2.0 --to HEAD
patchlog --from v1.2.0 --format json --out release.json
```

These reporting commands do not perform release mutations.

## Prepare a release

Run the complete safe default release as a two-step review/apply sequence:

```bash
patchlog release --dry-run
```

This plans an automatic semantic version bump, an isolated commit, an annotated
tag, and an atomic branch/tag push. It performs the same deterministic preflight
as Apply but does not write files, Git refs, caches, reports, changelogs, or
remote resources.

After reviewing the plan:

```bash
patchlog release
```

The golden path needs:

- a clean worktree;
- a discoverable version source such as `VERSION`;
- an `origin` remote with the current branch configured;
- at least one releasable commit since the previous tag.

When configuration or preflight fails, Patchlog names the missing fields and
prints the exact recovery command. Fix the stated condition and rerun
`patchlog release --dry-run`.

The default path intentionally excludes provider publishing, changelog writes,
AI, Confluence, metrics, and labs. To compose a custom workflow, provide its
release flags explicitly. For example, a provider release requires:

```bash
patchlog release --bump auto --tag --push --publish --dry-run
patchlog release --bump auto --tag --push --publish
```

Publishing requires the atomic tag push so a release cannot point at an
implicit provider default branch.

## Next steps

- Configure integrations in [CONFIGURATION.md](CONFIGURATION.md).
- See focused release and multi-service examples in [ADVANCED_WORKFLOWS.md](ADVANCED_WORKFLOWS.md).
- Consult [REFERENCE.md](REFERENCE.md) for every flag and feature.
