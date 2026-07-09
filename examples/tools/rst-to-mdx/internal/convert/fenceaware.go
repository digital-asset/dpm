// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "strings"

// transformOutsideFences runs `fn` over every portion of the input that
// lies outside a triple-backtick fenced code block, leaving the fenced
// content untouched. After the code-block transform fences content, any
// downstream text transform that could match inside code (inline roles,
// comments, link regexes, list markers) should go through this wrapper
// so we don't rewrite source code.
//
// The fence detector is line-based and only recognizes openers/closers
// that start with three or more backticks after optional leading
// whitespace. Indented fences beyond a blockquote are not supported
// because the Canton/Daml corpus doesn't use them.
func transformOutsideFences(s string, fn func(string) string) string {
	lines := strings.Split(s, "\n")
	var out []string
	var buf []string
	inFence := false
	for _, line := range lines {
		trim := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trim, "```") {
			// Flush any pending prose buffer through fn.
			if len(buf) > 0 {
				out = append(out, splitLines(fn(strings.Join(buf, "\n")))...)
				buf = buf[:0]
			}
			out = append(out, line)
			inFence = !inFence
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		buf = append(buf, line)
	}
	if len(buf) > 0 {
		out = append(out, splitLines(fn(strings.Join(buf, "\n")))...)
	}
	return strings.Join(out, "\n")
}

// splitLines is like strings.Split(s, "\n") but returns an empty slice
// for an empty string so the caller doesn't accumulate spurious blank
// lines around the join seam.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// stripDoubleBackticksInFences walks fenced code blocks and removes any
// remaining `` characters from the content. RST `:: + ``code``` patterns
// occasionally produce code blocks whose body still has `` markers from
// the original inline-code role; in MDX those backticks render as
// literal text inside the code block, which is not what readers want.
func stripDoubleBackticksInFences(s string) string {
	lines := strings.Split(s, "\n")
	inFence := false
	for i, line := range lines {
		trim := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		if strings.Contains(line, "``") {
			lines[i] = strings.ReplaceAll(line, "``", "")
		}
	}
	return strings.Join(lines, "\n")
}
