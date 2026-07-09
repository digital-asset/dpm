// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "regexp"

// After the code-block transform emits fenced sections, we normalize the
// language tags to what Mintlify / Shiki expects:
//
//	none     → text
//	console  → bash   (treated as shell session)
//	daml     → haskell (Shiki recognizes haskell but not daml)

var (
	reLangNone    = regexp.MustCompile("(?m)^(\\s*)```none\\b")
	reLangConsole = regexp.MustCompile("(?m)^(\\s*)```console\\b")
	reLangDaml    = regexp.MustCompile("(?m)^(\\s*)```daml\\b")
)

func normalizeLanguages(s string) string {
	s = reLangNone.ReplaceAllString(s, "$1```text")
	s = reLangConsole.ReplaceAllString(s, "$1```bash")
	s = reLangDaml.ReplaceAllString(s, "$1```haskell")
	return s
}
