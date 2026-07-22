# Advanced workflows

## Transactional provider release

```bash
patchlog release \
  --bump auto \
  --review \
  --gate \
  --tag \
  --push \
  --publish
```

Execution order is fixed:

1. Analyze and render.
2. Complete interactive review and policy gate.
3. Construct and preflight one immutable release plan.
4. Revalidate `HEAD` and the worktree immediately before Apply.
5. Transactionally bump version files.
6. Commit only planned files through an isolated Git index.
7. Create and atomically push the branch and annotated tag.
8. Verify the remote tag resolves to the local release commit.
9. Create the provider release.

A later failure reports every operation already completed. Remote operations are not presented as globally rollbackable.

## Changelog and Confluence

```bash
patchlog release --bump minor --tag --push --changelog
patchlog release --confluence
patchlog release --confluence --trends
```

Destination-specific credentials and targets are validated during planning. `--trends` requires `--confluence`.

## CI usage

Use reporting mode for pull requests and reserve mutation mode for protected release automation:

```bash
patchlog --from "$BASE_SHA" --to "$HEAD_SHA" --no-cache --out release-notes.md
```

The repository workflows build immutable tag artifacts and publish `SHA256SUMS`. Required checks should protect the default branch; coverage artifacts are informational rather than a numeric release gate.

## Experimental labs

People analytics, individual grading, percentiles, and gamification remain explicitly experimental and require `--labs`. Descriptive proxies are not substitutes for CI coverage, language-aware dependency graphs, or PR/deployment timestamps and are not enforced by the release gate.
