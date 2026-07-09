// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertHeadings(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "overlined title",
			in: `======================
Canton Getting Started
======================`,
			want: `# Canton Getting Started`,
		},
		{
			name: "underlined = is H2 (Canton/Daml convention)",
			in: `Introduction
============`,
			want: `## Introduction`,
		},
		{
			name: "underlined - is H3",
			in: `Section
=======

Subsection
----------`,
			want: `## Section

### Subsection`,
		},
		{
			name: "underlined ~ is H4",
			in: `Top
===

Mid
---

Leaf
~~~~`,
			want: `## Top

### Mid

#### Leaf`,
		},
		{
			name: "underline shorter than title is not a heading",
			in: `Not a heading
=====`,
			want: `Not a heading
=====`,
		},
		{
			name: "multiple H3s at same level",
			in: `Top
===

One
---

Two
---`,
			want: `## Top

### One

### Two`,
		},
		{
			name: "underlined * is H2",
			in: `Synchronizer Functionality
**************************`,
			want: `## Synchronizer Functionality`,
		},
		{
			name: "overlined * is H1 (bumped one level shallower than underline)",
			in: `**********
Subtitle
**********`,
			want: `# Subtitle`,
		},
		{
			name: "underlined # is H1",
			in: `Protocols on One Synchronizer
#############################`,
			want: `# Protocols on One Synchronizer`,
		},
		{
			name: "overlined # stays at H1 (cap)",
			in: `#####
Title
#####`,
			want: `# Title`,
		},
		{
			name: "overlined - is H2",
			in: `------
Title
------`,
			want: `## Title`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertHeadings(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nwant:\n%q\n got:\n%q", tc.want, got)
			}
		})
	}
}
