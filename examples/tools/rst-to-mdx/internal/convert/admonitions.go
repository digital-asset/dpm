// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST admonitions wrap short callouts. Mintlify has Note/Tip/Warning/Info
// JSX components that render as colored boxes. The mapping (per the
// migration guide):
//
//	.. note::        → <Note>
//	.. attention::   → <Note>
//	.. tip::         → <Tip>
//	.. hint::        → <Tip>
//	.. warning::     → <Warning>
//	.. caution::     → <Warning>
//	.. danger::      → <Warning>  (prefixed "**Danger:** ")
//	.. important::   → <Warning>  (prefixed "**Important:** ")
//	.. seealso::     → <Info>
//	.. deprecated::  → <Warning>  (version note prefix)
//	.. versionadded:: → <Note>    (version note prefix)
//	.. versionchanged:: → <Note>  (version note prefix)
//
// Some admonitions have content on the same line (inline) and some have
// an indented block that follows. We handle both.

type admonitionSpec struct {
	kind    string // RST directive name, without ::
	tag     string // JSX component name (Note, Tip, Warning, Info)
	prefix  string // optional body prefix like "**Important:** "
	argBold bool   // if true, emit the directive argument as bold prefix
}

var admonitions = []admonitionSpec{
	{kind: "note", tag: "Note"},
	{kind: "attention", tag: "Note"},
	{kind: "tip", tag: "Tip"},
	{kind: "hint", tag: "Tip"},
	{kind: "warning", tag: "Warning"},
	{kind: "caution", tag: "Warning"},
	{kind: "danger", tag: "Warning", prefix: "**Danger:** "},
	{kind: "important", tag: "Warning", prefix: "**Important:** "},
	{kind: "seealso", tag: "Info"},
	{kind: "deprecated", tag: "Warning", argBold: true},
	{kind: "versionadded", tag: "Note", argBold: true},
	{kind: "versionchanged", tag: "Note", argBold: true},
}

// convertAdmonitions walks the input line-by-line and rewrites RST
// admonition directives into JSX component blocks.
func convertAdmonitions(s string) string {
	lines := strings.Split(s, "\n")
	var out []string

	// Precompile the directive matchers for speed and to capture the
	// indent + optional argument.
	matchers := make(map[string]*regexp.Regexp, len(admonitions))
	for _, a := range admonitions {
		matchers[a.kind] = regexp.MustCompile(
			`^(\s*)\.\.\s+` + regexp.QuoteMeta(a.kind) + `::\s*(.*)$`)
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		matched := false
		for _, a := range admonitions {
			m := matchers[a.kind].FindStringSubmatch(line)
			if m == nil {
				continue
			}
			indent := m[1]
			arg := strings.TrimSpace(m[2])
			i++

			body, consumed := collectAdmonitionBody(lines[i:], indent, arg, a)
			i += consumed

			out = append(out, indent+"<"+a.tag+">")
			out = append(out, reindentAdmonitionBody(body, indent)...)
			out = append(out, indent+"</"+a.tag+">")
			matched = true
			break
		}
		if matched {
			continue
		}
		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

// collectAdmonitionBody gathers the indented content (or inline arg) of
// an admonition. It returns the body lines, dedented and with the
// configured prefix applied, plus how many input lines were consumed.
func collectAdmonitionBody(lines []string, parentIndent, arg string, a admonitionSpec) ([]string, int) {
	// Inline form: `.. note:: some text` puts the content on the same
	// line; no indented block follows.
	if arg != "" {
		body := arg
		switch {
		case a.argBold:
			body = "**" + a.prefix + capitalize(a.kind) + " " + arg + ":** "
			// The argument is usually a version; any text that
			// follows it lives in the indented block below.
		case a.prefix != "":
			body = a.prefix + arg
		}
		// Check whether there's ALSO an indented block; if so, merge.
		indented, consumed := consumeIndentedBlock(lines, parentIndent)
		if len(indented) == 0 {
			return []string{body}, consumed
		}
		var combined []string
		combined = append(combined, body)
		combined = append(combined, indented...)
		return combined, consumed
	}

	// Block form: indented lines after a blank separator.
	// Skip blank separator(s).
	skip := 0
	for skip < len(lines) && strings.TrimSpace(lines[skip]) == "" {
		skip++
	}
	body, consumed := consumeIndentedBlock(lines[skip:], parentIndent)
	if a.prefix != "" && len(body) > 0 {
		body[0] = a.prefix + body[0]
	}
	return body, skip + consumed
}

// reGenericAdmonition matches `.. admonition:: Title text`.
var reGenericAdmonition = regexp.MustCompile(
	`^(\s*)\.\.\s+admonition::\s*(.+)$`)

// convertGenericAdmonition handles `.. admonition:: Title` — the
// catch-all RST admonition that doesn't map to a specific Mintlify
// component. Emits the title as bold text followed by the body.
func convertGenericAdmonition(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		m := reGenericAdmonition.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		indent := m[1]
		title := strings.TrimSpace(m[2])
		i++

		skip := 0
		for skip < len(lines[i:]) && strings.TrimSpace(lines[i+skip]) == "" {
			skip++
		}
		body, consumed := consumeIndentedBlock(lines[i+skip:], indent)
		i += skip + consumed

		out = append(out, "")
		out = append(out, indent+"**"+title+"**")
		if len(body) > 0 {
			out = append(out, "")
			out = append(out, body...)
		}
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// reindentAdmonitionBody prefixes every non-blank body line with
// indent so the content sits inside the JSX tags at the same
// indentation level. Without this, an admonition nested inside a list
// item would have its body at column 0 while the <Note>/<Warning>
// tags are indented, causing MDX to close the component prematurely.
func reindentAdmonitionBody(body []string, indent string) []string {
	if indent == "" {
		return body
	}
	out := make([]string, len(body))
	for i, line := range body {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
		} else {
			out[i] = indent + line
		}
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
