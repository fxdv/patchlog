package drift

import (
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/jira"
)

func TestFormatMarkdownEmpty(t *testing.T) {
	if FormatMarkdown(nil) != "" {
		t.Error("nil report should produce empty string")
	}
}

func TestFormatMarkdownNoDrift(t *testing.T) {
	r := &Report{
		DeliveryRate:   100,
		ScopeCreepRate: 0,
	}
	out := FormatMarkdown(r)
	if !strings.Contains(out, "All planned tickets were delivered") {
		t.Error("should contain no-drift message")
	}
	if !strings.Contains(out, "100%") {
		t.Error("should contain delivery rate")
	}
}

func TestFormatMarkdownWithDrift(t *testing.T) {
	r := &Report{
		PlannedNotDelivered: []*jira.Issue{
			{Key: "KEXP-350", Summary: "Add SSO", Priority: "High", Status: "Open", URL: "https://example.com/KEXP-350"},
		},
		DeliveredNotPlanned: []*jira.Issue{
			{Key: "KEXP-420", Summary: "Fix sorting", Priority: "Low", Status: "Closed", URL: "https://example.com/KEXP-420"},
		},
		DeliveryRate:   85,
		ScopeCreepRate: 5,
	}
	out := FormatMarkdown(r)
	if !strings.Contains(out, "Planned but not delivered") {
		t.Error("should contain planned-not-delivered section")
	}
	if !strings.Contains(out, "KEXP-350") {
		t.Error("should contain ticket key")
	}
	if !strings.Contains(out, "Delivered but not planned") {
		t.Error("should contain delivered-not-planned section")
	}
	if !strings.Contains(out, "85%") {
		t.Error("should contain delivery rate")
	}
}

func TestSortIssues(t *testing.T) {
	issues := []*jira.Issue{
		{Key: "KEXP-300"},
		{Key: "KEXP-100"},
		{Key: "KEXP-200"},
	}
	sortIssues(issues)
	if issues[0].Key != "KEXP-100" {
		t.Error("should sort by key ascending")
	}
}

func TestLinkKey(t *testing.T) {
	if linkKey(&jira.Issue{Key: "KEXP-1"}) != "KEXP-1" {
		t.Error("plain key without URL")
	}
	if !strings.Contains(linkKey(&jira.Issue{Key: "KEXP-1", URL: "https://example.com"}), "https://example.com") {
		t.Error("should contain URL")
	}
}
