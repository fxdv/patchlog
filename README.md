# Patchlog

Patchlog is a safe release coordinator for protected Git repositories. It turns
commit history into a fingerprinted release plan, prepares an isolated
version-bump branch for review, proves the exact protected-branch commit passed
its required checks, then creates the immutable release tag.

## Safe protected release

From a clean repository with a version file:

```bash
# Universal immutable plan. It prints an exact approval fingerprint.
patchlog release --dry-run

# Optional machine-readable contract; stdout is versioned JSON.
patchlog release --dry-run --plan-json --quiet

# Apply only that approved plan: create, commit, and push release/vX.Y.Z.
patchlog release prepare --approve sha256:<fingerprint>
```

Open the pull request, require green CI, merge it, and wait for post-merge CI on
the protected branch. Patchlog then detects the finalize phase:

```bash
patchlog release --dry-run
patchlog release finalize --approve sha256:<fingerprint>
```

Finalize revalidates that local and remote protected-branch commits match,
queries GitHub for the branch's required checks on that exact SHA, checks the
policy again immediately before tag push, and binds the approved fingerprint
into the annotated tag. The default path does not call AI, write analytics,
contact Confluence, or combine optional extensions with the transaction.

## Install

With Go 1.22 or newer:

```bash
go install github.com/fxdv/patchlog/cmd/patchlog@latest
patchlog --version
```

Release archives and `SHA256SUMS` are published on the [GitHub releases page](https://github.com/fxdv/patchlog/releases). Archives and the versioned
[`patchlog-release-receipt.json`](docs/schemas/release-receipt-v1.schema.json)
receive signed Sigstore provenance attestations, verifiable with
`gh attestation verify <file> --repo fxdv/patchlog`.

Homebrew is live and installation-tested automatically:

```bash
brew tap fxdv/tap
brew install patchlog
patchlog --version
```

The [fxdv/homebrew-tap](https://github.com/fxdv/homebrew-tap) synchronizes the
checksum-pinned formula from stable Patchlog releases and installs/tests it
before committing an update. The Patchlog release workflow independently tests
the published formula on macOS.

Scoop is also checksum-pinned and installation-tested on Windows:

```powershell
scoop bucket add fxdv https://github.com/fxdv/scoop-bucket
scoop install fxdv/patchlog
patchlog --version
```

## Read-only notes

```bash
patchlog --from v0.1.0 --to HEAD
```

Plain `patchlog` is reporting-only. Optional capabilities use focused
subcommands:

```bash
patchlog ai
patchlog confluence
patchlog metrics
patchlog labs --gamify
```

Direct commit/tag/push remains available only as the explicit
`patchlog release direct` compatibility workflow. Flags never select it
implicitly; for example, a manual protected bump is
`patchlog release prepare --bump patch --dry-run`.

## Documentation

- [Quick start](docs/QUICK_START.md)
- [Configuration and security](docs/CONFIGURATION.md)
- [Advanced workflows](docs/ADVANCED_WORKFLOWS.md)
- [0.2.0 stability contract](docs/STABILITY.md)
- [Product measurements](docs/PRODUCT_METRICS.md)
- [Real-repository evidence](docs/evidence/README.md)
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
