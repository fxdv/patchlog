# Patchlog

Patchlog turns Git history into release notes and can safely coordinate version bumps, tags, atomic pushes, provider releases, changelogs, and Confluence pages.

## Install

With Go 1.22 or newer:

```bash
go install github.com/fxdv/patchlog/cmd/patchlog@latest
patchlog --version
```

Release archives and `SHA256SUMS` are published on the [GitHub releases page](https://github.com/fxdv/patchlog/releases). Archives also receive signed Sigstore provenance attestations, verifiable with `gh attestation verify <archive> --repo fxdv/patchlog`.

## Quick start

```bash
# Generate Markdown release notes without mutating the repository.
patchlog --from v0.1.0 --to HEAD

# Inspect the complete release plan without changing local or remote state.
patchlog release --bump auto --tag --push --publish --dry-run

# Apply the reviewed plan. Provider configuration is required for --publish.
patchlog release --bump auto --tag --push --publish
```

Publishing is intentionally strict: `--publish` requires `--tag --push`, and Patchlog verifies that the remote tag resolves to the local release commit before creating the provider release.

## Documentation

- [Quick start](docs/QUICK_START.md)
- [Configuration and security](docs/CONFIGURATION.md)
- [Advanced workflows](docs/ADVANCED_WORKFLOWS.md)
- [Complete CLI and feature reference](docs/REFERENCE.md)
- [Security policy](SECURITY.md)

## Development

```bash
bash scripts/gate.sh
```

The gate runs formatting, vet, build, race-enabled package tests, integration tests, and orchestration E2E tests. CI also uploads coverage for `cmd/patchlog`, `internal`, and `pkg`; coverage is diagnostic and is not a release gate.

## License

MIT — see [LICENSE](LICENSE).
