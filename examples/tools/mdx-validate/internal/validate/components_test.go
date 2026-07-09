// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "testing"

func TestComponentValidator(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantCode string // "" means expect no findings
	}{
		// Check 1: missing required prop.
		{
			name:    "accordion with required title (pass)",
			content: `<Accordion title="Setup">body</Accordion>`,
		},
		{
			name:     "accordion missing required title (fail)",
			content:  `<Accordion>body</Accordion>`,
			wantCode: "component-missing-required-prop",
		},
		{
			name:     "card missing required title (fail)",
			content:  `<Card href="/x">body</Card>`,
			wantCode: "component-missing-required-prop",
		},
		// Check 2: invalid enum value.
		{
			name:    "icon valid iconType enum (pass)",
			content: `<Icon icon="check" iconType="solid" />`,
		},
		{
			name:     "icon invalid iconType enum (fail)",
			content:  `<Icon icon="check" iconType="sparkly" />`,
			wantCode: "component-invalid-enum",
		},
		// Check 3: unknown prop.
		{
			name:    "card with only known props (pass)",
			content: `<Card title="Hi" icon="book" href="/x">body</Card>`,
		},
		{
			name:     "accordion with unknown prop (fail)",
			content:  `<Accordion title="Setup" bogusProp="x">body</Accordion>`,
			wantCode: "component-unknown-prop",
		},
		// Zero-false-positive contract: unknown component is ignored entirely,
		// even though it has no catalog entry and "props" we cannot validate.
		{
			name:    "unknown component is ignored",
			content: `<ExternalCantonFoo bar="baz" title="anything" />`,
		},
		// Required prop supplied as an expression counts as present.
		{
			name:    "required prop as expression counts as present",
			content: `<Accordion title={x}>body</Accordion>`,
		},
	}

	v := ComponentValidator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte(tc.content)
			got := v.Validate("test.mdx", content, newParsed(content))
			switch {
			case tc.wantCode == "":
				if len(got) != 0 {
					t.Errorf("expected no findings, got %v", got)
				}
			case len(got) == 0:
				t.Errorf("expected finding %q, got none", tc.wantCode)
			case got[0].Code != tc.wantCode:
				t.Errorf("expected finding code %q, got %q (msg: %q)",
					tc.wantCode, got[0].Code, got[0].Message)
			}
		})
	}
}

func TestComponentValidatorName(t *testing.T) {
	if got := (ComponentValidator{}).Name(); got != "components" {
		t.Errorf("Name() = %q, want %q", got, "components")
	}
}
