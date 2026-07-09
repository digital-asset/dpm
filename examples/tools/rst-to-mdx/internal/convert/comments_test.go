// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertComments(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "single line",
			in:   `.. This is a stray note`,
			want: `{/* This is a stray note */}`,
		},
		{
			name: "does not rewrite unknown directive",
			in:   `.. unknowndirective::`,
			want: `.. unknowndirective::`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertComments(tc.in)
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}
