# Patchlog

Patchlog is a safe release coordinator for Git repositories. It turns commit
history into a deterministic release plan, validates every local and remote
action before mutation, then transactionally bumps, tags, and atomically pushes.

## Safe release in two commands

From a clean repository with a version file:

```bash
# Immutable planning and preflight: no files, refs, caches, or remotes change.
patchlog release --dry-run

# Apply the same safe core workflow: auto-bump, annotated tag, atomic push.
patchlog release
```

The default path does not call AI, publish a provider release, write a
changelog, or contact Confluence. Add those extensions only when you need them.

## Install

With Go 1.22 or newer:

```bash
go install github.com/fxdv/patchlog/cmd/patchlog@latest
patchlog --version
```

Release archives and `SHA256SUMS` are published on the [GitHub releases page](https://github.com/fxdv/patchlog/releases). Archives also receive signed Sigstore provenance attestations, verifiable with `gh attestation verify <archive> --repo fxdv/patchlog`.

Homebrew is the first planned package-manager channel, followed by Scoop.
Checksum-pinned manifests are already produced by the release workflow, but the
tap and bucket are not advertised as live until their external repositories and
automated installation checks are operational.

## Read-only notes

```bash
patchlog --from v0.1.0 --to HEAD
```

Plain `patchlog` is reporting-only. Provider releases, changelogs, AI-assisted
writing, Confluence analytics, metrics, and experimental labs are optional
advanced workflows. Publishing remains strict: `--publish` requires an
immutable, remotely verified tag.

## Documentation

- [Quick start](docs/QUICK_START.md)
- [Configuration and security](docs/CONFIGURATION.md)
- [Advanced workflows](docs/ADVANCED_WORKFLOWS.md)
- [0.2.0 stability contract](docs/STABILITY.md)
- [Package-manager publishing](packaging/README.md)
- [Complete CLI and feature reference](docs/REFERENCE.md)
- [Security policy](SECURITY.md)

## Development

```bash
bash scripts/gate.sh
```

The gate runs formatting, vet, build, race-enabled package tests, integration tests, and orchestration E2E tests. CI also uploads coverage for `cmd/patchlog`, `internal`, and `pkg`; coverage is diagnostic and is not a release gate.

## License

MIT — see [LICENSE](LICENSE).
