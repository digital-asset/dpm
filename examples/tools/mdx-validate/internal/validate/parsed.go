// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "daml.com/x/dpm-components/mdx-validate/internal/mdxscan"

// parsed holds the per-file derived data that more than one validator needs:
// the masked element scan and the set of locally shadowed component names.
// Both are pure functions of the file content, so the runner computes them
// once per file and shares the result, instead of each validator tokenizing
// the same bytes independently.
type parsed struct {
	elements   []mdxscan.Element
	localNames map[string]bool
}

// newParsed derives the shared per-file data from content.
func newParsed(content []byte) *parsed {
	return &parsed{
		elements:   mdxscan.Elements(content),
		localNames: localComponentNames(content),
	}
}
