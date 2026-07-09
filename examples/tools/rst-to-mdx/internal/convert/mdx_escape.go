// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// MDX is stricter than plain Markdown about angle brackets: a sequence
// like `<scheme>` looks like a JSX/HTML opening tag. Mintlify's parser
// will either fail, drop the text, or warn — none of which we want.
//
// Documentation prose in the corpus uses bracketed placeholders heavily
// (`<scheme>`, `<file>`, `<path>`, `<your-token>`, etc.). We escape
// the surrounding `[`, `<`, `>`, `]` so the rendered MDX shows the
// original characters as text.
//
// We DO NOT escape:
//   - JSX components we emit ourselves (Note, Tip, Warning, Frame,
//     iframe with attributes, etc.) — those use uppercase initial
//     letters or have a space-attribute follow-up that bypasses the
//     simple `<word>` shape.
//   - Content inside fenced code blocks (handled by the fence-aware
//     wrapper).
//   - Markdown links `[text](url)` — those don't have `<...>` between
//     the brackets so they don't match the patterns below.

var (
	// `[<word>]` as a unit. Captures the lowercase placeholder so the
	// surrounding brackets and angle brackets all get escaped together
	// (matches the human migrator convention).
	reBracketedPlaceholder = regexp.MustCompile(`\[<([a-z][a-zA-Z0-9_\-]*)>\]`)
	// `<word>` standing on its own (no surrounding `[]`). Lowercase
	// initial letter to skip JSX components like `<Note>` and our own
	// emitted iframe (which has attributes and a space, so it never
	// matches anyway).
	reAngleOnlyPlaceholder = regexp.MustCompile(`<([a-z][a-zA-Z0-9_\-]*)>`)
	// Stray `<` followed by a character that can't start a JSX/HTML
	// name. MDX errors out on shapes like `<-`, `<=`, `<3`, `< x`
	// because it expects a name-start (letter, `$`, `_`) or `/`/`!`/`?`
	// after `<`. Anything else is just prose and needs the angle
	// bracket escaped so Mintlify renders it as a literal `<`.
	reStrayLT = regexp.MustCompile("<([^A-Za-z/!?$_])")
)

// escapeMDXPlaceholders escapes the angle-bracket placeholders that
// MDX would otherwise interpret as tags. Runs in fence-aware mode so
// code blocks are never rewritten.
func escapeMDXPlaceholders(s string) string {
	return transformOutsideFences(s, func(prose string) string {
		prose = reBracketedPlaceholder.ReplaceAllString(prose, `\[\<$1\>\]`)
		prose = reAngleOnlyPlaceholder.ReplaceAllStringFunc(prose, func(m string) string {
			// Skip closing tags like `</foo>` — they shouldn't appear
			// here (the inner regex doesn't match `/`), but be safe.
			if strings.HasPrefix(m, "</") {
				return m
			}
			inner := m[1 : len(m)-1]
			return `\<` + inner + `\>`
		})
		prose = reStrayLT.ReplaceAllString(prose, `\<$1`)
		return prose
	})
}
