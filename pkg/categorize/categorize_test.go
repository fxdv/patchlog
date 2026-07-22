package categorize

import (
	"testing"

	"github.com/fxdv/patchlog/pkg/commit"
	"github.com/fxdv/patchlog/pkg/render"
)

func TestByTypeBasicGrouping(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add feature", Scope: "api", Breaking: false, Significance: "minor", JiraKeys: []string{"PROJ-1"}},
		{Type: "fix", Header: "fix bug", Breaking: false, Significance: "patch"},
		{Type: "feat", Header: "add another", Breaking: false, Significance: "minor"},
	}
	report := ByType(commits, nil)

	if len(report.Breaking) > 0 {
		t.Error("should have no breaking changes")
	}

	sectionMap := map[string]*render.Section{}
	for i := range report.Sections {
		sectionMap[report.Sections[i].Heading] = &report.Sections[i]
	}

	features, ok := sectionMap["Features"]
	if !ok {
		t.Fatal("missing Features section")
	}
	totalItems := len(features.Items)
	for _, sg := range features.Scopes {
		totalItems += len(sg.Items)
	}
	if totalItems != 2 {
		t.Errorf("expected 2 feature items, got %d", totalItems)
	}

	bugfixes, ok := sectionMap["Bug Fixes"]
	if !ok {
		t.Fatal("missing Bug Fixes section")
	}
	if len(bugfixes.Items)+len(bugfixes.Scopes) == 0 {
		t.Error("expected bug fix items")
	}
}

func TestByTypeBreakingChanges(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "breaking thing", Breaking: true, Significance: "major"},
		{Type: "feat", Header: "normal thing", Breaking: false, Significance: "minor"},
	}
	report := ByType(commits, nil)

	if len(report.Breaking) != 1 {
		t.Fatalf("expected 1 breaking item, got %d", len(report.Breaking))
	}
	if report.Breaking[0].Description != "breaking thing" {
		t.Errorf("unexpected breaking description: %s", report.Breaking[0].Description)
	}
}

func TestByTypeScopeGrouping(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add a", Scope: "api", Breaking: false},
		{Type: "feat", Header: "add b", Scope: "api", Breaking: false},
		{Type: "feat", Header: "add c", Scope: "ui", Breaking: false},
	}
	report := ByType(commits, nil)

	var features *render.Section
	for i := range report.Sections {
		if report.Sections[i].Heading == "Features" {
			features = &report.Sections[i]
			break
		}
	}
	if features == nil {
		t.Fatal("missing Features section")
	}

	scopeMap := map[string]bool{}
	for _, sg := range features.Scopes {
		scopeMap[sg.Name] = true
		if sg.Name == "api" && len(sg.Items) != 2 {
			t.Errorf("api scope: expected 2 items, got %d", len(sg.Items))
		}
		if sg.Name == "ui" && len(sg.Items) != 1 {
			t.Errorf("ui scope: expected 1 item, got %d", len(sg.Items))
		}
	}
	if !scopeMap["api"] || !scopeMap["ui"] {
		t.Errorf("expected api and ui scopes, got %v", scopeMap)
	}
}

func TestByTypeCustomSectionMap(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add thing", Breaking: false},
	}
	sectionMap := map[string]string{"feat": "New Stuff"}
	report := ByType(commits, sectionMap)

	if len(report.Sections) == 0 {
		t.Fatal("no sections")
	}
	if report.Sections[0].Heading != "New Stuff" {
		t.Errorf("expected 'New Stuff', got %q", report.Sections[0].Heading)
	}
}

func TestByTypeNonConventional(t *testing.T) {
	commits := []commit.Commit{
		{Type: "other", Header: "something", RawHeader: "something random", Breaking: false},
	}
	report := ByType(commits, nil)

	if len(report.Sections) == 0 {
		t.Fatal("no sections")
	}
	found := false
	for _, s := range report.Sections {
		if s.Heading == "Uncategorised" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Uncategorised section for 'other' type")
	}
}

func TestByTypeRefExtraction(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add thing (#42)", RawHeader: "feat: add thing (#42)", Breaking: false},
	}
	report := ByType(commits, nil)

	if len(report.Sections) == 0 {
		t.Fatal("no sections")
	}
	items := report.Sections[0].Items
	if len(items) == 0 {
		t.Fatal("no items")
	}
	if items[0].Ref != "#42" {
		t.Errorf("expected ref #42, got %q", items[0].Ref)
	}
}

func TestByTypeEmptyCommits(t *testing.T) {
	report := ByType(nil, nil)
	if len(report.Breaking) != 0 {
		t.Error("expected no breaking changes")
	}
	if len(report.Sections) != 0 {
		t.Error("expected no sections for empty input")
	}
}

func TestByTypeJiraKeysCarried(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add PROJ-123", Breaking: false, JiraKeys: []string{"PROJ-123"}},
	}
	report := ByType(commits, nil)

	if len(report.Sections) == 0 || len(report.Sections[0].Items) == 0 {
		t.Fatal("no items")
	}
	keys := report.Sections[0].Items[0].JiraKeys
	if len(keys) != 1 || keys[0] != "PROJ-123" {
		t.Errorf("expected JiraKeys [PROJ-123], got %v", keys)
	}
}

func TestByTypeSignificanceCarried(t *testing.T) {
	commits := []commit.Commit{
		{Type: "feat", Header: "add thing", Breaking: false, Significance: "minor"},
	}
	report := ByType(commits, nil)

	if len(report.Sections) == 0 || len(report.Sections[0].Items) == 0 {
		t.Fatal("no items")
	}
	if report.Sections[0].Items[0].Significance != "minor" {
		t.Errorf("expected significance minor, got %q", report.Sections[0].Items[0].Significance)
	}
}
