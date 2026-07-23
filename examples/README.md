# Examples

## Safe core release

No integration configuration is required:

```bash
patchlog release --dry-run
patchlog release prepare --approve sha256:<fingerprint>
# Open and merge the green PR, then wait for green post-merge CI.
patchlog release --dry-run
patchlog release finalize --approve sha256:<fingerprint>
```

The universal planner detects prepare or finalize. Each mutation requires the
exact fingerprint produced for that phase.

## Read-only release notes

```bash
patchlog --from v1.2.0 --to HEAD
```

## Optional provider publication

Configure the provider fields documented in
[`docs/CONFIGURATION.md`](../docs/CONFIGURATION.md), then make every requested
action explicit:

```bash
patchlog release direct --bump auto --tag --push --publish --dry-run
patchlog release direct --bump auto --tag --push --publish --approve sha256:<fingerprint>
```

AI, Confluence, metrics, and labs use `patchlog ai`, `patchlog confluence`,
`patchlog metrics`, and `patchlog labs`. They cannot silently join the protected
release transaction.
