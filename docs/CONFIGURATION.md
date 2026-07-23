# Configuration and security

Patchlog loads `patchlog.yaml` by default. Unknown and misspelled YAML fields fail validation. Keep the file out of version control when it contains credentials; the repository `.gitignore` excludes it.

The safe core release needs no integration configuration. Start with:

```bash
patchlog release --dry-run
```

## Protected release configuration

Protected mode is the default:

```yaml
release:
  protected_branch: main
  branch_prefix: release/
  tag_prefix: v
```

Prepare requires the local protected branch to match its remote before creating
`branch_prefix + tag`. Finalize requires that match again before it can create
and push the annotated tag. Use `patchlog release direct` only for repositories
that deliberately choose direct commit/tag/push.

If a configuration file cannot be decoded or validated, the diagnostic names
its path and tells you to run `patchlog init` or fix the reported field before
retrying the dry run.

### Rust virtual workspaces

A root `Cargo.toml` containing `[workspace]` but no `[package].version` has no
single package version for Patchlog to infer. Patchlog therefore rejects
automatic bump planning before mutation; it does not guess which workspace
members must remain version-aligned.

For a workspace released under one repository version, add a root `VERSION`
file containing the current semantic version and select it explicitly:

```yaml
bump:
  auto_detect: false
  files:
    - VERSION
```

This makes `VERSION` the sole protected release source. Patchlog 0.2.x does not
rewrite member `Cargo.toml` versions or dependency constraints. Repositories
that require coordinated member-manifest updates must keep that coordination
in their existing build tooling and run it before Patchlog planning, or remain
on an explicitly documented custom release workflow.

## Optional provider configuration

```yaml
repo: fxdv/patchlog

provider:
  type: github
  repo: fxdv/patchlog
  token: ${GITHUB_TOKEN}
```

`provider.type`, `provider.repo`, and `provider.token` are preflighted before a
publishing release can mutate state. Provider configuration has no effect on
protected prepare.

## AI disclosure and controls

AI is optional and disabled in the core release path. When a remote AI provider
is explicitly enabled, prompt content derived from release data leaves the
machine for the configured endpoint. Use exclusions and limits to constrain it:

```yaml
ai:
  provider: openai
  api_key: ${OPENAI_API_KEY}
  max_input_chars: 120000
  exclude_files:
    - "**/*.pem"
    - "**/.env*"
    - "vendor/**"
```

Patchlog redacts common credential forms before transmission. Exclusions and redaction are defense in depth; review configuration and generated prompts for highly sensitive repositories.

## Optional trend snapshots

Cross-release analytics storage is disabled by default so the core release does
not create `.patchlog` data:

```yaml
trends:
  store: false
```

Set `store: true` only when trend analytics are wanted. The exact snapshot path
then becomes part of the immutable release plan and is shown by dry-run.

## Transport policy

Credential-bearing endpoints must use HTTPS. Loopback HTTP is allowed for local development. The global insecure override should be used only in isolated development environments:

```yaml
security:
  allow_insecure_credentials: false
```

HTTP clients reject cross-origin redirects so custom credential headers cannot be forwarded to another origin. Response bodies are bounded before decoding.

## Link templates

Commit, issue, and compare templates accept only HTTP(S) URLs:

```yaml
links:
  commit: https://github.com/%s/commit/%s
  issue: https://github.com/%s/issues/%s
  compare: https://github.com/%s/compare/%s...%s
```

For the exhaustive field reference and environment-variable mapping, see [REFERENCE.md](REFERENCE.md#configuration).
