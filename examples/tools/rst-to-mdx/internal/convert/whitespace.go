// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

// cleanupWhitespace tidies the output after all transforms have run:
//   - trim trailing spaces from every line
//   - collapse runs of 3+ blank lines to exactly two (one blank line of
//     separation)
//   - strip trailing blank lines; composeOutput adds exactly one \n.
var (
	reTrailingSpace = regexp.MustCompile(`(?m)[ \t]+$`)
	reExcessBlanks  = regexp.MustCompile(`\n{3,}`)
)

func cleanupWhitespace(s string) string {
	s = reTrailingSpace.ReplaceAllString(s, "")
	s = reExcessBlanks.ReplaceAllString(s, "\n\n")
	s = strings.TrimRight(s, "\n")
	return s
}
