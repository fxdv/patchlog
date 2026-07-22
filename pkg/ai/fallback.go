package ai

import (
	"bytes"
	"fmt"

	"github.com/fxdv/patchlog/pkg/render"
)

func generateTemplateFallback(report render.Report, tone Tone) string {
	var buf bytes.Buffer

	switch tone {
	case ToneDev:
		writeDevProse(&buf, report)
	case ToneCustomer:
		writeCustomerProse(&buf, report)
	case ToneExec:
		writeExecProse(&buf, report)
	}

	return buf.String()
}

func writeDevProse(buf *bytes.Buffer, report render.Report) {
	fmt.Fprintf(buf, "# %s", report.Version)
	if report.Date != "" {
		fmt.Fprintf(buf, " (%s)", report.Date)
	}
	buf.WriteString("\n\n")

	if len(report.Breaking) > 0 {
		buf.WriteString("## Breaking Changes\n\n")
		for _, item := range report.Breaking {
			fmt.Fprintf(buf, "- **%s**", item.Description)
			if item.Ref != "" {
				fmt.Fprintf(buf, " (%s)", item.Ref)
			}
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}

	for _, section := range report.Sections {
		if len(section.Items) == 0 && len(section.Scopes) == 0 {
			continue
		}
		fmt.Fprintf(buf, "## %s\n\n", section.Heading)
		for _, item := range section.Items {
			fmt.Fprintf(buf, "- %s", item.Description)
			if item.Ref != "" {
				fmt.Fprintf(buf, " (%s)", item.Ref)
			}
			buf.WriteByte('\n')
		}
		for _, sg := range section.Scopes {
			for _, item := range sg.Items {
				fmt.Fprintf(buf, "- **%s**: %s", sg.Name, item.Description)
				if item.Ref != "" {
					fmt.Fprintf(buf, " (%s)", item.Ref)
				}
				buf.WriteByte('\n')
			}
		}
		buf.WriteByte('\n')
	}
}

func writeCustomerProse(buf *bytes.Buffer, report render.Report) {
	fmt.Fprintf(buf, "# %s\n\n", report.Version)

	if len(report.Breaking) > 0 {
		buf.WriteString("## Important Changes\n\n")
		buf.WriteString("This release includes changes that may require your attention:\n\n")
		for _, item := range report.Breaking {
			fmt.Fprintf(buf, "- %s\n", item.Description)
		}
		buf.WriteByte('\n')
	}

	headingMap := map[string]string{
		"Features":                 "What's New",
		"Bug Fixes":                "Bug Fixes",
		"Performance Improvements": "Performance",
		"Code Refactoring":         "Improvements",
		"Documentation":            "Documentation",
		"Tests":                    "Testing",
		"Style / Formatting":       "",
		"CI / Build":               "",
		"Chores":                   "",
		"Uncategorised":            "Other Changes",
	}

	intros := map[string]string{
		"Features":                 "We're excited to share what's new in this release:\n\n",
		"Bug Fixes":                "We've squashed some bugs to make your experience smoother:\n\n",
		"Performance Improvements": "We've made things faster:\n\n",
		"Code Refactoring":         "We've improved the codebase under the hood:\n\n",
	}

	for _, s := range report.Sections {
		if len(s.Items) == 0 && len(s.Scopes) == 0 {
			continue
		}

		displayHeading, show := headingMap[s.Heading]
		if !show {
			continue
		}
		if displayHeading == "" {
			displayHeading = s.Heading
		}

		fmt.Fprintf(buf, "## %s\n\n", displayHeading)
		if intro, ok := intros[s.Heading]; ok {
			buf.WriteString(intro)
		}
		for _, item := range s.Items {
			fmt.Fprintf(buf, "- %s\n", item.Description)
		}
		for _, sg := range s.Scopes {
			for _, item := range sg.Items {
				fmt.Fprintf(buf, "- %s\n", item.Description)
			}
		}
		buf.WriteByte('\n')
	}

	buf.WriteString("---\n\n")
	buf.WriteString("Thank you for using our product! If you have any questions or feedback, please reach out.\n")
}

func writeExecProse(buf *bytes.Buffer, report render.Report) {
	fmt.Fprintf(buf, "# Executive Summary — %s\n\n", report.Version)

	featCount := 0
	fixCount := 0
	breakCount := len(report.Breaking)

	for _, s := range report.Sections {
		n := len(s.Items)
		for _, sg := range s.Scopes {
			n += len(sg.Items)
		}
		if s.Heading == "Features" {
			featCount += n
		} else if s.Heading == "Bug Fixes" {
			fixCount += n
		}
	}

	buf.WriteString("This release ")
	if featCount > 0 {
		fmt.Fprintf(buf, "introduces %d new feature", featCount)
		if featCount > 1 {
			buf.WriteByte('s')
		}
	}
	if featCount > 0 && fixCount > 0 {
		buf.WriteString(" and ")
	}
	if fixCount > 0 {
		fmt.Fprintf(buf, "addresses %d bug fix", fixCount)
		if fixCount > 1 {
			buf.WriteByte('s')
		}
	}
	buf.WriteString(". ")

	if breakCount > 0 {
		buf.WriteString("Please note that this release contains breaking changes that may require migration. ")
	}

	buf.WriteString("The focus has been on improving reliability and user experience.\n")
}
