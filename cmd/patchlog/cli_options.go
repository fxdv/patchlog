package main

import (
	"flag"
	"fmt"
	"io"
)

type cliOptions struct {
	releaseMode bool
	from        string
	to          string
	cfgPath     string
	repo        string
	outPath     string
	format      string
	first       bool
	dryRun      bool
	showVer     bool
	filter      string
	tone        string
	classifyOn  bool
	publish     bool
	bumpLevel   string
	review      bool
	confluence  bool
	changelog   bool
	metrics     bool
	aiEnhance   bool
	quiet       bool
	noCache     bool
	theme       bool
	tag         bool
	push        bool
	force       bool
	trends      bool
	lang        string
	gate        bool
	deps        bool
	requireConv float64
	infer       bool
	semantic    bool
	drift       bool
	gamify      bool
	html        bool
	labs        bool
}

func parseCLI(args []string, stderr io.Writer) (cliOptions, []string, error) {
	var opts cliOptions
	if len(args) > 0 && args[0] == "release" {
		opts.releaseMode = true
		args = args[1:]
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
	applySafeReleaseDefaults(&opts, fs)
	return opts, fs.Args(), nil
}

func applySafeReleaseDefaults(opts *cliOptions, fs *flag.FlagSet) {
	if opts == nil || !opts.releaseMode {
		return
	}
	explicitReleaseAction := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "bump", "tag", "push", "publish", "confluence", "changelog", "trends":
			explicitReleaseAction = true
		}
	})
	if explicitReleaseAction {
		return
	}
	opts.bumpLevel = "auto"
	opts.tag = true
	opts.push = true
}

func printCLIUsage(fs *flag.FlagSet, out io.Writer) {
	printBanner()
	fmt.Fprintln(out, "\nUsage:")
	fmt.Fprintln(out, "  patchlog [flags]           Generate release notes without mutations")
	fmt.Fprintln(out, "  patchlog release [flags]   Safely bump, tag, and atomically push")
	fmt.Fprintln(out, "\nSubcommands:")
	fmt.Fprintln(out, "  patchlog release       Safe release; defaults to --bump auto --tag --push")
	fmt.Fprintln(out, "  patchlog init          Interactive setup wizard")
	fmt.Fprintln(out, "  patchlog lint          Lint commits against conventional commit standards")
	fmt.Fprintln(out, "  patchlog audit         Audit changelog against git history")
	fmt.Fprintln(out, "  patchlog multi <repos> Aggregate changelogs from multiple repos")
	fmt.Fprintln(out, "  patchlog recover <file> Re-render markdown from saved JSON")
	fmt.Fprintln(out, "  patchlog cache clear   Clear the file cache")
	fmt.Fprintln(out, "  patchlog trends        Show cross-release trend analysis")
	fmt.Fprintln(out, "  patchlog curate        Interactive TUI curator for release notes")
	fmt.Fprintln(out, "  patchlog postmortem    Analyze post-release stability (rollbacks, hotfixes)")
	fmt.Fprintln(out, "\nFlags:")
	fs.PrintDefaults()
	fmt.Fprintln(out, "\nExamples:")
	fmt.Fprintln(out, "  patchlog release --dry-run   Plan and preflight the safe default release")
	fmt.Fprintln(out, "  patchlog release             Apply the reviewed default release")
	fmt.Fprintln(out, "  patchlog --from v1.0.0 --to v1.1.0")
	fmt.Fprintln(out, "  patchlog release --bump minor --tag --push --publish")
}
