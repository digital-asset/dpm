// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "testing"

func TestFrontmatterValidator(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantCode    string // "" means expect no findings
	}{
		{
			name: "valid frontmatter with unquoted title",
			content: `---
title: Quickstart
description: Get started fast
---

# Quickstart

body
`,
		},
		{
			name: "valid frontmatter with double-quoted title",
			content: `---
title: "Canton Quickstart"
---

body
`,
		},
		{
			name: "valid frontmatter with single-quoted title",
			content: `---
title: 'Canton Quickstart'
---

body
`,
		},
		{
			name: "no frontmatter at all",
			content: `# Some Page

body without frontmatter
`,
			wantCode: "frontmatter-missing",
		},
		{
			name: "unclosed frontmatter block",
			content: `---
title: Foo
description: bar
`,
			wantCode: "frontmatter-missing",
		},
		{
			name: "frontmatter without title",
			content: `---
description: a page with no title
sidebarTitle: foo
---

body
`,
			wantCode: "frontmatter-missing-title",
		},
		{
			name: "empty title (unquoted)",
			content: `---
title:
description: x
---

body
`,
			wantCode: "frontmatter-empty-title",
		},
		{
			name: "empty title (quoted)",
			content: `---
title: ""
---

body
`,
			wantCode: "frontmatter-empty-title",
		},
		{
			name: "empty title (single-quoted whitespace)",
			content: `---
title: '   '
---

body
`,
			wantCode: "frontmatter-empty-title",
		},
	}

	v := FrontmatterValidator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := v.Validate("test.mdx", []byte(tc.content), nil)
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

func TestFrontmatterValidatorName(t *testing.T) {
	if got := (FrontmatterValidator{}).Name(); got != "frontmatter" {
		t.Errorf("Name() = %q, want %q", got, "frontmatter")
	}
}
