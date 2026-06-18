# patchlog

Auto-generate release notes from git history. Understands conventional commits, enriches with Jira context, streams AI-powered prose, publishes to Confluence, and figures out your next version number on its own.

## Quick Start

```bash
# Install
go install patchlog/cmd/patchlog@latest

# Generate release notes from the last tag to HEAD
patchlog

# Interactive setup (creates patchlog.yaml)
patchlog init
```

## Workflow

### 1. Set up

```bash
patchlog init
```

Walks you through configuring your AI provider, git host, Jira, and Confluence. Writes a `patchlog.yaml` to your project root.

Minimal config:

```yaml
ai:
  provider: ollama
  model: llama3.2
```

Full config:

```yaml
ai:
  provider: openai
  model: gpt-4o-mini
  api_key: $OPENAI_API_KEY
  max_tokens: 4096

jira:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  project_key: PROJ

confluence:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  space_key: ENG
  parent_page_id: "123456"

provider:
  type: github
  token: $GITHUB_TOKEN
  repo: myorg/myrepo
```

### 2. Write commits

Use conventional commits. Reference Jira tickets in your messages — patchlog picks them up automatically.

```
feat(api): add pagination to user list endpoint PROJ-123

Fixes overflow on large datasets. Also improves response times
by caching query results.

PROJ-456
```

Recognized types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `style`, `ci`, `chore`

Breaking changes are detected via `!` after scope or `BREAKING CHANGE` in the footer:

```
feat(api)!: rename /users to /accounts
```

### 3. Generate release notes

```bash
# Markdown (default)
patchlog

# AI-generated prose, streaming tokens to your terminal
patchlog --format prose --tone dev

# Customer-facing notes
patchlog --format prose --tone customer

# Executive summary
patchlog --format prose --tone exec

# Specific range
patchlog --from v1.2.0 --to v1.3.0

# First release (no previous tag)
patchlog --first

# Dry run — just see the commit count
patchlog --dry-run

# Monorepo — only changes under a path
patchlog --filter pkg/api

# JSON output for piping elsewhere
patchlog --format json
```

### 4. Auto-bump version

```bash
# Let patchlog decide the bump level based on commit significance
patchlog --bump auto

# Or specify manually
patchlog --bump patch
patchlog --bump minor
patchlog --bump major
```

`--bump auto` scans your commits and picks the highest significance:

| Commit type        | Breaking | Changed files | Significance |
|--------------------|----------|---------------|-------------|
| `feat!` / `BREAKING CHANGE` | yes | any | **major** |
| `feat`             | no       | >= 5 files    | **major** |
| `feat`             | no       | < 5 files     | **minor** |
| `fix`              | no       | >= 5 files    | **minor** |
| `fix`              | no       | < 5 files     | **patch** |
| `perf`             | no       | any           | **minor** |
| `refactor`         | no       | any           | **patch** |
| `docs`, `test`, `style`, `ci`, `chore` | no | any | **skip** |

Auto-bump prints a color-coded summary:

```
  1x major  3x minor  7x patch
  → Auto bump (major) → 2.0.0
```

Version detection supports `package.json`, `Cargo.toml`, `pyproject.toml`, `VERSION`, and `version.txt`.

### 5. Publish

```bash
# Create a draft release on GitHub
patchlog --bump auto --publish

# Review before publishing
patchlog --review --publish

# Publish to Confluence
patchlog --confluence

# Publish everywhere
patchlog --bump auto --publish --confluence
```

### 6. Full release pipeline

```bash
patchlog --bump auto --format prose --tone dev --publish --confluence
```

This one command:
1. Reads commits since the last tag
2. Classifies each commit and determines the bump level
3. Fetches Jira details for any referenced tickets
4. Bumps the version in your project files
5. Streams AI-generated release notes to your terminal
6. Creates a draft release on your git host
7. Publishes or updates a Confluence page with analytics

## Console Output

patchlog provides real-time feedback with spinners, colors, and a summary table:

```
┌──────────────────────────────────────────────────┐
│  ⚡ patchlog v0.4.0                              │
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

  ─── Release Summary ───
  Version:             1.3.0
  Commits analyzed:    12
  Jira tickets:        4 enriched
  Bump level:          minor
  Release draft:       https://github.com/myorg/myrepo/releases/tag/1.3.0
  Confluence page:     https://myorg.atlassian.net/wiki/spaces/ENG/pages/12345
  Duration:            8.3s
```

Set `NO_COLOR=1` to disable colors.

## Confluence Integration

When `--confluence` is passed, patchlog publishes a rich page to Confluence Cloud:

- **Create or update**: if a page with the same title exists, it's updated in-place (versioned). Otherwise a new page is created.
- **Under a parent**: set `parent_page_id` to publish under a specific page tree.
- **Reuses Jira auth**: if `confluence.base_url` is not set, it falls back to `jira.base_url` with the same credentials.

The Confluence page includes:

- **Formatted release notes** with proper headings, lists, and author attribution
- **Jira ticket links** as clickable links with priority status macros (color-coded: red for critical, orange for high, etc.)
- **Breaking change banner** using Confluence status macro
- **Collapsible analytics section** with a table showing:
  - Total commits, breaking changes
  - Per-type breakdown (Features, Bug Fixes, etc.)
  - Impact distribution (major/minor/patch)
  - Jira tickets linked

Config:

```yaml
confluence:
  base_url: https://myorg.atlassian.net    # defaults to jira.base_url
  email: dev@myorg.com                      # defaults to jira.email
  api_token: $JIRA_API_TOKEN                # defaults to jira.api_token
  space_key: ENG                            # required
  parent_page_id: "123456"                  # optional
```

## Jira Integration

When Jira is configured, patchlog:

1. Extracts ticket keys from commit messages and bodies (`PROJ-123`, `ENG-456`, etc.)
2. Filters by project key if configured
3. Fetches issue details (summary, priority, status, labels) from Jira Cloud API
4. Enriches each changelog entry with linked ticket info

In markdown output:

```
- add pagination to user list endpoint (#42) [PROJ-123](https://myorg.atlassian.net/browse/PROJ-123) Add cursor-based pagination by @alice
```

In AI-generated prose, Jira context is included in the prompt so the model can reference ticket summaries and priorities.

## AI Streaming & Chunking

When using `--format prose`, patchlog streams tokens to stderr in real-time — you see the release notes being written as they're generated.

For large releases, patchlog automatically chunks the prompt:

- If the prompt exceeds 4,000 characters, it's split into section-based chunks
- Each chunk is sent as a separate request with part numbering (`Part 1 of 3`)
- Breaking changes are always included in the first chunk
- Large sections are split by item batches
- The final output is stitched together

This avoids context window limits and token caps on any provider.

## Configuration Reference

```yaml
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

ignore:
  - "^Merge"
  - "^chore\\(deps\\)"

author:
  show: true
  format: name

links:
  commit: https://github.com/%s/commit/%s
  issue: https://github.com/%s/issues/%s
  compare: https://github.com/%s/compare/%s...%s

repo: myorg/myrepo

ai:
  provider: ollama        # ollama | openai | anthropic
  model: llama3.2
  api_key: $OPENAI_API_KEY
  base_url: ""
  max_tokens: 4096

provider:
  type: github            # github | gitlab | gitea
  token: $GITHUB_TOKEN
  repo: myorg/myrepo
  base_url: ""

jira:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  project_key: PROJ

confluence:
  base_url: https://myorg.atlassian.net
  email: dev@myorg.com
  api_token: $JIRA_API_TOKEN
  space_key: ENG
  parent_page_id: "123456"

bump:
  auto_detect: true
  files: []
```

Environment variables are resolved when a value starts with `$` (e.g. `$GITHUB_TOKEN` reads from `GITHUB_TOKEN`).

## Subcommands

| Command | Description |
|---------|-------------|
| `patchlog init` | Interactive setup wizard |
| `patchlog recover <file>` | Re-render markdown from a saved JSON report |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | latest tag | Start ref |
| `--to` | `HEAD` | End ref |
| `--config` | `patchlog.yaml` | Config file path |
| `--repo` | `.` | Path to git repository |
| `--out` | stdout | Output file |
| `--format` | `markdown` | Output format: `markdown`, `json`, `prose` |
| `--tone` | `dev` | Prose tone: `dev`, `customer`, `exec` |
| `--bump` | | Bump version: `patch`, `minor`, `major`, `auto` |
| `--publish` | false | Create draft release on remote provider |
| `--confluence` | false | Publish or update Confluence page |
| `--filter` | | Monorepo path filter |
| `--first` | false | Start from first commit (no tag needed) |
| `--dry-run` | false | Show commit range and count |
| `--classify` | true | Enable significance classification |
| `--review` | false | Review output before writing |
| `--version` | | Show version |

## Testing

patchlog has a multi-layer test suite: unit, integration, E2E, regression, and a quality gate that runs before push.

### Test layers

| Layer | Location | What it covers |
|-------|----------|---------------|
| **Unit** | `pkg/*/\*_test.go` | Pure logic: commit parsing, classification, version bumping, categorization, config loading, prompt building, Jira key extraction, Confluence HTML rendering, markdown/JSON output |
| **Integration** | `tests/integration/` | Mock HTTP servers for Jira API (fetch + cache + auth), all 3 AI providers (streaming + non-streaming), Confluence create + update page |
| **E2E** | `tests/e2e/` | Builds the binary, creates real git repos, runs patchlog with various flags (markdown, JSON, dry-run, --bump auto/patch, breaking changes, Jira keys, --filter, monorepo, --version) |
| **Regression** | `testdata/golden/` | Golden file snapshots for markdown output, diffed on future runs |
| **Quality gate** | `scripts/gate.sh` | go vet, gofmt, build, all test layers — blocks push on failure |

### Running tests

```bash
# All tests
make test

# Individual layers
make unit
make integration
make e2e

# Quality checks
make quality          # fmt + vet + build
make vet              # go vet only
make fmt              # check formatting
make cover            # generate coverage report
make cover-check      # fail if coverage < 60%
make bench            # run benchmarks

# Full gate (quality + unit + integration + e2e)
make gate

# Or run the gate script directly
bash scripts/gate.sh
```

### Test coverage

Current coverage by package:

| Package | Coverage |
|---------|----------|
| `pkg/render` | 100% |
| `pkg/commit` | 96.8% |
| `pkg/classify` | 94.1% |
| `pkg/categorize` | 90.3% |
| `pkg/config` | 80% |
| `pkg/bump` | 74.2% |
| `pkg/confluence` | 43.3% |
| `pkg/jira` | 34% |
| `pkg/ai` | 23.7% |

Packages like `gitlog`, `console`, and `provider` require real git repos or terminals and are covered by the E2E suite instead.

### Pre-push hook

Install the quality gate as a git pre-push hook to block pushes when tests fail:

```bash
echo 'bash scripts/gate.sh || exit 1' > .git/hooks/pre-push
chmod +x .git/hooks/pre-push
```

Now every `git push` will automatically run all checks first.

### Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary |
| `make unit` | Unit tests only |
| `make integration` | Integration tests with mock HTTP servers |
| `make e2e` | End-to-end tests with real git repos |
| `make test` | All three layers |
| `make quality` | gofmt + go vet + build |
| `make cover` | Generate coverage report (HTML + terminal) |
| `make cover-check` | Fail if total coverage drops below 60% |
| `make bench` | Run benchmarks with `go test -bench` |
| `make gate` | Full quality gate (quality + unit + integration) |
| `make clean` | Remove build artifacts |

## License

MIT
