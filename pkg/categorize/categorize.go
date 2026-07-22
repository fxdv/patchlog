package categorize

import (
	"regexp"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/render"
)

type sectionDef struct {
	Type    string
	Heading string
}

var defaultSections = []sectionDef{
	{"feat", "Features"},
	{"fix", "Bug Fixes"},
	{"perf", "Performance Improvements"},
	{"refactor", "Code Refactoring"},
	{"revert", "Reverts"},
	{"docs", "Documentation"},
	{"test", "Tests"},
	{"style", "Style / Formatting"},
	{"ci", "CI / Build"},
	{"chore", "Chores"},
	{"other", "Uncategorised"},
}

var refRe = regexp.MustCompile(`\(#(\d+)\)`)

func ByType(commits []commit.Commit, sectionMap map[string]string) render.Report {
	sectionHeading := make(map[string]string, len(defaultSections))
	for _, sd := range defaultSections {
		heading := sd.Heading
		if h, ok := sectionMap[sd.Type]; ok {
			heading = h
		}
		sectionHeading[sd.Type] = heading
	}

	customTypes := make(map[string]bool)
	for t := range sectionMap {
		if _, ok := sectionHeading[t]; !ok {
			sectionHeading[t] = sectionMap[t]
			customTypes[t] = true
		}
	}

	type bucket struct {
		items []render.Item
	}
	buckets := make(map[string]*bucket)
	var breaking []render.Item

	for _, c := range commits {
		item := toItem(c)

		switch {
		case c.Breaking:
			breaking = append(breaking, item)
			continue
		case c.Type == "other":
			item.Description = c.RawHeader
		case !knownType(c.Type, customTypes):
			item.Description = c.RawHeader
			c.Type = "other"
		}

		if buckets[c.Type] == nil {
			buckets[c.Type] = &bucket{}
		}
		buckets[c.Type].items = append(buckets[c.Type].items, item)
	}

	var sectionOrder []string
	for _, sd := range defaultSections {
		sectionOrder = append(sectionOrder, sd.Type)
	}
	for t := range customTypes {
		sectionOrder = append(sectionOrder, t)
	}

	var sections []render.Section
	seen := make(map[string]bool)
	for _, typ := range sectionOrder {
		if seen[typ] {
			continue
		}
		seen[typ] = true
		b, ok := buckets[typ]
		if !ok || len(b.items) == 0 {
			continue
		}

		sect := render.Section{Heading: sectionHeading[typ], Type: typ}
		scopeGroups := make(map[string][]render.Item)
		var unscoped []render.Item
		for _, item := range b.items {
			if item.Scope != "" {
				scopeGroups[item.Scope] = append(scopeGroups[item.Scope], item)
			} else {
				unscoped = append(unscoped, item)
			}
		}

		sect.Items = unscoped
		var scopes []string
		for s := range scopeGroups {
			scopes = append(scopes, s)
		}
		sort.Strings(scopes)
		for _, s := range scopes {
			sect.Scopes = append(sect.Scopes, render.ScopeGroup{
				Name:  s,
				Items: scopeGroups[s],
			})
		}
		sections = append(sections, sect)
	}

	return render.Report{
		Breaking: breaking,
		Sections: sections,
	}
}

func knownType(typ string, customTypes map[string]bool) bool {
	for _, sd := range defaultSections {
		if sd.Type == typ {
			return true
		}
	}
	return customTypes[typ]
}

func toItem(c commit.Commit) render.Item {
	ref := extractRef(c.RawHeader)
	if ref == "" {
		ref = extractRef(c.Body + "\n" + c.Footer)
	}

	desc := c.Header
	if ref != "" {
		desc = stripRef(desc, ref)
	}

	return render.Item{
		Description:  desc,
		Scope:        c.Scope,
		Author:       c.Author,
		Ref:          ref,
		Breaking:     c.Breaking,
		Hash:         c.Hash,
		Significance: c.Significance,
		JiraKeys:     c.JiraKeys,
	}
}

func extractRef(s string) string {
	matches := refRe.FindStringSubmatch(s)
	if matches != nil {
		return "#" + matches[1]
	}
	return ""
}

func stripRef(desc, ref string) string {
	tag := "(" + ref + ")"
	return strings.TrimSpace(strings.Replace(desc, tag, "", 1))
}
