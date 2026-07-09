// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import "testing"

func TestLocalComponentNames(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "export const",
			content:     "export const Tooltip = ({children, content}) => {};\n",
			wantPresent: []string{"Tooltip"},
		},
		{
			name: "export function/let/var keywords",
			content: "export function Foo() {}\n" +
				"export let Bar = 1\n" +
				"export var Baz = 2\n",
			wantPresent: []string{"Foo", "Bar", "Baz"},
		},
		{
			name:        "default import",
			content:     "import Foo from '/snippets/foo.mdx'\n",
			wantPresent: []string{"Foo"},
		},
		{
			name:        "named import single",
			content:     "import { Bar } from '/snippets/bar.mdx'\n",
			wantPresent: []string{"Bar"},
		},
		{
			name:        "named import multiple",
			content:     "import { A, B } from '/snippets/x.mdx'\n",
			wantPresent: []string{"A", "B"},
		},
		{
			name:        "aliased named import binds local name",
			content:     "import { Original as Alias } from '/snippets/x.mdx'\n",
			wantPresent: []string{"Alias"},
			wantAbsent:  []string{"Original"},
		},
		{
			name: "multiline named import block",
			content: "import {\n" +
				"  One,\n" +
				"  Two as Three,\n" +
				"} from '/snippets/x.mdx'\n",
			wantPresent: []string{"One", "Three"},
			wantAbsent:  []string{"Two"},
		},
		{
			name: "lowercase identifiers ignored",
			content: "export const helper = 1\n" +
				"import { networkData } from '/snippets/data.mdx'\n",
			wantAbsent: []string{"helper", "networkData"},
		},
		{
			name:       "component only used, not declared or imported",
			content:    "---\ntitle: T\n---\n\n<Note>hello</Note>\n",
			wantAbsent: []string{"Note"},
		},
		{
			name: "real-world dashboard shape",
			content: "import { networkData } from '/snippets/generated/data.mdx';\n" +
				"export const Tooltip = ({ children, content }) => {\n" +
				"  return <div>{content}</div>;\n" +
				"};\n",
			wantPresent: []string{"Tooltip"},
			wantAbsent:  []string{"networkData"},
		},
		{
			name:       "type-only import is not matched (limitation)",
			content:    "import type { Foo } from '/snippets/foo.mdx'\n",
			wantAbsent: []string{"Foo"},
		},
		{
			name:       "mixed default+named import: neither default nor named binding is matched (limitation)",
			content:    "import React, { Card } from 'react'\n",
			wantAbsent: []string{"React", "Card"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := localComponentNames([]byte(tc.content))
			for _, name := range tc.wantPresent {
				if !got[name] {
					t.Errorf("expected %q to be detected as local; got set %v", name, keys(got))
				}
			}
			for _, name := range tc.wantAbsent {
				if got[name] {
					t.Errorf("expected %q NOT to be detected as local; got set %v", name, keys(got))
				}
			}
		})
	}
}

// keys returns the map keys for readable failure messages.
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
