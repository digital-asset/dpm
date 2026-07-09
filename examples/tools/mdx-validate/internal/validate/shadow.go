// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "regexp"

// Component-name validation (catalog lookup, required props, tag balance) only
// makes sense for the *Mintlify built-in* of that name. A page can define or
// import its own component that shadows a catalog name — e.g.
// `export const Tooltip = ({children, content}) => …` — with a different prop
// contract. Checking such a usage against the catalog produces false positives,
// so both validators skip any name that the page defines or imports locally.
//
// These patterns are intentionally narrow (named imports, default imports, and
// top-level export const/function/let/var) and match capitalized identifiers
// only, since component names are capitalized. Anything fancier (namespace
// imports, re-exports) is out of scope.
var (
	reExportDecl = regexp.MustCompile(`(?m)^\s*export\s+(?:const|function|let|var)\s+([A-Z][A-Za-z0-9_]*)`)
	reDefaultImp = regexp.MustCompile(`(?m)^\s*import\s+([A-Z][A-Za-z0-9_]*)\s+from\b`)
	reNamedImp   = regexp.MustCompile(`(?ms)^\s*import\s*\{([^}]*)\}\s*from\b`)
	reIdent      = regexp.MustCompile(`[A-Z][A-Za-z0-9_]*`)
)

// localComponentNames returns the set of capitalized component names that the
// file defines (export const/function/…) or imports (default or named). Names
// in this set are locally shadowed and must not be checked against the catalog.
func localComponentNames(content []byte) map[string]bool {
	out := map[string]bool{}
	for _, m := range reExportDecl.FindAllSubmatch(content, -1) {
		out[string(m[1])] = true
	}
	for _, m := range reDefaultImp.FindAllSubmatch(content, -1) {
		out[string(m[1])] = true
	}
	// Named imports: `import { A, B as C } from '…'`. Take the local binding,
	// which is the identifier after `as` when present, otherwise the name.
	for _, m := range reNamedImp.FindAllSubmatch(content, -1) {
		for _, clause := range splitCommas(string(m[1])) {
			ids := reIdent.FindAllString(clause, -1)
			if len(ids) == 0 {
				continue
			}
			// `A as C` → bind C (last ident); plain `A` → bind A.
			out[ids[len(ids)-1]] = true
		}
	}
	return out
}

// splitCommas splits a brace-import body on commas. A tiny helper kept separate
// so the import-clause handling above stays readable.
func splitCommas(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
