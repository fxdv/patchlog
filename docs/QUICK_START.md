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

Patchlog contains a root `VERSION` file and can therefore use its own bump workflow:

```bash
patchlog release \
  --bump auto \
  --tag \
  --push \
  --dry-run
```

Dry-run performs the same deterministic preflight as Apply but does not write files, Git refs, caches, reports, changelogs, or remote resources.

After reviewing the plan:

```bash
patchlog release --bump auto --tag --push
```

To create a provider release as well, configure a provider and add `--publish`. Publishing requires the atomic tag push so the release cannot point at an implicit provider default branch.

## Next steps

- Configure integrations in [CONFIGURATION.md](CONFIGURATION.md).
- See focused release and multi-service examples in [ADVANCED_WORKFLOWS.md](ADVANCED_WORKFLOWS.md).
- Consult [REFERENCE.md](REFERENCE.md) for every flag and feature.
