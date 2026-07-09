// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"

	"daml.com/x/dpm-components/mdx-validate/internal/catalog"
	"daml.com/x/dpm-components/mdx-validate/internal/mdxscan"
)

// StructureValidator checks JSX tag balance and nesting. It maintains a stack
// over the elements returned by mdxscan.Elements (capitalized component tags
// found outside of fenced code blocks and inline code spans), but tracks ONLY
// known catalog components that are not locally shadowed. Non-catalog tokens —
// placeholders like <SPONSOR>, generics like <Long>, and custom/imported
// components — are ignored entirely, because they are not balanced JSX and
// would otherwise poison the stack and cascade into false positives. The rules:
//
//   - Open       → push the element.
//   - SelfClose  → no-op (self-balanced).
//   - Close      → must match the top of the stack. A mismatch or an empty
//                  stack is an Error "jsx-unexpected-close".
//   - End of file with a non-empty stack → one Error "jsx-unclosed-tag" per
//                  still-open element, attributed to its opening line.
//
// Out of scope (deliberately omitted because they cannot be made
// false-positive-free against real prose and JS imports): {expression} brace
// balancing, stray '<' / '>' detection, and lowercase/HTML tag balancing.
// Only the capitalized-component tag stack is tracked here.
type StructureValidator struct{}

// Name implements Validator.
func (StructureValidator) Name() string { return "structure" }

// Validate implements Validator.
func (v StructureValidator) Validate(path string, content []byte, p *parsed) []Finding {
	var findings []Finding
	var stack []mdxscan.Element
	local := p.localNames

	for _, el := range p.elements {
		// Only balance known catalog components that are not locally shadowed.
		// Everything else (placeholders, generics, custom/imported components)
		// is not balanced JSX we can reason about, so it is skipped entirely.
		if _, ok := catalog.Lookup(el.Name); !ok || local[el.Name] {
			continue
		}
		switch el.Kind {
		case mdxscan.Open:
			stack = append(stack, el)
		case mdxscan.SelfClose:
			// Self-closing tags are balanced on their own; ignore.
		case mdxscan.Close:
			if len(stack) == 0 {
				// Stray close: nothing is open.
				findings = append(findings, Finding{
					Path:     path,
					Line:     el.Line,
					Severity: Error,
					Code:     "jsx-unexpected-close",
					Message: fmt.Sprintf(
						"unexpected closing tag </%s>: no open tag to close", el.Name),
				})
				continue
			}
			top := stack[len(stack)-1]
			if top.Name == el.Name {
				// Matching close: pop it.
				stack = stack[:len(stack)-1]
				continue
			}
			// Mismatched close. We report the error but do NOT pop the stack:
			// the close is treated as stray and the open tag at the top is left
			// in place so it can still be matched by its own correct closing tag
			// later (and otherwise be reported as jsx-unclosed-tag at EOF). This
			// keeps behavior predictable for the common "wrong tag name typed"
			// case without cascading into spurious extra findings.
			findings = append(findings, Finding{
				Path:     path,
				Line:     el.Line,
				Severity: Error,
				Code:     "jsx-unexpected-close",
				Message: fmt.Sprintf(
					"unexpected closing tag </%s>: currently open tag is <%s> (opened at line %d)",
					el.Name, top.Name, top.Line),
			})
		}
	}

	// Anything left on the stack was never closed.
	for _, open := range stack {
		findings = append(findings, Finding{
			Path:     path,
			Line:     open.Line,
			Severity: Error,
			Code:     "jsx-unclosed-tag",
			Message:  fmt.Sprintf("unclosed tag <%s>: no matching closing tag", open.Name),
		})
	}

	return findings
}
