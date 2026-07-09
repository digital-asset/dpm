// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST comments come in two forms:
//
//	.. Single line comment
//
//	..
//	   Multi-line comment
//	   indented under the dots.
//
// This transform runs near the end of the pipeline, AFTER directive
// stripping (so we don't match a directive's `::` prefix) and admonition
// conversion (so `.. note::` etc. are already gone). That leaves only
// true comments by the time we get here.
//
// Any `.. <word>::` that survived is an unknown directive. We leave those
// untouched so a reader can spot them and fix the converter.

var (
	// `.. something` where "something" doesn't look like a directive
	// head (directive heads end in `::`). Go's regexp doesn't support
	// negative lookahead, so we detect directives with a separate
	// pattern and skip.
	reCommentLead    = regexp.MustCompile(`^\.\.\s+(.+)$`)
	reDirectiveLead  = regexp.MustCompile(`^\.\.\s+[A-Za-z][A-Za-z0-9_\-]*::`)
	reMultiCommentHd = regexp.MustCompile(`^\.\.\s*$`)
	// `.. [N] body` is RST footnote syntax — the body should render
	// as visible text, not get hidden as a comment. The `N` can be a
	// number, `*`, `#`, or a name.
	reFootnote = regexp.MustCompile(`^\.\.\s+\[([^\]]+)\]\s+(.+)$`)
)

func convertComments(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]

		// Multi-line: `..` on its own, then indented body.
		// RST's rule: the comment body continues until a line returns
		// to or below the column where `..` sits. Blank lines do NOT
		// terminate the body — they're part of it as long as content
		// at body-indent or deeper resumes after them.
		if reMultiCommentHd.MatchString(line) {
			parentIndent := leadingWS(line)
			i++
			indent := ""
			var body []string
			for i < len(lines) {
				cur := lines[i]
				if strings.TrimSpace(cur) == "" {
					// Look ahead past consecutive blank lines.
					j := i + 1
					for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
						j++
					}
					if j >= len(lines) {
						// File ends inside the comment.
						break
					}
					nextIndent := leadingWS(lines[j])
					// If the next non-blank line returns to or below
					// the directive's parent indent, the comment ends.
					if len(nextIndent) <= len(parentIndent) {
						break
					}
					// Still inside the comment — keep the blank line
					// in the body.
					body = append(body, "")
					i++
					continue
				}
				lws := leadingWS(cur)
				if indent == "" {
					indent = lws
				}
				// A line with strictly less indent than the body
				// indent ends the comment.
				if len(lws) < len(indent) {
					break
				}
				body = append(body, strings.TrimPrefix(cur, indent))
				i++
			}
			joined := strings.Join(body, "\n")
			joined = sanitizeCommentBody(joined)
			if strings.Contains(joined, "\n") {
				out = append(out, "{/*\n"+joined+"\n*/}")
			} else {
				out = append(out, "{/* "+joined+" */}")
			}
			continue
		}

		// `.. [N] body`: RST footnote, render the body as visible text
		// with a bracketed marker so the reader still sees the
		// reference. Must come before the generic comment branch.
		if m := reFootnote.FindStringSubmatch(line); m != nil {
			out = append(out, "["+m[1]+"] "+m[2])
			i++
			continue
		}

		// Single-line `.. text`: only if NOT a directive head.
		if m := reCommentLead.FindStringSubmatch(line); m != nil {
			if !reDirectiveLead.MatchString(line) {
				out = append(out, "{/* "+m[1]+" */}")
				i++
				continue
			}
		}

		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

// sanitizeCommentBody escapes `*/` sequences so they don't terminate
// the surrounding `{/* ... */}` MDX comment early. We replace each
// `*/` with `*\/` which is invisible to JSX but breaks the close
// pattern. Comments are rendering-only so the visual result is the
// same.
func sanitizeCommentBody(s string) string {
	return strings.ReplaceAll(s, "*/", "*\\/")
}
