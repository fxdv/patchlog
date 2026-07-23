# Patchlog 0.2.0 stability contract

Version 0.2.0 is a stability milestone for safe release coordination, not a
feature-count milestone. Until 0.2.0 is published, this document is the target
contract rather than a claim of compatibility.

## Core CLI contract

The supported golden path is:

```bash
patchlog release --dry-run
patchlog release
```

With no explicit release-action flags, both commands select automatic version
bumping, annotated tagging, and atomic branch/tag push. Dry-run is strictly
immutable. Plain `patchlog` remains reporting-only.

Explicit release-action flags select a composed advanced workflow; Patchlog
does not silently add publishing, Confluence, changelog, metrics, AI, or labs.
Exit-code meanings and documented core flags will remain compatible throughout
the 0.2.x line.

## Configuration contract

The documented YAML schema is strict: unknown fields fail decoding, every
documented field is wired, and security-sensitive defaults remain restrictive.
Additive optional fields may appear in 0.2.x. Removing or changing the meaning
of a field waits for a versioned migration.

Configuration errors must name the file and invalid or missing fields, then
provide a concrete recovery command.

## Deterministic plan/apply contract

One immutable release plan owns every requested mutation. Planning and all
gates complete before Apply. Apply revalidates repository state, bumps only the
plan's exact file list transactionally, commits through an isolated index, and
uses an immutable remote release ref. The same repository state and options
must produce the same plan.

## Release trust loop

Every normal tag-triggered release must automatically:

1. run the required quality gate;
2. prove `tag == "v" + VERSION` and tag target equals the checkout;
3. build explicit platform archives and `SHA256SUMS`;
4. attach signed provenance attestations;
5. publish the GitHub release;
6. download and validate every checksum;
7. verify archive provenance against the release workflow and tag commit;
8. execute an archive and verify its reported version;
9. install `@version` and stable `@latest` from a fresh Go environment.

A release workflow tag is therefore restricted to stable
`vMAJOR.MINOR.PATCH`. A future prerelease channel must define and test its own
resolution semantics instead of weakening the stable `@latest` assertion.

A failed verification leaves the release visible for diagnosis but the release
workflow remains red. Recovery runs verify existing immutable tags without
claiming that an older tag is `@latest`.

## Distribution contract

0.2.0 requires at least one package-manager installation channel with an
automatically checksum-pinned manifest and an installation check in the trust
loop. Homebrew is first; Scoop follows. Generated manifests in GitHub release
assets are preparation, not a live channel by themselves.

## Real-repository evidence

Before 0.2.0, record successful immutable dry-run and Apply behavior in at least
three repositories that differ in layout or release history. Evidence should
include the Patchlog version, repository commit, plan output, exact changed-file
list, resulting tag target, CI result, and any recovery required. Do not include
credentials or proprietary release content.

Patchlog itself is the dogfood repository. Two additional repositories are
still required before declaring this criterion complete.

## Explicitly outside the stable core

AI writing, metrics, Jira/Confluence analytics, DPI, health, gamification, and
all `--labs` behavior are optional extensions. Experimental labs have no 0.2.x
compatibility promise and their proxies are never release gates.
