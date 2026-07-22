// Package render provides markdown, JSON, and changelog output rendering
// for release reports.
package render

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// Report holds all data for a release report: version, sections, breaking
// changes, dependencies, and link templates.
type Report struct {
	Version           string
	Date              string
	Breaking          []Item
	Sections          []Section
	Dependencies      []DependencyChange
	CompareURL        string
	ShowAuthor        bool
	Emojis            bool
	CommitURLTemplate string
	IssueURLTemplate  string
	Repo              string
}

func (r Report) CommitURL(hash string) string {
	if hash == "" || r.CommitURLTemplate == "" || r.Repo == "" {
		return ""
	}
	if strings.Contains(r.CommitURLTemplate, "%s") {
		return fmt.Sprintf(r.CommitURLTemplate, r.Repo, hash)
	}
	return r.CommitURLTemplate
}

func (r Report) IssueURL(ref string) string {
	if ref == "" || r.IssueURLTemplate == "" || r.Repo == "" {
		return ""
	}
	ref = strings.TrimPrefix(ref, "#")
	if strings.Contains(r.IssueURLTemplate, "%s") {
		return fmt.Sprintf(r.IssueURLTemplate, r.Repo, ref)
	}
	return r.IssueURLTemplate
}

func (r Report) ForEachItem(fn func(item *Item)) {
	for i := range r.Breaking {
		fn(&r.Breaking[i])
	}
	for s := range r.Sections {
		for i := range r.Sections[s].Items {
			fn(&r.Sections[s].Items[i])
		}
		for sg := range r.Sections[s].Scopes {
			for i := range r.Sections[s].Scopes[sg].Items {
				fn(&r.Sections[s].Scopes[sg].Items[i])
			}
		}
	}
}

func (r Report) ItemCount() int {
	n := len(r.Breaking)
	for _, s := range r.Sections {
		n += len(s.Items)
		for _, sg := range s.Scopes {
			n += len(sg.Items)
		}
	}
	return n
}

// Section groups commits by conventional commit type.
type Section struct {
	Heading string
	Type    string
	Items   []Item
	Scopes  []ScopeGroup
}

type ScopeGroup struct {
	Name  string
	Items []Item
}

// Item represents a single changelog entry parsed from a commit.
type Item struct {
	Description  string
	Scope        string
	Author       string
	Ref          string
	Breaking     bool
	Hash         string
	Significance string
	JiraKeys     []string
	JiraIssues   []*JiraInfo
}

type JiraInfo struct {
	Key         string
	Summary     string
	Priority    string
	Status      string
	URL         string
	Labels      []string
	Type        string
	EpicKey     string
	FixVersions []string
	Components  []string
	Description string
	Assignee    string
}

// DependencyChange represents a detected dependency version bump.
type DependencyChange struct {
	Name         string `json:"name"`
	OldVersion   string `json:"old_version"`
	NewVersion   string `json:"new_version"`
	Ecosystem    string `json:"ecosystem"`
	Manifest     string `json:"manifest"`
	Changelog    string `json:"changelog,omitempty"`
	ChangelogURL string `json:"changelog_url,omitempty"`
}

func Markdown(report Report) ([]byte, error) {
	var buf bytes.Buffer
	useEmoji := report.Emojis

	if report.Version != "" {
		fmt.Fprintf(&buf, "# %s", report.Version)
	}
	if report.Date != "" {
		fmt.Fprintf(&buf, " (%s)", report.Date)
	} else {
		fmt.Fprintf(&buf, " (%s)", time.Now().Format("2006-01-02"))
	}
	buf.WriteString("\n\n")

	if report.CompareURL != "" {
		fmt.Fprintf(&buf, "[Full Changelog](%s)\n\n", report.CompareURL)
	}

	if len(report.Breaking) > 0 {
		fmt.Fprintf(&buf, "## %s\n\n", breakingHeading(useEmoji))
		for _, item := range report.Breaking {
			writeItem(&buf, item, report.ShowAuthor)
		}
		buf.WriteByte('\n')
	}

	for _, section := range report.Sections {
		if len(section.Items) == 0 && len(section.Scopes) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "## %s\n\n", PrettyHeading(section.Heading, section.Type, useEmoji))

		for _, item := range section.Items {
			writeItem(&buf, item, report.ShowAuthor)
		}

		for _, sg := range section.Scopes {
			fmt.Fprintf(&buf, "### %s\n\n", sg.Name)
			for _, item := range sg.Items {
				writeScopedItem(&buf, item, report.ShowAuthor)
			}
		}
		buf.WriteByte('\n')
	}

	if len(report.Dependencies) > 0 {
		fmt.Fprintf(&buf, "## %s\n\n", dependenciesHeading(useEmoji))
		for _, dep := range report.Dependencies {
			writeDependency(&buf, dep)
		}
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func writeItem(buf *bytes.Buffer, item Item, showAuthor bool) {
	buf.WriteString("- ")
	if item.Scope != "" {
		fmt.Fprintf(buf, "**%s**: ", item.Scope)
	}
	writeItemBody(buf, item, showAuthor)
}

func writeScopedItem(buf *bytes.Buffer, item Item, showAuthor bool) {
	buf.WriteString("- ")
	writeItemBody(buf, item, showAuthor)
}

func writeItemBody(buf *bytes.Buffer, item Item, showAuthor bool) {
	buf.WriteString(item.Description)
	if item.Ref != "" {
		fmt.Fprintf(buf, " (%s)", item.Ref)
	}
	for _, j := range item.JiraIssues {
		if j.URL != "" {
			fmt.Fprintf(buf, " [%s](%s)", j.Key, j.URL)
		} else {
			fmt.Fprintf(buf, " [%s]", j.Key)
		}
		if j.Summary != "" {
			fmt.Fprintf(buf, " %s", j.Summary)
		}
		if j.Status != "" {
			fmt.Fprintf(buf, " `%s`", j.Status)
		}
		if len(j.Components) > 0 {
			fmt.Fprintf(buf, " (%s)", strings.Join(j.Components, ", "))
		}
		if len(j.FixVersions) > 0 {
			fmt.Fprintf(buf, " → %s", strings.Join(j.FixVersions, ", "))
		}
	}
	if item.Author != "" && showAuthor {
		fmt.Fprintf(buf, " by @%s", item.Author)
	}
	buf.WriteByte('\n')
}

func dependenciesHeading(useEmoji bool) string {
	if useEmoji {
		return "📦 Dependencies"
	}
	return "Dependencies"
}

func writeDependency(buf *bytes.Buffer, dep DependencyChange) {
	arrow := dep.NewVersion
	if dep.OldVersion != "" {
		arrow = fmt.Sprintf("%s → %s", dep.OldVersion, dep.NewVersion)
	}
	fmt.Fprintf(buf, "### %s %s\n\n", dep.Name, arrow)
	if dep.Ecosystem != "" || dep.Manifest != "" {
		var meta []string
		if dep.Ecosystem != "" {
			meta = append(meta, fmt.Sprintf("ecosystem: %s", dep.Ecosystem))
		}
		if dep.Manifest != "" {
			meta = append(meta, fmt.Sprintf("manifest: %s", dep.Manifest))
		}
		fmt.Fprintf(buf, "_%s_\n\n", strings.Join(meta, " · "))
	}
	if dep.Changelog != "" {
		buf.WriteString("<details>\n<summary>Upstream changes</summary>\n\n")
		buf.WriteString(dep.Changelog)
		buf.WriteString("\n\n</details>\n\n")
	} else if dep.ChangelogURL != "" {
		fmt.Fprintf(buf, "[Upstream](%s)\n\n", dep.ChangelogURL)
	}
}
