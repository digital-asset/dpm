// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST sphinx-tabs syntax:
//
//	.. tabs::
//
//	   .. tab:: First Tab
//
//	      Content for the first tab.
//
//	   .. tab:: Second Tab
//
//	      Content for the second tab.
//
// Mintlify Tabs equivalent (https://www.mintlify.com/docs/components/tabs):
//
//	<Tabs>
//	  <Tab title="First Tab">
//	    Content for the first tab.
//	  </Tab>
//	  <Tab title="Second Tab">
//	    Content for the second tab.
//	  </Tab>
//	</Tabs>
//
// We run this transform near the top of the pipeline so the body of
// each tab still flows through every downstream transform (headings,
// code blocks, admonitions, etc.) as if it had been written inline.

var (
	reTabsStart = regexp.MustCompile(`^(\s*)\.\.\s+tabs::\s*$`)
	reTabStart  = regexp.MustCompile(`^(\s*)\.\.\s+tab::\s+(.+?)\s*$`)
)

// convertTabs walks the input and rewrites `.. tabs::` blocks into
// Mintlify `<Tabs><Tab>...</Tab></Tabs>` JSX. Any nested `.. tab::`
// directives inside `.. tabs::` become individual tab panels.
func convertTabs(s string) string {
	lines := strings.Split(s, "\n")
	var out []string

	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reTabsStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		i++

		// Collect indented body until the first line that returns to
		// or below the directive's indent.
		var body []string
		for i < len(lines) {
			cur := lines[i]
			if strings.TrimSpace(cur) == "" {
				body = append(body, "")
				i++
				continue
			}
			if !deeperIndent(cur, indent) {
				break
			}
			body = append(body, cur)
			i++
		}

		out = append(out, indent+"<Tabs>")
		out = append(out, parseTabs(body, indent)...)
		out = append(out, indent+"</Tabs>")
	}
	return strings.Join(out, "\n")
}

// parseTabs walks a `.. tabs::` body and emits `<Tab>` blocks for each
// nested `.. tab::` directive, dedenting tab content so it sits at
// column 0 of the tab body (Mintlify renders MDX inside `<Tab>` cleanly
// when the content isn't indented).
func parseTabs(body []string, parentIndent string) []string {
	var out []string
	i := 0
	for i < len(body) {
		line := body[i]
		m := reTabStart.FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}
		tabIndent := m[1]
		title := strings.TrimSpace(m[2])
		i++

		// Skip blank lines after the `.. tab::` directive.
		for i < len(body) && strings.TrimSpace(body[i]) == "" {
			i++
		}

		// Collect tab content: everything indented further than the
		// `.. tab::` directive itself.
		var content []string
		for i < len(body) {
			cur := body[i]
			if strings.TrimSpace(cur) == "" {
				content = append(content, "")
				i++
				continue
			}
			if !deeperIndent(cur, tabIndent) {
				break
			}
			content = append(content, cur)
			i++
		}
		// Trim trailing blank lines.
		for len(content) > 0 && strings.TrimSpace(content[len(content)-1]) == "" {
			content = content[:len(content)-1]
		}
		// Dedent to remove the tab's content indent so the body sits
		// neatly under <Tab>.
		dedented := dedentLines(content)

		out = append(out, parentIndent+`  <Tab title="`+escapeAttr(title)+`">`)
		for _, c := range dedented {
			if c == "" {
				out = append(out, "")
			} else {
				out = append(out, parentIndent+"    "+c)
			}
		}
		out = append(out, parentIndent+`  </Tab>`)
	}
	return out
}

// dedentLines strips the minimum common leading-whitespace run from
// every non-blank line. Used to normalize tab-content indentation.
func dedentLines(lines []string) []string {
	min := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(leadingWS(line))
		if min == -1 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= min {
			out[i] = line[min:]
		} else {
			out[i] = line
		}
	}
	return out
}
