// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "regexp"

// RST inline roles and formatting:
//
//	``code``       → `code`
//	:code:`text`   → `text`
//	:file:`text`   → `text`
//	:command:`text`→ `text`
//	:kbd:`text`    → `text`
//	:guilabel:`x`  → **x**
//	:menuselection:`x` → **x**
//	:term:`x`      → **x**  (Phase 2+ may upgrade to a glossary link)
//	:abbr:`text (expansion)` → text
//	*italic*       → *italic* (already markdown-compatible)
//	**bold**       → **bold** (already markdown-compatible)

// All inline regexes exclude `\n` from their character classes so a
// stray opening backtick can't pair with a closing backtick on a much
// later line — RST inline markup is always single-line.
var (
	// `` must match before single backtick so `` isn't eaten as two
	// empty inlines.
	reDoubleBacktick = regexp.MustCompile("``([^`\n]+?)``")
	reCodeRole       = regexp.MustCompile(":code:`([^`\n]+?)`")
	reFileRole       = regexp.MustCompile(":file:`([^`\n]+?)`")
	reCommandRole    = regexp.MustCompile(":command:`([^`\n]+?)`")
	reKbdRole        = regexp.MustCompile(":kbd:`([^`\n]+?)`")
	reGuilabelRole   = regexp.MustCompile(":guilabel:`([^`\n]+?)`")
	reMenuRole       = regexp.MustCompile(":menuselection:`([^`\n]+?)`")
	reTermRole       = regexp.MustCompile(":term:`([^`\n]+?)`")
	// `:abbr:` carries an expansion in parens we just drop.
	reAbbrRole = regexp.MustCompile(":abbr:`([^(`\n]+?)\\s*\\([^)\n]*\\)`")
)

func convertInlineRoles(s string) string {
	s = reDoubleBacktick.ReplaceAllString(s, "`$1`")
	s = reCodeRole.ReplaceAllString(s, "`$1`")
	s = reFileRole.ReplaceAllString(s, "`$1`")
	s = reCommandRole.ReplaceAllString(s, "`$1`")
	s = reKbdRole.ReplaceAllString(s, "`$1`")
	s = reGuilabelRole.ReplaceAllString(s, "**$1**")
	s = reMenuRole.ReplaceAllString(s, "**$1**")
	s = reTermRole.ReplaceAllString(s, "**$1**")
	s = reAbbrRole.ReplaceAllString(s, "$1")
	return s
}
