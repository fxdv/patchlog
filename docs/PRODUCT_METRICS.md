# Product measurements

Patchlog measures whether protected releases become safer and easier. These are
workflow outcomes, not code-quality proxies, individual grades, or release
gates.

## Published baseline

The machine-readable baseline is
[`docs/evidence/metrics.json`](evidence/metrics.json), regenerated with:

```bash
python3 scripts/product-metrics.py
```

As of the current evidence set, the two maintainer-controlled hosted workflows
show:

- median time to first successful plan: 322.5 seconds; p90: 335 seconds;
- plan-to-release success: 2/2 (100%);
- classified preflight rejections across all five validation attempts:
  `version_detection` 1;
- releases requiring recovery: 0/2 (0%), with zero recovery executions;
- releases without manual Git intervention: 1/2 (50%).

These are controlled workflow measurements, not adoption evidence. Validation
in repositories controlled by unrelated maintainers remains 0/3 and is an
explicit launch gate for 0.2.0.

## Metrics

### Time to first successful plan

Elapsed time from the start of an onboarding validation session to the first
successful `patchlog release --dry-run` that produces a fingerprint.

### Plan-to-release success rate

Number of approved protected plans that reach a verified published release,
divided by protected plans approved for mutation.

### Preflight rejection reasons

Count deterministic rejection categories such as dirty worktree, stale
protected branch, occupied release branch, stale fingerprint, version/tag
mismatch, remote tag collision, version detection, and missing configuration.
The CLI emits a stable `Preflight rejection: <category>` line alongside the
human diagnostic. Store the category, not repository contents or error
payloads containing secrets.

### Recovery frequency

Number of release attempts that required at least one guarded recovery divided
by release publication attempts. The separate recovery-execution count
preserves repeated recovery work. Keep the original failed run in the
denominator: recovery is evidence of a failure mode, not a way to erase it.

### Releases without manual Git intervention

Percentage of successful releases where Patchlog performed every Git mutation
after fingerprint approval. Opening or merging a pull request through the
hosting provider is orchestration, not manual Git intervention; handwritten
`git commit`, `git tag`, or `git push` commands make this value false.

## Collection boundary

Patchlog does not send usage telemetry and immutable dry-run never writes a
measurement file or contacts a measurement service. Validation sessions record
timestamps and outcome categories in an explicit evidence artifact outside the
dry-run process. CI may upload that artifact when the repository owner opts in.

Aggregate only repository-level workflow outcomes. Do not collect commit
contents, contributor rankings, credentials, source diffs, or individual
performance data.

## Evidence rollup

For an evidence set, compute:

- median and p90 time to first successful plan;
- successful verified releases / approved plans;
- rejection counts grouped by the stable category;
- releases requiring recovery / publication attempts, plus total recovery
  executions;
- successful releases with `manual_git_intervention: false` / all successful
  releases.

Keep local compatibility simulations separate from hosted CI evidence. They
exercise orchestration but do not enter launch-readiness success rates.
