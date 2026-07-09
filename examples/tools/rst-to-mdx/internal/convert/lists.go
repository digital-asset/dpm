// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// RST bullet lists use * or -. MDX conventionally uses -. RST also has
// `#.` auto-numbered lists that we replace with explicit numbering since
// markdown renderers auto-number anyway and `1. 1. 1.` renders fine.
// We scope the transform to list items — plain `*` in prose (e.g. *italic*)
// should be left alone.

var reBulletAsterisk = regexp.MustCompile(`(?m)^(\s*)\*\s+`)
var reAutoNumber = regexp.MustCompile(`(?m)^(\s*)#\.\s+`)

func convertLists(s string) string {
	lines := strings.Split(s, "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		// Only convert an asterisk bullet if the line starts with it
		// (after optional whitespace) — leaving `*italic*` in prose
		// untouched. reBulletAsterisk already anchors to ^, so this is
		// fine, but we also require the character after the asterisk
		// to be whitespace (handled by \s+ in the regex).
		line = reBulletAsterisk.ReplaceAllString(line, "${1}- ")
		line = reAutoNumber.ReplaceAllString(line, "${1}1. ")
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
