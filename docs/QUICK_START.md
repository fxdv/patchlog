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

Run the universal protected-release planner:

```bash
patchlog release --dry-run
```

This detects whether the repository needs `prepare` or `finalize`, performs
every deterministic preflight, and prints a content-addressed approval
fingerprint. It does not write files, Git refs, caches, reports, changelogs, or
remote resources.

For prepare, approve the exact fingerprint:

```bash
patchlog release prepare --approve sha256:<fingerprint>
```

Patchlog creates `release/vX.Y.Z`, transactionally bumps only the exact planned
files, commits through an isolated index, and pushes that branch without
creating a tag. Open and merge its pull request only after required CI passes.
After green post-merge CI:

```bash
patchlog release --dry-run
patchlog release finalize --approve sha256:<fingerprint>
```

The protected path needs:

- a clean worktree;
- a discoverable version source such as `VERSION`;
- an `origin` remote with the current branch configured;
- a local protected branch matching its exact remote commit;
- at least one releasable commit since the previous tag.

When configuration or preflight fails, Patchlog names the missing fields and
prints the exact recovery command. Fix the stated condition and rerun
`patchlog release --dry-run`.

The default path intentionally excludes AI, Confluence, metrics, and labs.
Those extensions have dedicated subcommands. Direct release coordination is an
explicit compatibility mode:

```bash
patchlog release direct --bump auto --tag --push --publish --dry-run
patchlog release direct --bump auto --tag --push --publish --approve sha256:<fingerprint>
```

Publishing requires the atomic tag push so a release cannot point at an
implicit provider default branch.

## Next steps

- Configure integrations in [CONFIGURATION.md](CONFIGURATION.md).
- See focused release and multi-service examples in [ADVANCED_WORKFLOWS.md](ADVANCED_WORKFLOWS.md).
- Consult [REFERENCE.md](REFERENCE.md) for every flag and feature.
