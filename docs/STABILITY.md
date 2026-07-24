# Patchlog 0.2.0 stability contract

Version 0.2.0 is a stability milestone for safe release coordination, not a
feature-count milestone. Until 0.2.0 is published, this document is the target
contract rather than a claim of compatibility.

## Core CLI contract

The supported universal planning entry point is:

```bash
patchlog release --dry-run
```

It auto-detects the protected lifecycle phase. `prepare` applies an approved
fingerprint to an isolated version-bump branch and never tags. `finalize`
applies a separately approved fingerprint only when the local and remote
protected-branch commits match, then pushes only the annotated tag. Dry-run is
strictly immutable. Plain `patchlog` remains reporting-only.

Protected planning selects only reachable stable tags matching
`tag_prefix + MAJOR.MINOR.PATCH`; nightly, deployment-marker, and prerelease
tags cannot steer the stable release phase.

Direct commit/tag/push requires the explicit `release direct` compatibility
workflow. No mutation flag may select direct mode implicitly. A manual protected
bump uses `release prepare --bump LEVEL`; `release --bump LEVEL` is rejected.
AI, Confluence, metrics, and labs use focused subcommands and cannot silently
join the protected transaction.
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

One immutable release plan owns every requested mutation. Its SHA-256
fingerprint covers phase, HEAD, branch identities, versions, tag, actions,
exact file paths, modes, before/after content hashes, and stable required-check
policy evidence. Every mutation requires approval of the exact current
fingerprint. Apply revalidates repository, remote, and provider policy state.
Finalize proves the exact SHA satisfies every required GitHub check and repeats
that verification immediately before the remote tag push.

`--plan-json` exports the
[`release-plan/v1`](schemas/release-plan-v1.schema.json) schema to stdout
during immutable dry-run. Every normal release publishes and attests
`patchlog-release-receipt.json` using the
[`release-receipt/v1`](schemas/release-receipt-v1.schema.json) schema.

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
10. install the Homebrew formula on macOS and the Scoop manifest on Windows.
11. verify the checksummed, attested release receipt.

A release workflow tag is therefore restricted to stable
`vMAJOR.MINOR.PATCH`. A future prerelease channel must define and test its own
resolution semantics instead of weakening the stable `@latest` assertion.

A failed verification leaves the release visible for diagnosis but the release
workflow remains red. Recovery runs verify existing immutable tags without
claiming that an older tag is `@latest`.

## Distribution contract

The live `fxdv/homebrew-tap` and `fxdv/scoop-bucket` channels consume
checksum-pinned manifests rendered from explicit release archives. Artifact
construction stays in the release workflow; package repositories distribute
only manifests. Patchlog independently installs the published Homebrew formula
on macOS and Scoop manifest on Windows.

## Real-repository evidence

Before 0.2.0, record successful immutable dry-run and Apply behavior in at
least three repositories controlled by maintainers unrelated to Patchlog.
Evidence should
include the Patchlog version, repository commit, plan output, exact changed-file
list, resulting tag target, CI result, and any recovery required. Do not include
credentials or proprietary release content.

Patchlog itself and two maintainer-controlled public mirrors provide engineering
evidence, including multi-manifest Python and Rust/Python layouts. They do not
count toward the unrelated-maintainer launch gate. That gate remains 0/3 until
independent repository owners authorize and complete production validations.
Evidence uses the schema in `docs/evidence` and feeds the product measurements
defined in `docs/PRODUCT_METRICS.md`.

Rust virtual workspaces with a root `[workspace]` table and no root
`[package].version` are outside the 0.2.x automatic multi-member bump contract.
Patchlog rejects that layout before mutation instead of guessing which members
share a release. A repository may opt into a single language-neutral `VERSION`
file as described in `CONFIGURATION.md`; coordinated workspace-member manifest
updates remain a future, language-aware feature.

## Explicitly outside the stable core

AI writing, metrics, Jira/Confluence analytics, DPI, health, gamification, and
all `patchlog labs` behavior are optional extensions. Experimental labs have no 0.2.x
compatibility promise and their proxies are never release gates.
