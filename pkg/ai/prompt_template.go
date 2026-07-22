package ai

import (
	"bytes"
	"text/template"

	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

type promptData struct {
	Version    string
	Date       string
	Sections   []templatedSection
	Breaking   []render.Item
	Tone       string
	ChunkIndex int
	ChunkTotal int
	LangPrefix string
}

type ContributorStat struct {
	Name    string
	Commits int
}

type SummaryMetrics struct {
	TotalCommits                     int
	TotalAuthors                     int
	TopContributors                  []ContributorStat
	BreakingChanges                  int
	SignificanceCounts               map[string]int
	TypeCounts                       map[string]int
	DateRange                        string
	CommitsPerDay                    float64
	MostActiveDay                    string
	MostActiveDayCount               int
	FilesTouched                     int
	LinesAdded                       int
	LinesDeleted                     int
	NetLines                         int
	JiraTicketsLinked                int
	HotspotDensity                   float64
	ChurnFactor                      float64
	ComplexityPerFeat                float64
	ReleaseCommitSpanHours           float64
	ReleaseAgeHours                  float64
	OwnershipConc                    float64
	OwnershipEntropy                 float64
	ReleaseContributionConcentration int
	FixToFeatureRatio                float64
	TestToSourceRatio                float64
	RefactoringRatio                 float64
	APISurfaceChange                 int
	ReleaseRiskScore                 float64
	BatchFactor                      float64
	RevertRate                       float64
	ScopeIsolation                   float64
	CrossCuttingPct                  float64
	FileVolatility                   float64
	ChangeComplexityProxy            float64
	CrossCuttingChangeRisk           float64
	TechnicalDebtUSD                 float64
	TouchedTestFileRatio             float64
	HotspotScore                     float64
	CommitSizeSmall                  int
	CommitSizeMedium                 int
	CommitSizeLarge                  int
	CommitSizeHuge                   int
	ReviewLoad                       float64
	DeleteRatio                      float64
	CommitMsgQuality                 float64
	AuthorOverlap                    float64
	PeakHourSpread                   float64
	DirectoryBreadth                 float64
	ConfigChurnRate                  float64
	CodeFreshness                    float64
	RhythmConsistency                float64
	ReleaseCadenceDays               int
	DepUpdateFreq                    float64
	LanguageMix                      float64
}

type templatedSection struct {
	Heading string
	Items   []render.Item
	Scopes  []render.ScopeGroup
}

func BuildPrompt(report render.Report, tone Tone) string {
	return BuildPromptLang(report, tone, i18n.LangEN)
}

func BuildPromptLang(report render.Report, tone Tone, lang i18n.Lang) string {
	var sections []templatedSection
	for _, s := range report.Sections {
		sections = append(sections, templatedSection{
			Heading: s.Heading,
			Items:   s.Items,
			Scopes:  s.Scopes,
		})
	}

	data := promptData{
		Version:    report.Version,
		Date:       report.Date,
		Sections:   sections,
		Breaking:   report.Breaking,
		Tone:       string(tone),
		LangPrefix: i18n.PromptPrefix(lang),
	}

	return executePromptTemplate(promptTemplate, data)
}

func BuildChunkPrompt(report render.Report, tone Tone, sections []templatedSection, breaking []render.Item, chunkIndex, chunkTotal int) string {
	return BuildChunkPromptLang(report, tone, sections, breaking, chunkIndex, chunkTotal, i18n.LangEN)
}

func BuildChunkPromptLang(report render.Report, tone Tone, sections []templatedSection, breaking []render.Item, chunkIndex, chunkTotal int, lang i18n.Lang) string {
	data := promptData{
		Version:    report.Version,
		Date:       report.Date,
		Sections:   sections,
		Breaking:   breaking,
		Tone:       string(tone),
		ChunkIndex: chunkIndex,
		ChunkTotal: chunkTotal,
		LangPrefix: i18n.PromptPrefix(lang),
	}

	return executePromptTemplate(promptTemplate, data)
}

func executePromptTemplate(tmplStr string, data promptData) string {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return ""
	}

	return buf.String()
}

const promptTemplate = `{{.LangPrefix}}

{{if .ChunkTotal}}Generate release notes for PART {{.ChunkIndex}} of {{.ChunkTotal}} of software project {{.Version}} ({{.Date}}).{{else}}Generate release notes for software project {{.Version}} ({{.Date}}).{{end}}

TONE: {{.Tone}}
{{- if eq .Tone "dev"}}
Write technical release notes for developers. Be precise, mention PR numbers and authors.
Use markdown with proper headings and code formatting.
{{- else if eq .Tone "customer"}}
Write customer-facing release notes. Focus on benefits and user-visible changes.
Use plain language, avoid jargon. Be warm and inviting.
{{- else if eq .Tone "exec"}}
Write a one-paragraph executive summary. Focus on business impact, key metrics, and strategic value.
Maximum 3-4 sentences. No technical details.
{{- end}}

{{if .ChunkTotal}}CHANGES IN THIS PART:{{else}}CHANGES:{{end}}
{{- if .Breaking}}
BREAKING CHANGES:
{{range .Breaking}}- {{.Description}}{{if .Scope}} ({{.Scope}}){{end}}{{if .Ref}} {{.Ref}}{{end}}{{if .Author}} by @{{.Author}}{{end}}{{range .JiraIssues}} [{{.Key}}: {{.Summary}}{{if .Status}} ({{.Status}}){{end}}{{if .Priority}} priority:{{.Priority}}{{end}}{{if .Type}} type:{{.Type}}{{end}}{{if .EpicKey}} epic:{{.EpicKey}}{{end}}{{if .Components}} components:{{range .Components}}{{.}},{{end}}{{end}}{{if .FixVersions}} fixVersion:{{range .FixVersions}}{{.}},{{end}}{{end}}{{if .Description}} desc:{{.Description}}{{end}}{{end}}
{{- end}}
{{- end}}
{{- range .Sections}}
{{.Heading}}:
{{- range .Items}}
- {{.Description}}{{if .Scope}} [{{.Scope}}]{{end}}{{if .Ref}} {{.Ref}}{{end}}{{if .Author}} by @{{.Author}}{{end}}{{if .Significance}} ({{.Significance}}){{end}}{{range .JiraIssues}} [{{.Key}}: {{.Summary}}{{if .Status}} ({{.Status}}){{end}}{{if .Priority}} priority:{{.Priority}}{{end}}{{if .Type}} type:{{.Type}}{{end}}{{if .EpicKey}} epic:{{.EpicKey}}{{end}}{{if .Components}} components:{{range .Components}}{{.}},{{end}}{{end}}{{if .FixVersions}} fixVersion:{{range .FixVersions}}{{.}},{{end}}{{end}}{{if .Labels}} labels:{{range .Labels}}{{.}},{{end}}{{end}}{{if .Description}} desc:{{.Description}}{{end}}{{end}}
{{- end}}
{{- range .Scopes}}
  {{.Name}}:
  {{- range .Items}}
  - {{.Description}}{{if .Ref}} {{.Ref}}{{end}}{{if .Author}} by @{{.Author}}{{end}}{{range .JiraIssues}} [{{.Key}}: {{.Summary}}{{if .Status}} ({{.Status}}){{end}}{{if .Priority}} ({{.Priority}}){{end}}{{if .EpicKey}} epic:{{.EpicKey}}{{end}}{{if .Components}} components:{{range .Components}}{{.}},{{end}}{{end}}{{end}}
  {{- end}}
{{- end}}
{{- end}}

{{if .ChunkTotal}}Generate the release notes for these changes only, in the specified tone. Do not include a main heading or version number — this is a section of a larger document.{{else}}Generate the release notes now in the specified tone.{{end}}`
