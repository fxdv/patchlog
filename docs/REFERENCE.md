# Patchlog complete reference

Safely coordinate deterministic version bumps, annotated tags, and atomic
pushes from Git history. Release notes are built in; AI, Jira, Confluence,
provider publishing, metrics, and labs are optional extensions.

## Table of Contents

- [Quick Start](#quick-start)
- [Installation](#installation)
- [How It Works](#how-it-works)
- [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Full Config Reference](#full-config-reference)
  - [Config Validation](#config-validation)
- [Command Reference](#command-reference)
  - [Global Flags](#global-flags)
  - [Subcommands](#subcommands)
- [AI Providers](#ai-providers)
- [Version Bumping](#version-bumping)
- [Significance Classification](#significance-classification)
- [Changelog Accumulation](#changelog-accumulation)
- [Jira Integration](#jira-integration)
- [Confluence Integration](#confluence-integration)
- [Git Provider Integration](#git-provider-integration)
- [Commit Linting](#commit-linting)
- [Changelog Audit](#changelog-audit)
- [Multi-Repo Aggregation](#multi-repo-aggregation)
- [Metrics & Code Stats](#metrics--code-stats)
- [Caching](#caching)
- [Retry & Resilience](#retry--resilience)
- [GitLab CI Integration](#gitlab-ci-integration)
- [Testing](#testing)
- [Architecture](#architecture)
- [Exit Codes](#exit-codes)
- [License](#license)

## Quick Start

```bash
# Build from source
git clone https://github.com/fxdv/patchlog.git
cd patchlog
make build
# Binary is at ./patchlog

# Start from the credential-safe example. patchlog.yaml is gitignored.
cp patchlog.example.yaml patchlog.yaml

# Interactive setup (creates patchlog.yaml with permissions 0600)
./patchlog init

# Generate release notes from the last tag to HEAD
patchlog

# Plan the safe core release without changing local or remote state
patchlog release --dry-run

# Apply the reviewed automatic bump, annotated tag, and atomic push
patchlog release
```

## Installation

### Go toolchain

```bash
go install github.com/fxdv/patchlog/cmd/patchlog@latest
```

### From source

```bash
git clone https://github.com/fxdv/patchlog.git
cd patchlog
make build
# Binary is at ./patchlog
```

Tagged releases contain platform archives and `SHA256SUMS`. Download both from the same immutable tag, run `sha256sum --check SHA256SUMS`, extract the matching archive, and verify `patchlog --version` before installation.

### Makefile

```bash
make build          # Build with version injection from git tags
```

The build injects the git version via `-ldflags "-X main.version=$(VERSION)"`, falling back to `"dev"` when no tags exist.

### Requirements

- Go 1.22+
- Git (accessible in PATH)
- Optional: Ollama running locally, or an OpenAI/Anthropic API key

## How It Works

```
git history → report + exact release plan → review → grounded gate
                                                     │
                                                     ▼
                            transactional bump → commit/tag/atomic push
                                                     │
                                                     ▼
                                  provider → Confluence → changelog/output
```

1. **Fetch** commits in the range `[from..to]` using `git log` (merge commits excluded)
2. **Parse** each commit for conventional-commit metadata (type, scope, breaking, body)
3. **Classify** significance using diff stats (files changed, insertions/deletions, file categories)
4. **Categorize** into sections (Features, Bug Fixes, etc.) based on commit type
5. **Enrich** with Jira issue details (summary, priority, status, components, fix versions)
6. **Render** as markdown, JSON, or AI-generated prose
7. **Plan** exact local and remote mutations without changing state
8. **Review and gate** the complete output before any mutation
9. **Apply** the transactional bump, exact-file Git operations, and requested publications

## Configuration

patchlog reads from `patchlog.yaml` by default. Use `--config` or `PATCHLOG_CONFIG` to select a different path. Unknown YAML fields are rejected, so misspellings cannot silently disable security or release settings. If no config file exists, sensible defaults are used.

### Minimal Config

```yaml
ai:
  provider: ollama
  model: llama3.2
```

### Environment Variables

Secrets in config can reference environment variables. The expansion rules:

| Syntax | Behavior | Example |
|--------|----------|---------|
| `$VAR` | Replaced with value of `VAR` env var | `$GITHUB_TOKEN` |
| `${VAR}` | Same as `$VAR` (braced form) | `${OPENAI_API_KEY}` |
| `${VAR:-default}` | Use `VAR` if set and non-empty, otherwise `default` | `${JIRA_TOKEN:-fallback}` |
| `$$VAR` | Literal `$VAR` (escape for values that start with `$`) | `$$HOME/bin` |

Expansion applies to all credential fields: `ai.api_key`, `provider.token`, `jira.api_token`, `confluence.api_token`.

> **Note:** Only values that are *entirely* a variable reference are expanded. Embedded variables like `https://$HOST/path` are treated as literal strings.

### Full Config Reference

```yaml
# ─── Sections ──────────────────────────────────────────────
# Maps conventional commit types to display headings.
# Defaults are shown below; override only the ones you want to change.
sections:
  feat: Features
  fix: Bug Fixes
  perf: Performance Improvements
  refactor: Code Refactoring
  docs: Documentation
  test: Tests
  style: Style / Formatting
  ci: CI / Build
  chore: Chores

# ─── Ignore ────────────────────────────────────────────────
# Regex patterns to skip matching commit messages.
# Patterns are OR'd together into a single regex.
ignore:
  - "^Merge"
  - "^chore\\(deps\\)"

# ─── Author ────────────────────────────────────────────────
author:
  show: true          # Show "@author" in output

# ─── Links ─────────────────────────────────────────────────
# URL templates with %s placeholders.
# %s order: commit/issue → (repo, ref); compare → (repo, from, to)
links:
  commit: "https://github.com/%s/commit/%s"
  issue: "https://github.com/%s/issues/%s"
  compare: "https://github.com/%s/compare/%s...%s"

# Repository in owner/name format (used for link generation)
repo: myorg/myrepo

# ─── AI ────────────────────────────────────────────────────
ai:
  provider: ollama          # ollama | openai | anthropic
  model: llama3.2           # provider-specific default if empty
  api_key: $OPENAI_API_KEY  # required for openai/anthropic
  base_url: ""              # override API endpoint
  max_tokens: 4096          # 0 = provider default
  max_input_chars: 120000   # hard outbound prompt limit
  exclude_files:            # omitted from AI diff input
    - .env.*
    - "*.pem"
    - "**/generated/**"

# ─── Provider (git host) ───────────────────────────────────
provider:
  type: github              # github | gitlab | gitea
  token: $GITHUB_TOKEN
  repo: myorg/myrepo
  base_url: ""              # required for gitea; optional for others
  draft: true               # GitHub/Gitea: create as draft (default true)

security:
  allow_insecure_credentials: false  # HTTPS required except loopback

# ─── Jira ──────────────────────────────────────────────────
jira:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  project_key: PROJ         # optional: filter to this project
  max_concurrency: 5        # parallel API calls (default 5)

# ─── Confluence ────────────────────────────────────────────
confluence:
  base_url: https://myorg.atlassian.net    # defaults to jira.base_url
  email: dev@myorg.com                      # defaults to jira.email
  api_token: $JIRA_API_TOKEN                # defaults to jira.api_token
  space_key: ENG                            # required
  parent_page_id: "123456"                  # optional: publish under this page

# ─── Bump ──────────────────────────────────────────────────
bump:
  auto_detect: true         # auto-detect version file type
  files: []                 # extra files to bump (plain text version)

# ─── Classify ──────────────────────────────────────────────
# File-count thresholds for significance classification.
classify:
  large_feature_files: 5    # feat with >= N files → major
  large_fix_files: 5        # fix with >= N files → minor
  large_unknown_files: 3    # unknown type with >= N files → minor

# ─── Changelog ─────────────────────────────────────────────
changelog:
  accumulate: false         # enable changelog accumulation
  destination: md           # md | wiki | confluence
  file: CHANGELOG.md        # for md destination
  title: Changelog          # for wiki/confluence destination
  slug: changelog           # for wiki destination
  emojis: true              # use emoji icons in section headings
```

### Config Validation

patchlog validates the config on load and reports all errors before proceeding. Validation checks:

- Unknown or misspelled YAML fields fail strict decoding

- AI provider is one of `ollama`, `openai`, `anthropic`
- API key present when required (openai, anthropic)
- `max_tokens` is non-negative
- Provider type is `github`, `gitlab`, or `gitea`
- `base_url` is required for gitea
- `provider.repo` is required when provider type is set
- All URLs start with `http://` or `https://` and have a host
- Credentials are never sent over non-loopback HTTP unless explicitly overridden
- Jira credentials present when `jira.base_url` is set
- `max_concurrency` is non-negative
- Confluence credentials present (or fall back to Jira)
- Changelog destination is `md`, `wiki`, or `confluence`
- Wiki destination requires `gitlab` provider
- All ignore patterns are valid regex
- All classify thresholds are non-negative
- Section keys and headings are non-empty
- `deps.max_dependencies` is non-negative
- All `deps.registries` URLs are valid

## Command Reference

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | latest tag | Start git ref (use `--first` for root commit) |
| `--to` | `HEAD` | End git ref |
| `--config` | `patchlog.yaml` | Path to config file |
| `--repo` | `.` | Path to git repository |
| `--out` | stdout | Write output to file instead of stdout |
| `--format` | `markdown` | Output format: `markdown`, `json`, `prose` |
| `--tone` | `dev` | Prose tone (only with `--format prose`): `dev`, `customer`, `exec` |
| `--filter` | | Monorepo path filter (e.g., `pkg/api`) — only includes commits touching this path |
| `--first` | `false` | Start from the very first commit (no previous tag needed) |
| `--dry-run` | `false` | Build and validate the complete plan without changing filesystem, Git, cache, or remotes |
| `--classify` | `true` | Enable diff-aware significance classification |
| `--review` | `false` | Write output to a temp file for manual editing before outputting |
| `--quiet` | `false` | Suppress banner and spinner output |
| `--no-cache` | `false` | Bypass file cache (for CI reproducibility) |
| `--metrics` | `false` | Append release metrics and code stats to output (markdown only) |
| `--ai-enhance` | `false` | Use AI to enhance item descriptions and prepend a summary |
| `--deps` | `false` | Detect dependency version bumps and fetch upstream changelogs |
| `--labs` | `false` | Enable experimental DPI, health signals, people analytics, and gamification |
| `--version` | `false` | Print version and exit |

### Examples

```bash
# Basic markdown output
patchlog

# AI-generated prose, streaming tokens to terminal
patchlog --format prose --tone dev

# Customer-facing release notes
patchlog --format prose --tone customer

# Executive one-paragraph summary
patchlog --format prose --tone exec

# Specific commit range
patchlog --from v1.2.0 --to v1.3.0

# First release (from root commit)
patchlog --first

# Dry run the safe default bump, tag, and atomic push without mutation
patchlog release --dry-run

# Monorepo — only changes under a path
patchlog --filter pkg/api

# JSON output for piping
patchlog --format json --out release.json

# Quiet mode for scripts
patchlog --quiet --format json

# Bypass cache for CI
patchlog --no-cache --format markdown

# Include metrics
patchlog --metrics

# AI-enhanced descriptions + summary
patchlog --ai-enhance

# Detect dependency version bumps and fetch upstream changelogs
patchlog --deps

# Review before outputting
patchlog --review --out RELEASE_NOTES.md

# Full pipeline
patchlog release --bump auto --tag --push --format prose --tone dev --publish --confluence --changelog
```

### Subcommands

#### `patchlog release`

The only command that accepts release mutations. It builds the complete
immutable plan, completes review and the grounded gate, then enters the apply
phase. With no explicit release-action flags, it defaults to `--bump auto --tag
--push`:

```bash
patchlog release --dry-run
patchlog release
```

Supplying any release-action flag selects a composed workflow instead of adding
the safe defaults. For example, `patchlog release --bump minor` performs a
bump-only release; provider publication must explicitly include
`--bump ... --tag --push --publish`.

| Flag | Default | Description |
|------|---------|-------------|
| `--bump` | `auto` in the golden path | Plan a `patch`, `minor`, `major`, or `auto` version bump |
| `--tag` | `true` in the golden path | Commit exactly the planned bump files and create an annotated tag |
| `--push` | `true` in the golden path | Atomically push the branch and tag (requires `--tag`) |
| `--force` | `false` | Explicitly override dirty-tree or existing-tag safeguards |
| `--publish` | `false` | Create a provider release after local Git operations succeed |
| `--confluence` | `false` | Publish or update a Confluence page |
| `--changelog` | `false` | Accumulate the release into the configured changelog destination |
| `--trends` | `false` | Publish the trends dashboard (requires `--confluence`) |

The reporting command rejects these mutation flags and does not store trend snapshots. `changelog.accumulate: true` is likewise honored only by `patchlog release`.

#### `patchlog init`

Interactive setup wizard. Walks through configuring AI provider, git host, Jira, Confluence, and changelog settings. Writes `patchlog.yaml` with file permissions `0600`.

```bash
patchlog init
```

#### `patchlog lint`

Lint commits against conventional-commit standards. Optionally uses AI to suggest improved messages.

```bash
# Lint since the last tag
patchlog lint

# Lint a specific range
patchlog lint --from v1.0.0 --to HEAD

# Use AI to suggest improvements
patchlog lint --ai
```

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | latest tag | Start ref |
| `--to` | `HEAD` | End ref |
| `--repo` | `.` | Path to git repository |
| `--config` | `patchlog.yaml` | Config file path (for AI settings) |
| `--ai` | `false` | Use AI to suggest improved commit messages |

The linter checks for:
- Conventional commit format compliance (`type(scope): description`)
- Valid commit types (`feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `style`, `ci`, `chore`, `build`, `revert`)
- Description quality (length >= 10 chars, capitalization, no trailing period)
- Imperative mood (checked against a verb allowlist + past-tense heuristic)
- Missing body for `feat`/`fix`/`refactor` commits
- Breaking changes without explanation
- `fix` commits without Jira references

Exit code 1 if any errors are found. Info-level issues (warnings) do not affect exit code.

#### `patchlog audit`

Audit a changelog file against git history to find missing or stale entries.

```bash
# Audit CHANGELOG.md against git history
patchlog audit

# Audit a specific file
patchlog audit --changelog docs/RELEASE_NOTES.md

# Audit a specific range
patchlog audit --from v1.0.0 --to HEAD
```

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | latest tag | Start ref |
| `--to` | `HEAD` | End ref |
| `--repo` | `.` | Path to git repository |
| `--changelog` | `CHANGELOG.md` in repo | Path to changelog file |

The audit:
- Fetches commits in the range and parses them
- Skips `docs`/`test`/`style`/`ci`/`chore` types (not expected in changelogs)
- Matches commits against changelog content using fuzzy matching (header prefix, keywords, Jira keys, hash prefix, PR refs)
- Detects placeholder entries (`todo`, `tbd`, `fixme`, `placeholder`, `coming soon`, `n/a`)

Exit code 1 if issues are found.

#### `patchlog multi`

Aggregate changelogs from multiple repositories into a single document.

```bash
# Aggregate from multiple repos
patchlog multi ./services/api ./services/auth ./frontend

# With a path filter
patchlog multi ./services/api ./services/auth --filter pkg/api

# Write to file
patchlog multi ./repo-a ./repo-b --out AGGREGATED_CHANGELOG.md

# JSON format
patchlog multi ./repo-a ./repo-b --format json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | latest tag | Start ref |
| `--to` | `HEAD` | End ref |
| `--config` | `patchlog.yaml` | Config file path |
| `--filter` | | Monorepo path filter |
| `--format` | `markdown` | Output format: `markdown`, `json` |
| `--out` | stdout | Output file |

Each repository's changes are grouped under its own heading. Repos are processed sequentially with classification enabled by default.

#### `patchlog recover`

Re-render markdown from a saved JSON report.

```bash
patchlog recover release-report.json
```

Useful when you saved a JSON output with `--format json` and want to generate markdown from it later.

#### `patchlog cache clear`

Clear the file cache.

```bash
patchlog cache clear
# Or with a specific repo path
patchlog cache clear --repo /path/to/repo
```

Removes the `.patchlog/cache` directory in the repo.

## AI Providers

patchlog supports three AI providers for prose generation and description enhancement.

When an AI feature uses a non-loopback endpoint, the CLI discloses the endpoint and data boundary before the first request. Depending on the feature, commit messages, selected code diffs, linked issue text, and release metrics may leave the machine. All prompt paths share secret redaction and `ai.max_input_chars`; files matching `ai.exclude_files` are removed before diff assembly. Review these controls for private repositories—redaction is defense in depth, not a substitute for endpoint trust.

### Ollama (default, local, free)

```yaml
ai:
  provider: ollama
  model: llama3.2           # any model pulled via `ollama pull`
  base_url: http://localhost:11434  # default
```

No API key required. Requires [Ollama](https://ollama.ai) running locally.

### OpenAI

```yaml
ai:
  provider: openai
  model: gpt-4o-mini        # default if empty
  api_key: $OPENAI_API_KEY  # required
  base_url: https://api.openai.com/v1  # override for Azure/etc.
  max_tokens: 4096
```

### Anthropic

```yaml
ai:
  provider: anthropic
  model: claude-3-haiku-20240307  # default if empty
  api_key: $ANTHROPIC_API_KEY     # required
  base_url: https://api.anthropic.com/v1
  max_tokens: 4096          # required by Anthropic API, default 4096
```

### Streaming & Chunking

When using `--format prose`, patchlog streams tokens to stderr in real-time. For large releases, the prompt is automatically chunked:

- If the prompt exceeds 4,000 characters, it's split into section-based chunks
- Each chunk is sent as a separate request with part numbering (`Part 1 of 3`)
- Breaking changes are always included in the first chunk
- Large sections are split by item batches
- Results are stitched together

If all AI calls fail, a template fallback is used so output is never empty.

### AI Enhancement (`--ai-enhance`)

When enabled, patchlog:
1. Sends each section's items to the AI for rewriting (more descriptive, user-friendly)
2. Generates a 2-3 sentence summary prepended to the output as a blockquote

If all sections fail AI enhancement, a warning is printed to stderr. Partial failures degrade gracefully (failed sections keep original descriptions).

## Version Bumping

```bash
# Auto-detect bump level from commit significance
patchlog release --bump auto

# Manual bump
patchlog release --bump patch
patchlog release --bump minor
patchlog release --bump major
```

### Auto-bump logic

`--bump auto` scans all commits and picks the highest significance level:

| Commit type | Breaking | Changed files | Significance |
|-------------|----------|---------------|-------------|
| `feat!` / `BREAKING CHANGE` | yes | any | **major** |
| `feat` (security-related) | no | any | **major** |
| `feat` (public API change) | no | any | **major** |
| `feat` | no | >= `large_feature_files` (5) or > 500 lines | **major** |
| `feat` | no | < threshold | **minor** |
| `fix` (security-related) | no | any | **major** |
| `fix` | no | >= `large_fix_files` (5) or > 300 lines | **minor** |
| `fix` | no | < threshold | **patch** |
| `perf` | no | > 200 lines or >= threshold | **major** |
| `perf` | no | < threshold | **minor** |
| `refactor` (deletion-heavy) | no | > 60% deletions, > 100 lines | **major** |
| `refactor` | no | > 500 lines or >= threshold | **minor** |
| `refactor` | no | < threshold | **patch** |
| `revert` | no | any | **patch** |
| `docs`, `test`, `style`, `ci` | no | any | **skip** |
| `chore` (deployment) | no | core files changed | **patch** |
| `chore` | no | any | **skip** |
| unknown (security) | no | any | **major** |
| unknown (public API) | no | any | **major** |
| unknown | no | >= `large_unknown_files` (3) or > 300 lines | **minor** |
| unknown | no | < threshold | **patch** |

Auto-bump prints a color-coded summary:

```
  1x major  3x minor  7x patch
  → Auto bump (major) → 2.0.0
```

### Version file detection

patchlog auto-detects and bumps version in:

| File | Format | Detection |
|------|--------|-----------|
| `package.json` | `"version": "1.2.3"` | NPM version field |
| `Cargo.toml` | `version = "1.2.3"` | TOML version key |
| `pyproject.toml` | `version = "1.2.3"` | TOML version key |
| `VERSION` / `version.txt` / `version` | `1.2.3` | Plain text |

Additional files can be specified via `bump.files` in config or are detected as plain-text version files.

Pre-release suffixes are preserved: `1.2.3-beta.1` → minor bump → `1.3.0` (suffix dropped on bump).

## Significance Classification

When `--classify` is enabled (default), patchlog analyzes each commit's diff to determine its significance. This uses `git show --numstat` to get file counts, insertions, deletions, and file paths.

### File categorization

Files are categorized by path patterns:

| Category | Matches |
|----------|---------|
| **Public API** | `openapi.yaml/json`, `swagger.*`, `.proto`, `.graphql`, `/api/v1/`, `/api/v2/`, `/public/api/` |
| **Security** | `auth`, `crypto`, `security`, `password`, `session`, `token`, `permission`, `rbac`, `acl`, `secret`, `.pem`, `.key`, `.crt` |
| **Migration** | `migration`, `migrations`, `.sql` + `migration`/`schema` |
| **Deployment** | `dockerfile`, `docker-compose`, `/k8s/`, `/kubernetes/`, `/helm/`, `/charts/`, `/terraform/`, `.tf`, `/.github/workflows/`, `/.gitlab-ci.`, `makefile` |
| **Test** | `_test.go`, `_test.py`, `.test.js/ts`, `.spec.js/ts`, `/test/`, `/tests/`, `/__tests__/`, `/testdata/` |
| **Docs** | `.md`, `.rst`, `/docs/`, `/doc/` |
| **Config** | `.json` + `config`/`settings`, `.yaml`, `.yml`, `.toml`, `.ini`, `.env` |
| **Generated** | `.generated.`, `.gen.`, `.pb.go`, `.pb.py`, `/generated/`, `/vendor/`, `/third_party/` |
| **Lockfile** | `.lock`, `package-lock.json`, `yarn.lock`, `go.sum`, `cargo.lock`, `poetry.lock`, `composer.lock`, `pnpm-lock.yaml`, `gemfile.lock` |
| **API** | `/api/`, `/handler`, `/controller`, `/route`, `/endpoint`, `/openapi` |

Generated files and lockfiles are excluded from "real changed" counts. Tests and docs are excluded from "core changed" counts.

### Thresholds

Configurable via `classify` section:

```yaml
classify:
  large_feature_files: 5    # feat with >= N core files → major
  large_fix_files: 5        # fix with >= N core files → minor
  large_unknown_files: 3    # unknown type with >= N core files → minor
```

## Dependency Changelog

When `--deps` is passed, patchlog detects dependency version bumps in manifest files between the commit range and optionally fetches upstream changelogs.

### Detection

patchlog parses the git diff of changed manifest files to extract version changes:

| Manifest File | Ecosystem | Registry |
|---------------|-----------|----------|
| `package.json` | npm | registry.npmjs.org |
| `Cargo.toml` | cargo | crates.io |
| `go.mod` | go | pkg.go.dev |
| `pyproject.toml` | pypi | pypi.org |
| `requirements.txt` | pypi | pypi.org |

For each detected change, patchlog extracts the package name, old version, and new version from the diff lines.

### Upstream Changelog Fetching

When `fetch_upstream` is enabled (default), patchlog fetches upstream changelogs:

- **npm/crates/pypi**: fetches package metadata from the registry, then follows GitHub repository links
- **GitHub releases**: fetches release notes between the old and new version tags via the GitHub API (unauthenticated, rate-limited)
- **Go modules**: for `github.com/...` packages, fetches GitHub releases directly

If fetching fails, the dependency change is still listed with a link to the package page.

### Output

Dependencies appear as a `## Dependencies` section in markdown output:

```markdown
## Dependencies

### react ^18.2.0 → ^18.3.1

_ecosystem: npm · manifest: package.json_

<details>
<summary>Upstream changes</summary>

**v18.3.1**
Removed legacy defaultProps for function components...
</details>
```

In Confluence, dependencies render as an expand macro with a table and nested expand macros for upstream changelogs.

### Config

```yaml
deps:
  enabled: false              # can also be toggled via --deps
  fetch_upstream: true        # fetch upstream changelogs (false = list only)
  max_dependencies: 10        # don't fetch more than N upstream changelogs
  github_releases: true       # use GitHub releases API for GH-hosted packages
  registries:
    npm: https://registry.npmjs.org
    crates: https://crates.io
    pypi: https://pypi.org
```

## Changelog Accumulation

patchlog can accumulate each release into a persistent changelog. Enable via `--changelog` flag or `changelog.accumulate: true` in config.

### Destinations

#### Markdown file (`md`)

Prepends the new release section to the top of `CHANGELOG.md`, preserving the existing header and content:

```yaml
changelog:
  accumulate: true
  destination: md
  file: CHANGELOG.md
  emojis: true
```

```bash
patchlog release --changelog
# Or combine with other flags
patchlog release --bump auto --changelog
```

The accumulated markdown uses `###` headings (nested under `# Changelog`) with version sections separated by `---`.

#### GitLab wiki (`wiki`)

Creates or updates a wiki page on GitLab. Requires `provider.type: gitlab` with a token.

```yaml
changelog:
  accumulate: true
  destination: wiki
  title: Changelog
  slug: changelog

provider:
  type: gitlab
  token: $GITLAB_TOKEN
  repo: myorg/myrepo
```

The new section is prepended to the existing wiki page content with a `---` separator.

#### Confluence (`confluence`)

Creates or updates a Confluence page. Uses the same credentials as the `confluence` config section (falls back to Jira).

```yaml
changelog:
  accumulate: true
  destination: confluence
  title: Changelog

confluence:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  space_key: ENG
```

### Emoji headings

When `changelog.emojis` is `true` (default), section headings include emoji:

| Type | Emoji |
|------|-------|
| feat | ✨ |
| fix | 🐛 |
| perf | ⚡ |
| refactor | 🔨 |
| docs | 📝 |
| test | 🧪 |
| style | 🎨 |
| ci | 👷 |
| chore | 🔧 |
| Breaking | ⚠️ |

## Jira Integration

When Jira is configured, patchlog:

1. **Extracts** ticket keys from commit messages and bodies (`PROJ-123`, `ENG-456`, etc.) using regex `[A-Z][A-Z0-9]+-\d+`
2. **Filters** by project key if `jira.project_key` is set
3. **Fetches** issue details concurrently (configurable via `jira.max_concurrency`, default 5):
   - Summary, priority, status, issue type
   - Labels, epic key, fix versions, components
   - Description (truncated to 500 chars), assignee
4. **Enriches** each changelog entry with linked ticket info
5. **Caches** fetched issues to `.patchlog/cache/jira/` (24h TTL) to avoid redundant API calls on re-runs

In markdown output:

```
- add pagination to user list endpoint (#42) [PROJ-123](https://myorg.atlassian.net/browse/PROJ-123) Add cursor-based pagination `Open` (api) → v2.1 by @alice
```

In AI-generated prose, Jira context (summary, priority, status, type, epic, components, fix versions, description) is included in the prompt so the model can reference it.

### Concurrency

Jira issues are fetched in parallel with a bounded worker pool. The `max_concurrency` setting controls the number of simultaneous HTTP requests:

```yaml
jira:
  max_concurrency: 10  # default: 5
```

## Confluence Integration

When `--confluence` is passed, patchlog publishes a rich page to Confluence Cloud:

- **Create or update**: if a page with the same title exists, it's updated in-place (versioned). Otherwise a new page is created.
- **Under a parent**: set `parent_page_id` to publish under a specific page tree.
- **Reuses Jira auth**: if `confluence.base_url` is not set, falls back to `jira.base_url` with the same credentials.
- **Labels**: add `labels` to automatically tag pages for searchability and dashboard filtering.
- **Page restrictions**: restrict view/edit access to specific users.
- **Configurable layout**: provide a custom Go template file to control page section ordering.

The Confluence page includes:

- **Page Properties macro** (`details`) for release dashboard aggregation
- **Table of Contents** for instant navigation
- **Prev/Next release navigation** links to sibling release note pages
- **Formatted release notes** with proper headings, lists, and author attribution
- **Commit hyperlinks** — issue refs and commit hashes link to the git provider
- **Jira ticket links** as clickable links with priority status macros (color-coded: red for critical, orange for high, etc.)
- **Jira Epic panel** grouping issues by epic with counts
- **Breaking change banner** using Confluence info macro
- **Expand macros** for sections with more than 8 items (keeps the page scannable)
- **Analytics panel** (always rendered when metrics are available, not just with `--ai-enhance`):
  - **Visual risk gauge** — HTML progress bar for Release Risk Score
  - **Stacked significance bar** — visual distribution of major/minor/patch
  - Code metrics table with AI interpretations (when `--ai-enhance`)
  - Top contributors table with medals
  - Release overview (commits, authors, lines, files)
  - Impact distribution by type and significance

```yaml
confluence:
  base_url: https://myorg.atlassian.net    # defaults to jira.base_url
  email: dev@myorg.com                      # defaults to jira.email
  api_token: $JIRA_API_TOKEN                # defaults to jira.api_token
  space_key: ENG                            # required
  parent_page_id: "123456"                  # optional
  labels: [release-notes, frontend]         # optional: auto-tag pages
  view_restriction: [alice, bob]            # optional: restrict view access
  edit_restriction: [alice]                 # optional: restrict edit access
  template: /path/to/page.tmpl              # optional: custom layout template
```

### Custom Template

The `template` field points to a Go `text/template` file that controls the order and presence of page sections. The template receives a `PageSections` struct with pre-rendered HTML strings:

| Field | Content |
|---|---|
| `.PrevNextNav` | Prev/Next release navigation links |
| `.PageProperties` | Page Properties macro for dashboards |
| `.AISummary` | AI-generated analytical summary (when `--ai-enhance`) |
| `.Analytics` | Metrics, contributors, overview, impact tables |
| `.ReleaseNotes` | The release notes body (sections, breaking changes) |
| `.EpicsPanel` | Jira epic grouping panel |
| `.CommandFooter` | The patchlog command that generated this page |

Default template order: `PrevNextNav → PageProperties → AISummary → Analytics → ReleaseNotes → EpicsPanel → CommandFooter`

## Git Provider Integration

patchlog can create draft releases on GitHub, GitLab, and Gitea.

### GitHub

```yaml
provider:
  type: github
  token: $GITHUB_TOKEN
  repo: myorg/myrepo
```

- Auth: `Authorization: Bearer <token>`
- Releases are created as **drafts** by default (set `provider.draft: false` to publish immediately)
- `FetchPR` links merge requests to commits (retries on 5xx/429)

### GitLab

```yaml
provider:
  type: gitlab
  token: $GITLAB_TOKEN
  repo: myorg/myrepo
```

- Auth: `PRIVATE-TOKEN: <token>`
- Releases are created immediately (GitLab has no draft concept)
- Creates release pointing to `HEAD` ref

### Gitea

```yaml
provider:
  type: gitea
  token: $GITEA_TOKEN
  repo: myorg/myrepo
  base_url: https://gitea.example.com  # required
```

- Auth: `Authorization: token <token>`
- Releases are created as **drafts** by default (set `provider.draft: false` to publish immediately)
- `base_url` is required (no default)

## Commit Linting

```bash
# Lint commits since the last tag
patchlog lint

# Lint a specific range
patchlog lint --from v1.0.0 --to HEAD

# Use AI to suggest improved commit messages
patchlog lint --ai
```

### Lint rules

| Rule | Severity | Description |
|------|----------|-------------|
| `format` | error | Must match `type(scope): description` pattern |
| `unknown-type` | error | Type not in known list (feat, fix, perf, refactor, docs, test, style, ci, chore, build, revert) |
| `short-description` | warning | Description shorter than 10 characters |
| `capitalization` | warning | Description should start with lowercase |
| `trailing-period` | warning | Description should not end with a period |
| `imperative-mood` | info | Description should use imperative mood |
| `missing-body` | info | feat/fix/refactor commits should have a body |
| `breaking-no-explanation` | warning | Breaking change without body explanation |
| `missing-jira-ref` | info | fix commits should reference a Jira ticket |

### Conventional commit format

```
type(scope): description

Optional body explaining the change in detail.

Optional footer with BREAKING CHANGE: or ticket references.
```

Recognized types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `style`, `ci`, `chore`, `build`, `revert`

Breaking changes are detected via `!` after scope or `BREAKING CHANGE` in the footer:

```
feat(api)!: rename /users to /accounts

BREAKING CHANGE: The /users endpoint has been renamed to /accounts.
Update all API clients to use the new path.
```

## Changelog Audit

```bash
# Audit CHANGELOG.md against git history
patchlog audit

# Audit a specific changelog file
patchlog audit --changelog docs/RELEASE_NOTES.md

# Audit a specific range
patchlog audit --from v1.0.0 --to HEAD
```

The audit compares your changelog file against actual git commits and reports:

- **Missing entries**: commits that should be in the changelog but aren't (matched via fuzzy matching on header, keywords, Jira keys, hash prefix, PR refs)
- **Stale entries**: placeholder or empty changelog entries (`todo`, `tbd`, `fixme`, `placeholder`, `coming soon`, `n/a`)

Commit types `docs`, `test`, `style`, `ci`, `chore` are skipped (not expected in changelogs).

Exit code 1 if issues are found.

## Multi-Repo Aggregation

```bash
# Aggregate changelogs from multiple repos
patchlog multi ./services/api ./services/auth ./frontend

# With a path filter
patchlog multi ./services/api ./services/auth --filter pkg/api

# Write to file
patchlog multi ./repo-a ./repo-b --out AGGREGATED_CHANGELOG.md

# JSON format
patchlog multi ./repo-a ./repo-b --format json
```

Generates a single aggregated changelog from multiple repositories, with each repo's changes grouped under its own heading. Each repo is processed with classification enabled.

## Metrics & Code Stats

When `--metrics` is passed (markdown format only), patchlog appends a metrics section to the output:

### Release metrics

- Total commits, unique authors, breaking changes
- Date range, conventional commit ratio
- Commits with Jira refs, total Jira tickets linked
- Average commit body length

### Commit quality

- Average header length
- Commits with body (count + percentage)
- Commits with scope (count + percentage)
- Breaking changes count, reverts count

### Velocity

- Commits per day
- Average hours between commits
- Most active day of week
- Weekend commit ratio
- Release commit span (first to last included commit) and release age. These are not PR lead/cycle-time measurements.

### Code changes

- Files touched, lines added, lines deleted, net lines
- Largest single change
- Files by extension
- File hotspots (most-changed files, top 10)

### Descriptive proxies

- `touched_test_file_ratio`: touched test files divided by touched test and source files; true coverage must come from CI coverage artifacts.
- `change_complexity_proxy`: changed-line/file/churn heuristic; true complexity requires language-aware analysis.
- `cross_cutting_change_risk`: cross-cutting/scope/API-change heuristic; true dependency risk requires a language-aware dependency graph.
- `release_contribution_concentration`: contributors accounting for 80% of release commits; it is not a maintainership or knowledge measure.

These proxies are analytics only and are never release-gate conditions. Lead and cycle time should be derived from PR, merge, deployment, and production timestamps.

### Contributors

Ranked by commit count (descending), with alphabetical tiebreaker.

## Caching

patchlog caches Jira issue responses and Confluence page lookups to `.patchlog/cache/` in the repo directory. This avoids redundant API calls when re-running patchlog during development.

- **TTL**: 24 hours (entries expire after this)
- **Location**: `.patchlog/cache/<namespace>/<key>.json`
- **Atomic writes**: write to temp file, then rename (prevents corruption)
- **Thread-safe**: `sync.RWMutex` protects concurrent access
- **Corrupt JSON**: treated as cache miss (graceful degradation)

Cached namespaces:

| Namespace | What | TTL |
|-----------|------|-----|
| `jira` | Issue details (summary, priority, status, etc.) | 24h |
| `confluence` | Page lookups (ID, title, URL by title) | 24h |

```bash
# Bypass cache (for CI reproducibility)
patchlog --no-cache

# Clear cache
patchlog cache clear
```

The `.patchlog/` directory is in `.gitignore` by default.

## Retry & Resilience

patchlog includes retry with exponential backoff for external API calls:

### HTTP client (`pkg/httpclient`)

- **Default client**: 30s timeout, connection pooling (10 max idle, 5 per host)
- **Streaming client**: 120s timeout (for AI token streaming)
- **Retry**: 3 attempts with exponential backoff (base 1s, max 10s) + jitter
- Retries on: 5xx server errors, 429 rate limit, network errors
- Respects context cancellation (no retry after `ctx.Done()`)

### AI streaming (`pkg/ai/stream.go`)

- Retries up to 2 times on retryable errors (rate limit, server error, network)
- Does NOT retry if tokens were already received (partial response returned)
- Exponential backoff between attempts (1s, 2s) with context-aware sleep
- Request body buffered and replayed across attempts

### Applied to

| Component | Retry | Concurrency |
|-----------|-------|-------------|
| Jira `FetchIssue` | Yes (3 attempts) | Bounded (default 5) |
| Confluence API calls | Yes (3 attempts) | Sequential |
| AI generate/stream | Yes (2 retries) | Sequential |
| Provider `CreateRelease` | No (non-idempotent) | Sequential |
| Provider `FetchPR` | Yes (3 attempts) | Sequential |

## Console Output

patchlog provides real-time feedback with spinners, colors, and a summary table:

```
┌──────────────────────────────────────────────────┐
│  ⚡ patchlog v0.5.0                              │
│                                                  │
│  auto-generate release notes                     │
│  from git history                                │
└──────────────────────────────────────────────────┘

  ✓ Fetching commits...
  ✓ Enriching Jira issues...
  → Auto bump (minor) → 1.3.0
  → Generating AI prose (streaming)...
  Here are the release notes for v1.3.0...
  ✓ Publishing release draft...
  ✓ Publishing to Confluence...
  → Created Confluence page: https://myorg.atlassian.net/wiki/spaces/ENG/pages/12345
  ✓ Accumulating changelog (md)...

  ─── Release Summary ───
  Version:             1.3.0
  Commits analyzed:    12
  Jira tickets:        4 enriched
  Bump level:          minor
  Release draft:       https://github.com/myorg/myrepo/releases/tag/1.3.0
  Confluence page:     https://myorg.atlassian.net/wiki/spaces/ENG/pages/12345
  Changelog:           updated
  Duration:            8.3s
```

Set `NO_COLOR=1` to disable colors. Use `--quiet` to suppress all non-essential output.

## GitLab CI Integration

A checksum-verifying GitLab CI pipeline template is included at `templates/gitlab-ci-patchlog.yml`. It pins an exact released patchlog version and never bumps or retags an immutable tag pipeline. It provides:

- **Commit linting** on merge requests (with AI suggestions)
- **Release notes generation** from the already-created immutable tag
- **Changelog audit** on main branch changes
- **Release preview** (dry-run) on merge requests

### Include in your pipeline

```yaml
# If patchlog is in your repo
include:
  - local: '/templates/gitlab-ci-patchlog.yml'

# Or from the published repository
include:
  - remote: 'https://raw.githubusercontent.com/fxdv/patchlog/main/templates/gitlab-ci-patchlog.yml'
```

### Required CI/CD variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `AI_API_KEY` | If using openai/anthropic | AI provider authentication |
| `PATCHLOG_VERSION` | Always | Exact immutable patchlog tag, such as `v1.4.2` |
| `PATCHLOG_RELEASE_BASE` | Always | Release root used to download assets and `SHA256SUMS` |
| `GIT_PROVIDER_TOKEN` | For releases | Git provider API token |
| `JIRA_API_TOKEN` | Optional | Jira enrichment |

### Optional variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PATCHLOG_CONFIG` | `patchlog.yaml` | Path to config file |

## Testing

patchlog has a multi-layer test suite: unit, integration, E2E, and a quality gate.

### Test layers

| Layer | Location | What it covers |
|-------|----------|---------------|
| **Unit** | `pkg/*/*_test.go` | Pure logic: commit parsing, classification, version bumping, categorization, config loading, prompt building, Jira key extraction, Confluence HTML rendering, markdown/JSON output, cache, audit, lint, metrics |
| **Integration** | `tests/integration/` | Mock HTTP servers for Jira API (fetch + cache + auth + concurrency), all 3 AI providers (streaming + non-streaming), Confluence create + update |
| **E2E** | `tests/e2e/` | Builds the binary, creates real git repos, runs patchlog with various flags (markdown, JSON, dry-run, --bump auto/patch, breaking changes, Jira keys, --filter, monorepo, --version, Confluence, GitHub, quiet, env expansion) |

### Running tests

```bash
# All tests
make test

# Individual layers
make unit          # pkg/... with -race
make integration   # tests/integration/...
make e2e           # builds binary, tests/e2e/...

# Quality checks
make quality       # fmt + vet + build
make vet           # go vet only
make fmt           # check formatting
make cover         # generate coverage report (HTML + terminal)
make cover-check   # fail if coverage < 60%
make bench         # run benchmarks

# Full gate (quality + unit + integration)
make gate

# Full gate including e2e (slower, builds binary + real git repos)
bash scripts/gate.sh
```

### Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary with version injection |
| `make unit` | Unit tests with `-race` flag |
| `make integration` | Integration tests with mock HTTP servers |
| `make e2e` | End-to-end tests with real git repos |
| `make test` | All three layers |
| `make quality` | gofmt + go vet + build |
| `make cover` | Generate coverage report (HTML + terminal) |
| `make cover-check` | Fail if total coverage drops below 60% |
| `make bench` | Run benchmarks with `go test -bench` |
| `make gate` | Quality gate (quality + unit + integration, no e2e) |
| `make regression` | Run golden-file e2e regression tests |
| `make clean` | Remove build artifacts |

### Quality gate (`scripts/gate.sh`)

Runs: `go vet`, `gofmt` check, build, unit tests (with `-race`), integration tests, e2e tests.

Auto-detects Go in common locations and fails closed if Go or any required check is unavailable.

### Pre-push hook

```bash
echo 'bash scripts/gate.sh || exit 1' > .git/hooks/pre-push
chmod +x .git/hooks/pre-push
```

## Architecture

```
patchlog/
├── cmd/patchlog/          # CLI entry point
│   ├── main.go            # Flag parsing, orchestration, signal handling
│   ├── orchestration.go   # Apply ledger and partial-completion errors
│   ├── summary_metrics.go # Descriptive release proxies
│   ├── stages.go          # Focused enrichment/analysis stages
│   ├── pipeline_state.go  # Shared stage state, including dry-run/cache
│   ├── pipeline.go        # buildReport, enrichWithJira, determineBumpLevel
│   ├── publish.go         # renderProse, runPublish, runConfluencePublish, changelog accumulate
│   ├── interactive.go     # `init` wizard, `review`, `recover` subcommands
│   ├── lint.go            # `lint` subcommand
│   ├── audit.go           # `audit` subcommand
│   ├── multi.go           # `multi` subcommand
│   ├── env.go             # Environment variable expansion
│   └── banner.go          # ASCII art banner
├── internal/
│   └── ignore/            # Shared ignore-pattern regex compiler
├── pkg/
│   ├── ai/                # AI provider clients (Ollama, OpenAI, Anthropic)
│   │   ├── ai.go          # Client interface, NewClient factory
│   │   ├── ollama.go      # Ollama API client
│   │   ├── openai.go      # OpenAI API client
│   │   ├── anthropic.go   # Anthropic API client
│   │   ├── stream.go      # Shared SSE streaming with retry
│   │   ├── prompt_template.go # Prompt construction
│   │   ├── generate.go    # Prose orchestration
│   │   ├── chunk.go       # Bounded prompt chunking
│   │   ├── safe.go        # Shared redaction and input limits
│   │   ├── exclude.go     # Sensitive/generated file exclusion
│   │   └── errors.go      # Typed AI errors with retry classification
│   ├── audit/             # Changelog audit (missing/stale entries)
│   ├── bump/              # Immutable bump plans + transactional apply
│   ├── cache/             # File-based JSON cache with TTL
│   ├── categorize/        # Commit → section categorization
│   ├── classify/          # Diff-aware significance classification
│   │   ├── classify.go    # Threshold-based classification
│   │   └── diff.go        # File categorization, diff analysis
│   ├── commit/            # Conventional commit parsing
│   ├── config/            # Config loading, validation, defaults
│   ├── confluence/        # Split client, storage renderer, analytics, trends
│   ├── console/           # Terminal output (spinners, colors, summary)
│   ├── deps/              # Dependency changelog detection + upstream fetch
│   ├── gitlog/            # Git log fetching (context-aware)
│   ├── gitwiki/           # GitLab wiki API client
│   ├── httpclient/        # HTTP client factory + retry with backoff
│   ├── internal/
│   │   ├── pattern/       # Jira key extraction regex
│   │   └── truncate/      # Rune-safe string truncation
│   ├── jira/              # Jira Cloud API client (concurrent, cached)
│   ├── lint/              # Conventional commit linter
│   ├── metrics/           # Release metrics & code stats
│   ├── multirepo/         # Multi-repo changelog aggregation
│   ├── provider/          # Git provider clients (GitHub, GitLab, Gitea)
│   ├── safehtml/          # Single HTML/XML text-escaping boundary
│   ├── render/            # Output rendering
│   │   ├── markdown.go    # Markdown rendering
│   │   ├── json.go        # JSON rendering
│   │   └── changelog.go   # Changelog accumulation rendering
│   └── significance/      # Significance level types (skip/patch/minor/major)
├── templates/             # GitLab CI template
├── tests/
│   ├── integration/       # Integration tests (mock HTTP servers)
│   └── e2e/               # End-to-end tests (real git repos + binary)
├── scripts/
│   └── gate.sh            # Quality gate script
├── .github/workflows/     # Required CI and checksum-producing releases
├── Makefile               # Build, test, coverage, bench targets
└── go.mod                 # Go 1.22, dep: gopkg.in/yaml.v3
```

### Dependencies

patchlog has a single external dependency: `gopkg.in/yaml.v3` for config parsing. All HTTP client logic, retry, streaming, and API integrations are implemented from scratch using only the Go standard library.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Git error, rendering error, lint errors, audit issues, or publish failure |
| `2` | Config or flag error |

## License

MIT
