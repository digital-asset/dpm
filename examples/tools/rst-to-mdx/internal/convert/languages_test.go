// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestNormalizeLanguages(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"none to text", "```none\nfoo\n```", "```text\nfoo\n```"},
		{"console to bash", "```console\n$ ls\n```", "```bash\n$ ls\n```"},
		{"daml to haskell", "```daml\ntemplate Foo\n```", "```haskell\ntemplate Foo\n```"},
		{"bash untouched", "```bash\nls\n```", "```bash\nls\n```"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLanguages(tc.in)
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}
