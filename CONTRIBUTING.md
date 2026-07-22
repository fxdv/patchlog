# Contributing to patchlog

## Quick Start

```bash
bash scripts/build.sh    # Requires Go 1.22+ and builds the binary
./patchlog --version
```

## Architecture

```
patchlog/
├── cmd/patchlog/          CLI entry point and pipeline orchestration
│   ├── main.go            Flag parsing, pipeline flow, summary output
│   ├── orchestration.go   Apply boundary and partial-completion reporting
│   ├── summary_metrics.go Descriptive release proxy computation
│   ├── pipeline_state.go  PipelineState struct, shared helpers
│   ├── stages.go          Pipeline stage functions (infer, jira, deps, etc.)
│   ├── pipeline.go        buildReport, enrichWithJira, computeSnapshot
│   ├── publish.go         runPublish, runConfluencePublish, changelog
│   ├── env.go             Env var expansion, atomicWriteFile, AI client factory
│   ├── trends.go          trends subcommand
│   ├── curate.go          curate subcommand (TUI)
│   ├── postmortem.go      postmortem subcommand
│   ├── lint.go            lint subcommand
│   └── interactive.go     init wizard, review, recover
├── pkg/                   Business logic (all testable without terminals/network)
│   ├── ai/                Provider clients, redaction, limits, prompt components
│   ├── bump/              Immutable bump plans and transactional apply
│   ├── metrics/           Release metrics computation (48 metrics)
│   ├── trends/            Cross-release trend storage and rendering
│   ├── confluence/        Confluence API client + rich page rendering
│   ├── htmlreport/        Standalone HTML report generation
│   ├── dpi/               Experimental labs-only contributor scoring
│   ├── health/            Experimental labs-only team-health signals
│   ├── ownership/         Code ownership heatmap
│   └── ...                (30+ packages, each with godoc)
├── scripts/
│   ├── build.sh           Fail-closed build with version injection
│   └── gate.sh            Quality gate (vet + fmt + build + tests)
└── templates/             GitLab CI template
```

## Pipeline Stages

The main pipeline runs these stages in order:

1. **Fetch** — `buildReport()` fetches commits, parses, classifies, categorizes
2. **Infer** — `stageInfer()` — AI commit type inference
3. **Jira** — `stageJira()` — Jira ticket enrichment
4. **Deps** — `stageDeps()` — Dependency changelog detection
5. **Semantic** — `stageSemantic()` — AI diff summaries
6. **Drift** — `stageDrift()` — Plan-vs-actual comparison
7. **Plan** — Build the immutable bump/tag/release action plan and exact file list
8. **Render** — Markdown/JSON/prose output
9. **Enhance/Theme** — Optional AI transformations before policy evaluation
10. **Review** — Complete manual review without applying release mutations
11. **Gate** — Evaluate only grounded conventional-commit policy
12. **Apply** — Transactional bump, exact-file commit/tag/atomic push
13. **Publish** — Provider, Confluence, changelog, HTML/output, and trends in order

`--dry-run` exits before step 12 and must leave filesystem, Git, cache, and
remote state unchanged. Release mutations are accepted only through
`patchlog release`.

## Key Principles

1. **Single dependency** — Only `gopkg.in/yaml.v3`. Everything else is Go stdlib.
2. **Explicit data boundaries** — AI input is redacted, filtered, bounded, and disclosed.
3. **Opt-in complexity** — `patchlog` with no args is reporting; mutations require `patchlog release`; people scoring requires `--labs`.
4. **Testability** — Pure logic lives in `pkg/`; HTTP and Git boundaries use mock services and temporary repositories.
5. **Exit codes** — 0 success, 1 runtime error, 2 config/flag error, 3 gate failure.

## Testing

```bash
make unit          # pkg/... with -race
make integration   # mock HTTP servers
make e2e           # real git repos + binary
make gate          # vet + fmt + build + unit + integration
bash scripts/gate.sh  # full gate including e2e
```

## Adding a New Feature

1. Create `pkg/yourfeature/` with logic + tests
2. Add config struct to `pkg/config/config.go` with doc comment
3. Add validation to `pkg/config/validate.go` if needed
4. Prefer a focused subcommand; add a top-level flag only for reliable report generation
5. Create `stageYourFeature()` in `cmd/patchlog/stages.go`
6. Wire into `main()` via `PipelineState`
7. Add to `Flag` map on `PipelineState`
8. Update README.md
9. Run `bash scripts/gate.sh`

## Code Style

- `gofmt` enforced (CI fails on unformatted code)
- Doc comments on all exported types and packages
- Named constants for magic numbers
- `fmt.Errorf("context: %w", err)` for error wrapping
- `atomicWriteFile` for file writes (not `os.WriteFile`)
- `newAIClientOrWarn` for AI client creation (not inline `ai.NewClient`)
- `thresholdsFromConfig` / `snapshotFromMetrics` for shared helpers
