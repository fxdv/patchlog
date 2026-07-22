// Package safehtml is the sole boundary for inserting untrusted text into HTML.
package safehtml

import "html"

// Text escapes text for both HTML text nodes and quoted attributes.
func Text(value string) string {
	return html.EscapeString(value)
}
