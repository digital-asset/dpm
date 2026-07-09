// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"strings"

	"daml.com/x/dpm-components/mdx-validate/internal/catalog"
	"daml.com/x/dpm-components/mdx-validate/internal/mdxscan"
)

// ComponentValidator checks usage of known Mintlify MDX components against the
// curated catalog. For each element whose name is in the catalog it enforces:
//
//   - Missing required prop  → Error   "component-missing-required-prop"
//   - Invalid enum value     → Error   "component-invalid-enum"
//   - Unknown prop           → Warning "component-unknown-prop"
//
// Elements whose name is NOT in the catalog produce no findings at all: they
// may be imported snippets or custom components (e.g. <ExternalCantonFoo />),
// and flagging them would break the zero-false-positive contract. Only Open and
// SelfClose elements are inspected; Close tags carry no attributes.
//
// Out of scope (handled elsewhere or deliberately omitted): parent/child
// nesting rules, prop value type checks beyond enums, and deprecation.
type ComponentValidator struct{}

// Name implements Validator.
func (ComponentValidator) Name() string { return "components" }

// Validate implements Validator.
func (v ComponentValidator) Validate(path string, content []byte, p *parsed) []Finding {
	var findings []Finding
	local := p.localNames

	for _, el := range p.elements {
		if el.Kind != mdxscan.Open && el.Kind != mdxscan.SelfClose {
			continue
		}
		if local[el.Name] {
			// Locally defined/imported component shadows the catalog name; its
			// prop contract differs. See shadow.go.
			continue
		}
		comp, ok := catalog.Lookup(el.Name)
		if !ok {
			// Unknown component: not ours to judge. See type doc.
			continue
		}

		// Index the element's attrs by name for required-prop checking.
		present := make(map[string]bool, len(el.Attrs))
		for _, a := range el.Attrs {
			present[a.Name] = true
		}

		// Check 1: missing required props.
		for _, req := range comp.RequiredProps() {
			if !present[req] {
				findings = append(findings, Finding{
					Path:     path,
					Line:     el.Line,
					Severity: Error,
					Code:     "component-missing-required-prop",
					Message: fmt.Sprintf(
						"<%s> is missing required prop %q", comp.Name, req),
				})
			}
		}

		// Checks 2 & 3: per-attr enum and unknown-prop validation.
		for _, a := range el.Attrs {
			spec, ok := comp.PropByName(a.Name)
			if !ok {
				// Check 3: unknown prop.
				findings = append(findings, Finding{
					Path:     path,
					Line:     el.Line,
					Severity: Warning,
					Code:     "component-unknown-prop",
					Message: fmt.Sprintf(
						"<%s> has unknown prop %q", comp.Name, a.Name),
				})
				continue
			}

			// Check 2: invalid enum value. Only literal values are checked;
			// expression values ({…}) are opaque and skipped.
			if len(spec.EnumValues) > 0 && !a.IsExpr && !contains(spec.EnumValues, a.Value) {
				findings = append(findings, Finding{
					Path:     path,
					Line:     el.Line,
					Severity: Error,
					Code:     "component-invalid-enum",
					Message: fmt.Sprintf(
						"<%s> prop %q has invalid value %q; allowed values: %s",
						comp.Name, a.Name, a.Value, strings.Join(spec.EnumValues, ", ")),
				})
			}
		}
	}

	return findings
}

// contains reports whether s is present in vals.
func contains(vals []string, s string) bool {
	for _, v := range vals {
		if v == s {
			return true
		}
	}
	return false
}
