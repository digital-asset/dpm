// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertAdmonitions(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "note block",
			in: `.. note::

   Canton Admin APIs are not the same as the admin package of gRPC.`,
			want: `<Note>
Canton Admin APIs are not the same as the admin package of gRPC.
</Note>`,
		},
		{
			name: "inline note",
			in:   `.. note:: Quick heads-up here.`,
			want: `<Note>
Quick heads-up here.
</Note>`,
		},
		{
			name: "warning",
			in: `.. warning::

   Running in production requires extra care.`,
			want: `<Warning>
Running in production requires extra care.
</Warning>`,
		},
		{
			name: "important maps to Warning with prefix",
			in: `.. important::

   In-memory config should not be used in production.`,
			want: `<Warning>
**Important:** In-memory config should not be used in production.
</Warning>`,
		},
		{
			name: "tip maps to Tip",
			in: `.. tip::

   Use dpm version --active to see the active SDK.`,
			want: `<Tip>
Use dpm version --active to see the active SDK.
</Tip>`,
		},
		{
			name: "hint maps to Tip",
			in: `.. hint::

   You can alias commands.`,
			want: `<Tip>
You can alias commands.
</Tip>`,
		},
		{
			name: "seealso maps to Info",
			in: `.. seealso::

   See the Canton Admin docs.`,
			want: `<Info>
See the Canton Admin docs.
</Info>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertAdmonitions(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nwant:\n%q\n got:\n%q", tc.want, got)
			}
		})
	}
}
