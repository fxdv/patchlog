package main

import (
	"flag"
	"fmt"
	"io"
)

type cliOptions struct {
	releaseMode   bool
	releaseAction string
	approvePlan   string
	releaseBranch string
	extensionMode string
	planJSON      bool
	from          string
	to            string
	cfgPath       string
	repo          string
	outPath       string
	format        string
	first         bool
	dryRun        bool
	showVer       bool
	filter        string
	tone          string
	classifyOn    bool
	publish       bool
	bumpLevel     string
	review        bool
	confluence    bool
	changelog     bool
	metrics       bool
	aiEnhance     bool
	quiet         bool
	noCache       bool
	theme         bool
	tag           bool
	push          bool
	force         bool
	trends        bool
	lang          string
	gate          bool
	deps          bool
	requireConv   float64
	infer         bool
	semantic      bool
	drift         bool
	gamify        bool
	html          bool
	labs          bool
}

func parseCLI(args []string, stderr io.Writer) (cliOptions, []string, error) {
	var opts cliOptions
	if len(args) > 0 {
		switch args[0] {
		case "release":
			opts.releaseMode = true
			args = args[1:]
			if len(args) > 0 {
				switch args[0] {
				case "prepare", "finalize", "direct":
					opts.releaseAction = args[0]
					args = args[1:]
				}
			}
		case "ai", "confluence", "metrics", "labs":
			opts.extensionMode = args[0]
			args = args[1:]
		}
	}
	fs := flag.NewFlagSet("patchlog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.from, "from", "", "Start ref (default: latest tag)")
	fs.StringVar(&opts.to, "to", "HEAD", "End ref")
	fs.StringVar(&opts.cfgPath, "config", defaultConfigPath(), "Config file path (or PATCHLOG_CONFIG)")
	fs.StringVar(&opts.repo, "repo", ".", "Path to git repository")
	fs.StringVar(&opts.outPath, "out", "", "Output file (default: stdout)")
	fs.StringVar(&opts.format, "format", "markdown", "Output format: markdown, json, prose")
	fs.BoolVar(&opts.first, "first", false, "Start from the very first commit")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "Plan and validate every requested action without modifying local or remote state")
	fs.BoolVar(&opts.planJSON, "plan-json", false, "Write the versioned protected release plan JSON to stdout (requires release --dry-run)")
	fs.StringVar(&opts.approvePlan, "approve", "", "Approve one exact sha256 release-plan fingerprint before mutation")
	fs.StringVar(&opts.releaseBranch, "release-branch", "", "Protected-mode prepare branch (default: release/<tag>)")
	fs.BoolVar(&opts.showVer, "version", false, "Show version and exit")
	fs.StringVar(&opts.filter, "filter", "", "Monorepo path filter (e.g. pkg/api)")
	fs.StringVar(&opts.tone, "tone", "dev", "Output tone: dev, customer, exec")
	fs.BoolVar(&opts.classifyOn, "classify", true, "Enable significance classification")
	fs.BoolVar(&opts.publish, "publish", false, "Create release draft on remote provider")
	fs.StringVar(&opts.bumpLevel, "bump", "", "Bump version: patch, minor, major, auto")
	fs.BoolVar(&opts.review, "review", false, "Review output before writing")
	fs.BoolVar(&opts.confluence, "confluence", false, "Publish or update Confluence page")
	fs.BoolVar(&opts.changelog, "changelog", false, "Accumulate changelog into md file, GitLab wiki, or Confluence page")
	fs.BoolVar(&opts.metrics, "metrics", false, "Append release metrics and code stats to output")
	fs.BoolVar(&opts.aiEnhance, "ai-enhance", false, "Prepend AI-generated summary to markdown output")
	fs.BoolVar(&opts.quiet, "quiet", false, "Suppress banner and spinner output")
	fs.BoolVar(&opts.noCache, "no-cache", false, "Bypass cache for CI reproducibility")
	fs.BoolVar(&opts.theme, "theme", false, "Group commits into AI-generated themes instead of conventional types")
	fs.BoolVar(&opts.tag, "tag", false, "Create annotated git tag after --bump")
	fs.BoolVar(&opts.push, "push", false, "Push commit and tag to origin (requires --tag)")
	fs.BoolVar(&opts.force, "force", false, "Override --tag safety checks (dirty tree, existing tag)")
	fs.BoolVar(&opts.trends, "trends", false, "Publish trend dashboard to Confluence (requires --confluence)")
	fs.StringVar(&opts.lang, "lang", "", "Output language(s): en, ru, zh, or en,ru for bilingual")
	fs.BoolVar(&opts.gate, "gate", false, "Exit non-zero if the conventional-commit release policy fails")
	fs.Float64Var(&opts.requireConv, "require-conv", 0, "Override gate min conventional commit ratio (0-1)")
	fs.BoolVar(&opts.infer, "infer", false, "Use AI to infer conventional commit types for uncategorized commits")
	fs.BoolVar(&opts.semantic, "semantic", false, "Use AI to generate semantic summaries of actual code diffs")
	fs.BoolVar(&opts.drift, "drift", false, "Compare planned Jira tickets vs delivered (plan-vs-actual)")
	fs.BoolVar(&opts.gamify, "gamify", false, "[experimental] Add contributor achievements (requires --labs)")
	fs.BoolVar(&opts.html, "html", false, "Generate standalone HTML report with charts")
	fs.BoolVar(&opts.labs, "labs", false, "[experimental] Enable DPI, health signals, people analytics, and gamification")
	fs.BoolVar(&opts.deps, "deps", false, "Detect dependency version bumps and fetch upstream changelogs")
	fs.Usage = func() { printCLIUsage(fs, stderr) }
	if err := fs.Parse(args); err != nil {
		return cliOptions{}, nil, err
	}
	if err := applyExtensionScope(&opts, fs); err != nil {
		return cliOptions{}, nil, err
	}
	if err := applySafeReleaseDefaults(&opts, fs); err != nil {
		return cliOptions{}, nil, err
	}
	return opts, fs.Args(), nil
}

func applyExtensionScope(opts *cliOptions, fs *flag.FlagSet) error {
	used := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { used[f.Name] = true })
	requiredScope := map[string]string{
		"ai-enhance": "ai",
		"infer":      "ai",
		"semantic":   "ai",
		"theme":      "ai",
		"confluence": "confluence",
		"trends":     "confluence",
		"metrics":    "metrics",
		"labs":       "labs",
		"gamify":     "labs",
	}
	for name, scope := range requiredScope {
		if used[name] && opts.extensionMode != scope {
			return fmt.Errorf("--%s moved to `patchlog %s`; run `patchlog %s --help`", name, scope, scope)
		}
	}
	switch opts.extensionMode {
	case "":
		return nil
	case "ai":
		if !used["ai-enhance"] && !used["infer"] && !used["semantic"] && !used["theme"] {
			opts.aiEnhance = true
		}
	case "confluence":
		opts.confluence = true
	case "metrics":
		opts.metrics = true
	case "labs":
		opts.labs = true
	default:
		return fmt.Errorf("unsupported extension subcommand %q", opts.extensionMode)
	}
	return nil
}

func applySafeReleaseDefaults(opts *cliOptions, fs *flag.FlagSet) error {
	if opts == nil || !opts.releaseMode {
		if opts != nil && opts.planJSON {
			return fmt.Errorf("--plan-json requires `patchlog release --dry-run`")
		}
		return nil
	}
	used := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		used[f.Name] = true
	})
	if opts.planJSON && !opts.dryRun {
		return fmt.Errorf("--plan-json is side-effect-free output and requires --dry-run")
	}
	if opts.planJSON && opts.releaseAction == "direct" {
		return fmt.Errorf("--plan-json exports the protected release contract and is unavailable in `patchlog release direct` compatibility mode")
	}

	directOnly := []string{"tag", "push", "force", "changelog"}
	for _, name := range directOnly {
		if used[name] && opts.releaseAction != "direct" {
			return fmt.Errorf("--%s is available only through explicit compatibility mode; run `patchlog release direct --%s ...`", name, name)
		}
	}

	switch opts.releaseAction {
	case "":
		if used["bump"] {
			return fmt.Errorf("manual protected bumps require an explicit prepare phase; run `patchlog release prepare --bump %s --dry-run`", opts.bumpLevel)
		}
		if used["publish"] {
			return fmt.Errorf("--publish requires an explicit finalize phase; run `patchlog release finalize --publish --dry-run`")
		}
		opts.bumpLevel = "auto"
	case "prepare":
		if used["publish"] {
			return fmt.Errorf("--publish belongs to protected finalize; run `patchlog release finalize --publish --dry-run` after the release PR is merged")
		}
		if opts.bumpLevel == "" {
			opts.bumpLevel = "auto"
		}
	case "finalize":
		if used["bump"] {
			return fmt.Errorf("--bump belongs to protected prepare; run `patchlog release prepare --bump %s --dry-run`", opts.bumpLevel)
		}
		opts.tag = true
		opts.push = true
	case "direct":
		if opts.bumpLevel == "" {
			opts.bumpLevel = "auto"
		}
		if !used["tag"] {
			opts.tag = true
		}
		if !used["push"] {
			opts.push = true
		}
	}
	return nil
}
