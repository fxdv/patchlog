# Advanced workflows

Everything in this document extends the safe core. Start with
`patchlog release --dry-run`; add integrations only when their output is needed.
AI, provider publishing, changelogs, Confluence analytics, metrics, and labs are
not part of the default release.

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

The repository workflows build immutable tag artifacts, publish `SHA256SUMS`
and provenance, then download and verify the public release. A normal stable
release must also pass fresh `go install` checks for both its exact tag and
`@latest`. Required checks should protect the default branch; coverage artifacts
are informational rather than a numeric release gate.

## Experimental labs

`--labs` is an explicit experimental boundary. DPI, health signals, individual
analytics, grades, percentiles, and gamification may change or disappear without
the core compatibility guarantees.

The descriptive proxies exposed by labs are diagnostic only:

- `TOUCHED_TEST_FILE_RATIO`
- `CHANGE_COMPLEXITY_PROXY`
- `CROSS_CUTTING_CHANGE_RISK`
- `RELEASE_CONTRIBUTION_CONCENTRATION`
- `RELEASE_COMMIT_SPAN_HOURS`

They are not release gates and are not presented as true coverage, dependency
risk, bus factor, or lead/cycle time. Those require CI coverage artifacts,
language-aware dependency graphs, and PR/deployment timestamps.
