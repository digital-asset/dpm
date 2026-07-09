// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST has three ways to introduce a code block:
//
//   1. `.. code-block:: <lang>` directive, optionally with options,
//      followed by a blank line, followed by indented content.
//   2. `.. code:: <lang>` — older alias used heavily in the Canton and
//      DPM docs.
//   3. `::` at the end of a line, introducing an indented literal block
//      with no language tag.
//
// We walk the document line-by-line so we can track the indent of the
// directive and consume all lines that are more indented than it. That's
// cheaper and more correct than trying to write one regex that respects
// Python-style indentation.

var (
	reCodeBlockDirective = regexp.MustCompile(
		`^(\s*)\.\.\s+(?:code-block|code|sourcecode|parsed-literal)::\s*([\w+\-]*)\s*$`)
	reDirectiveOption = regexp.MustCompile(
		`^(\s+):[A-Za-z][A-Za-z0-9_\-]*:[^\n]*$`)
	// A line ending in `::` (but not a role like `:code:`) introduces
	// a literal block. The capture group keeps the prefix so we can
	// emit it before the fenced block opens.
	reLiteralIntro = regexp.MustCompile(`^(.*[^:\s])::\s*$`)
	// A line that is JUST `::` on its own (or with leading whitespace)
	// also introduces a literal block — there's no prefix text to keep.
	reLiteralIntroSolo = regexp.MustCompile(`^(\s*)::\s*$`)
)

// convertCodeBlocks walks s line-by-line and rewrites `.. code-block:: lang`
// and `::` literal blocks as fenced ```lang``` blocks.
func convertCodeBlocks(s string) string {
	lines := strings.Split(s, "\n")
	var out []string

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Case 1+2: `.. code-block:: lang` or `.. code:: lang`.
		if m := reCodeBlockDirective.FindStringSubmatch(line); m != nil {
			indent := m[1]
			lang := strings.TrimSpace(m[2])
			i++

			// Skip directive option lines (`:linenos:` and so on).
			for i < len(lines) && reDirectiveOption.MatchString(lines[i]) {
				i++
			}
			// Skip blank line that separates options from content.
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}

			// Consume indented content.
			body, consumed := consumeIndentedBlock(lines[i:], indent)
			i += consumed

			out = append(out, "")
			out = append(out, indent+"```"+lang)
			out = append(out, reindentBlock(body, indent)...)
			out = append(out, indent+"```")
			out = append(out, "")
			continue
		}

		// Case 3: line ending in `::`. We skip lines that start with
		// `.. ` — those are unhandled directives that happen to end in
		// `::`, not literal-block introducers (e.g. `.. tabs::`,
		// `.. tab::`, custom Sphinx extensions we don't rewrite).
		trimmedLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmedLeft, ".. ") {
			out = append(out, line)
			i++
			continue
		}
		// 3a: standalone `::` on its own line.
		if m := reLiteralIntroSolo.FindStringSubmatch(line); m != nil {
			directiveIndent := m[1]
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			if i >= len(lines) {
				continue
			}
			body, consumed := consumeIndentedBlock(lines[i:], directiveIndent)
			i += consumed
			out = append(out, emitUnlabeledLiteral(directiveIndent, body)...)
			continue
		}
		// 3b: text + `::` on the same line.
		if m := reLiteralIntro.FindStringSubmatch(line); m != nil {
			prefix := m[1]
			out = append(out, prefix+":")
			i++
			// Skip blank lines.
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			if i >= len(lines) {
				continue
			}
			directiveIndent := leadingWS(line)
			body, consumed := consumeIndentedBlock(lines[i:], directiveIndent)
			i += consumed
			out = append(out, emitUnlabeledLiteral(directiveIndent, body)...)
			continue
		}

		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

// consumeIndentedBlock returns all lines that are more indented than
// `parentIndent`, stopping at the first non-blank line whose indent is ≤
// parentIndent. It also returns how many input lines were consumed.
// The returned body is dedented by the minimum common indent of the
// block so the fenced output reads naturally.
func consumeIndentedBlock(lines []string, parentIndent string) ([]string, int) {
	var body []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			i++
			continue
		}
		if !startsWithIndentDeeper(line, parentIndent) {
			break
		}
		body = append(body, line)
		i++
	}

	// Trim trailing blank lines — they belong after the fence.
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}

	// Dedent by the minimum non-blank indent.
	minIndent := -1
	for _, line := range body {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(leadingWS(line))
		if minIndent == -1 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent > 0 {
		for idx, line := range body {
			if len(line) >= minIndent {
				body[idx] = line[minIndent:]
			}
		}
	}

	return body, i
}

// startsWithIndentDeeper reports whether `line` is indented strictly
// further than `parentIndent`.
func startsWithIndentDeeper(line, parentIndent string) bool {
	lws := leadingWS(line)
	if len(lws) <= len(parentIndent) {
		return false
	}
	// Check that parentIndent is a prefix (mixed tab/space would fail
	// this test but RST doesn't mix in the Canton/Daml corpus).
	return strings.HasPrefix(line, parentIndent)
}

// leadingWS returns the leading whitespace run of a line.
func leadingWS(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

// reindentBlock prefixes every non-blank line in body with indent so
// that code content sits at the same indentation level as the enclosing
// fence. CommonMark strips up to the fence indent from content lines,
// so the rendered code is unaffected — but the raw MDX keeps the
// content inside the fence boundary, preventing MDX parsers from
// misinterpreting keywords like `import` as ES module statements.
func reindentBlock(body []string, indent string) []string {
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

// emitUnlabeledLiteral chooses an MDX rendering for a `::` literal
// block (RST didn't tell us what language it is):
//
//   - one non-blank content line → 4-space indented prose so the
//     output reads like a small inline command, not a syntax-
//     highlighted snippet (matches the human migrator's convention)
//   - two or more lines → fenced ```text``` block
//
// Either way we surround the result with blank lines so it doesn't
// glue onto adjacent prose.
func emitUnlabeledLiteral(indent string, body []string) []string {
	nonBlank := 0
	for _, l := range body {
		if strings.TrimSpace(l) != "" {
			nonBlank++
		}
	}
	if nonBlank == 1 {
		// Find the single content line and emit it with 4-space indent.
		for _, l := range body {
			if strings.TrimSpace(l) != "" {
				return []string{"", indent + "    " + strings.TrimSpace(l), ""}
			}
		}
	}
	out := []string{"", indent + "```text"}
	out = append(out, reindentBlock(body, indent)...)
	out = append(out, indent+"```", "")
	return out
}
