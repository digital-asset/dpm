// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertLists(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "asterisk bullets",
			in: `* One
* Two
  * Nested`,
			want: `- One
- Two
  - Nested`,
		},
		{
			name: "auto-numbered",
			in: `#. First
#. Second`,
			want: `1. First
1. Second`,
		},
		{
			name: "italic asterisk left alone",
			in:   `This is *italic* text.`,
			want: `This is *italic* text.`,
		},
		{
			name: "code fence bullets preserved",
			in: "```python\n* not a bullet\n```",
			want: "```python\n* not a bullet\n```",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLists(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nwant:\n%q\n got:\n%q", tc.want, got)
			}
		})
	}
}
