// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "testing"

func TestStructureValidator(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantCodes []string // nil/empty means expect no findings; order matters
	}{
		{
			name: "balanced nested doc (pass)",
			content: `<AccordionGroup>
<Accordion title="A">body</Accordion>
<Accordion title="B">body</Accordion>
</AccordionGroup>`,
		},
		{
			name:      "missing close (fail)",
			content:   `<Accordion title="A">body never closed`,
			wantCodes: []string{"jsx-unclosed-tag"},
		},
		{
			name: "mismatched close (fail)",
			content: `<Note>
text
</Tip>`,
			// The mismatched </Tip> is reported as unexpected; the open <Note>
			// is left on the stack and reported as unclosed at EOF.
			wantCodes: []string{"jsx-unexpected-close", "jsx-unclosed-tag"},
		},
		{
			name:      "stray close with empty stack (fail)",
			content:   `</Accordion>`,
			wantCodes: []string{"jsx-unexpected-close"},
		},
		{
			name: "self-closing tag does not affect balance (pass)",
			content: `<AccordionGroup>
<Icon icon="check" iconType="solid" />
<Accordion title="A">body</Accordion>
</AccordionGroup>`,
		},
		{
			name: "unclosed tag inside code fence is ignored (pass)",
			content: "Here is an example:\n" +
				"```mdx\n" +
				"<Accordion title=\"A\">\n" +
				"never closed in the fence\n" +
				"```\n" +
				"And normal prose continues.",
		},
	}

	v := StructureValidator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte(tc.content)
			got := v.Validate("test.mdx", content, newParsed(content))
			if len(got) != len(tc.wantCodes) {
				t.Fatalf("expected %d findings %v, got %d: %v",
					len(tc.wantCodes), tc.wantCodes, len(got), got)
			}
			for i, want := range tc.wantCodes {
				if got[i].Code != want {
					t.Errorf("finding[%d]: expected code %q, got %q (msg: %q)",
						i, want, got[i].Code, got[i].Message)
				}
			}
		})
	}
}

func TestStructureValidatorName(t *testing.T) {
	if got := (StructureValidator{}).Name(); got != "structure" {
		t.Errorf("Name() = %q, want %q", got, "structure")
	}
}
