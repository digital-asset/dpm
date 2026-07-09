// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST headings use underlines (and sometimes overlines) made of a repeat
// character. Canton's published CSS interprets the characters as a fixed
// hierarchy regardless of order-of-appearance, so the converter follows
// the same rule:
//
//	underline only:        #  → H1
//	                       *  → H2
//	                       =  → H2 (legacy alias seen in some files)
//	                       -  → H3
//	                       ~  → H4
//	                       ^  → H5
//	                       "  → H6
//
//	overline+underline:    one level shallower than the same character
//	                       used as underline-only, capped at H1.
//
// So `### Title ###` and `############` both render as H1; `*** Title ***`
// is also H1 (bumped from H2); `--- Title ---` is H2; etc.
//
// The pipeline converts headings before any other transform so we don't
// accidentally rewrite the `----` underlines (which look like horizontal
// rules in MDX) or the `=====` overlines (which look like setext H1).

// reUnderlineOnly matches a heading line followed by an underline of ≥3
// repeats of the same marker character. The marker chars are the ones
// the Canton/Daml RST style guide uses.
var reUnderlineOnly = regexp.MustCompile(
	`(?m)^(\S[^\n]*)\n([#+*=\-~^"]{3,})[ \t]*$`)

// reOverline matches an overlined heading where the overline and
// underline are the same character repeated at least 3 times.
var reOverline = regexp.MustCompile(
	`(?m)^([#+*=\-~^"]{3,})\n(\S[^\n]*)\n([#+*=\-~^"]{3,})[ \t]*$`)

// convertHeadings replaces RST underline/overline headings with `#`-style
// MDX headings. It first converts overlined headings (which can start
// with a hyphen overline that'd otherwise be mistaken for an underline of
// an empty line) and then converts plain underline-only headings.
func convertHeadings(s string) string {
	// Overlined: === / Title / ===. One level shallower than the same
	// character used as an underline-only heading, capped at H1.
	s = reOverline.ReplaceAllStringFunc(s, func(match string) string {
		m := reOverline.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		over, title, under := m[1], m[2], m[3]
		if !sameChar(over, under) {
			return match
		}
		level := baseLevel(over[0]) - 1
		if level < 1 {
			level = 1
		}
		return strings.Repeat("#", level) + " " + strings.TrimSpace(title)
	})

	// Underline only.
	s = reUnderlineOnly.ReplaceAllStringFunc(s, func(match string) string {
		m := reUnderlineOnly.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		title, under := m[1], m[2]
		if len(under) < len(title) {
			// RST requires the underline to be at least as long as
			// the title. If it isn't, this isn't really a heading.
			return match
		}
		level := baseLevel(under[0])
		return strings.Repeat("#", level) + " " + strings.TrimSpace(title)
	})

	return s
}

// sameChar reports whether both strings consist of the same repeated byte.
func sameChar(s, t string) bool {
	if len(s) == 0 || len(t) == 0 {
		return false
	}
	c := s[0]
	if t[0] != c {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != c {
			return false
		}
	}
	return true
}

// baseLevel is the MDX heading level a marker character produces when
// used as an underline-only heading (no overline). Overlined headings
// derive their level from this by subtracting 1 (capped at H1).
func baseLevel(c byte) int {
	switch c {
	case '#':
		return 1
	case '*', '=':
		return 2
	case '-':
		return 3
	case '~':
		return 4
	case '^':
		return 5
	case '"':
		return 6
	default:
		return 2
	}
}
