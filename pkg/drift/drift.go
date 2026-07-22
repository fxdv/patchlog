// Package drift compares planned Jira tickets against delivered commits to measure plan-vs-actual gap.
package drift

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/jira"
)

type Report struct {
	PlannedNotDelivered []*jira.Issue
	DeliveredNotPlanned []*jira.Issue
	Slipped             []*jira.Issue
	DeliveryRate        float64
	ScopeCreepRate      float64
	SlipRate            float64
}

func Analyze(ctx context.Context, client *jira.Client, projectKey, fixVersion string, deliveredKeys []string) (*Report, error) {
	planned, err := client.SearchByFixVersion(ctx, projectKey, fixVersion)
	if err != nil {
		return nil, fmt.Errorf("searching fix version: %w", err)
	}

	plannedMap := make(map[string]bool)
	for _, p := range planned {
		plannedMap[p.Key] = true
	}

	deliveredMap := make(map[string]bool)
	for _, k := range deliveredKeys {
		deliveredMap[k] = true
	}

	var r Report

	for _, p := range planned {
		if !deliveredMap[p.Key] {
			r.PlannedNotDelivered = append(r.PlannedNotDelivered, p)
		}
	}

	for _, k := range deliveredKeys {
		if !plannedMap[k] {
			issue, err := client.FetchIssue(ctx, k)
			if err == nil && issue != nil {
				r.DeliveredNotPlanned = append(r.DeliveredNotPlanned, issue)
			} else {
				r.DeliveredNotPlanned = append(r.DeliveredNotPlanned, &jira.Issue{Key: k})
			}
		}
	}

	delivered := len(deliveredKeys)
	if delivered > 0 {
		r.DeliveryRate = float64(delivered-len(r.PlannedNotDelivered)) / float64(len(planned)) * 100
		if len(planned) > 0 {
			r.DeliveryRate = float64(delivered-len(r.DeliveredNotPlanned)) / float64(len(planned)) * 100
		}
		r.ScopeCreepRate = float64(len(r.DeliveredNotPlanned)) / float64(delivered) * 100
	}
	if len(planned) > 0 {
		r.SlipRate = float64(len(r.Slipped)) / float64(len(planned)) * 100
	}

	sortIssues(r.PlannedNotDelivered)
	sortIssues(r.DeliveredNotPlanned)

	return &r, nil
}

func sortIssues(issues []*jira.Issue) {
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Key < issues[j].Key
	})
}

func FormatMarkdown(r *Report) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder

	sb.WriteString("## Plan vs Actual\n\n")

	if len(r.PlannedNotDelivered) > 0 {
		sb.WriteString(fmt.Sprintf("### Planned but not delivered (%d)\n\n", len(r.PlannedNotDelivered)))
		sb.WriteString("| Ticket | Summary | Priority | Status |\n|--------|---------|----------|--------|\n")
		for _, p := range r.PlannedNotDelivered {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", linkKey(p), p.Summary, p.Priority, p.Status))
		}
		sb.WriteString("\n")
	}

	if len(r.DeliveredNotPlanned) > 0 {
		sb.WriteString(fmt.Sprintf("### Delivered but not planned (%d)\n\n", len(r.DeliveredNotPlanned)))
		sb.WriteString("| Ticket | Summary | Priority | Status |\n|--------|---------|----------|--------|\n")
		for _, p := range r.DeliveredNotPlanned {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", linkKey(p), p.Summary, p.Priority, p.Status))
		}
		sb.WriteString("\n")
	}

	if len(r.PlannedNotDelivered) == 0 && len(r.DeliveredNotPlanned) == 0 {
		sb.WriteString("All planned tickets were delivered. No scope creep detected.\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Delivery rate:** %.0f%%  \n", r.DeliveryRate))
	sb.WriteString(fmt.Sprintf("**Scope creep:** %.0f%%  \n", r.ScopeCreepRate))
	if r.SlipRate > 0 {
		sb.WriteString(fmt.Sprintf("**Slip rate:** %.0f%%\n", r.SlipRate))
	}

	return sb.String()
}

func linkKey(issue *jira.Issue) string {
	if issue.URL != "" {
		return fmt.Sprintf("[%s](%s)", issue.Key, issue.URL)
	}
	return issue.Key
}
