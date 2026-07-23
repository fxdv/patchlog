# Examples

## Safe core release

No integration configuration is required:

```bash
patchlog release --dry-run
patchlog release
```

The first command produces and preflights the immutable plan. The second
revalidates state and applies automatic bump, annotated tag, and atomic push.

## Read-only release notes

```bash
patchlog --from v1.2.0 --to HEAD
```

## Optional provider publication

Configure the provider fields documented in
[`docs/CONFIGURATION.md`](../docs/CONFIGURATION.md), then make every requested
action explicit:

```bash
patchlog release --bump auto --tag --push --publish --dry-run
patchlog release --bump auto --tag --push --publish
```

AI, changelog, Confluence, metrics, and `--labs` options are advanced extensions.
They are intentionally absent from the core example.
