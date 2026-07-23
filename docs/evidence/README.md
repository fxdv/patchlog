# Real-repository validation evidence

Patchlog 0.2.0 requires successful protected prepare/finalize evidence from
Patchlog itself and two additional repositories with different layouts or
release histories.

Create one record from `validation.example.json` per repository. Replace every
placeholder, remove proprietary repository identifiers when necessary, and
confirm that the record contains no credentials, source content, or contributor
performance data.

Required evidence:

- Patchlog version and repository commit;
- validation start and first successful plan timestamps;
- prepare and finalize fingerprints;
- exact version-file list;
- prepared branch and merged protected-branch commit;
- required CI result;
- tag target and release verification result;
- classified preflight rejections and recoveries;
- whether manual Git intervention was required.

## Validation levels

`external-source-local-bare-remote` records prove layout compatibility,
fingerprint stability, exact-file prepare, squash-merge handoff, annotated-tag
finalize, and remote tag targeting without touching the source repository's
production remote. They do not prove hosted branch protection or CI.

Before 0.2.0, repeat both successful records against hosted test repositories
or explicitly approved production repositories with required CI. Only records
with `quality_result: success` count as full launch evidence. Preserve failed
preflights as compatibility evidence instead of silently selecting an easier
repository.
