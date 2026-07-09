// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestEscapeMDXPlaceholders(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "bracketed placeholder",
			in:   "Values in brackets (**[<scheme>]**) indicate config strings",
			want: `Values in brackets (**\[\<scheme\>\]**) indicate config strings`,
		},
		{
			name: "angle-only placeholder",
			in:   "Replace <your-token> with the actual value.",
			want: `Replace \<your-token\> with the actual value.`,
		},
		{
			name: "JSX component left alone",
			in:   "<Note>This is a note</Note>",
			want: "<Note>This is a note</Note>",
		},
		{
			name: "iframe with attributes left alone",
			in:   `<iframe width="560" />`,
			want: `<iframe width="560" />`,
		},
		{
			name: "markdown link untouched",
			in:   "See [the docs](https://example.com/x) for more.",
			want: "See [the docs](https://example.com/x) for more.",
		},
		{
			name: "code fence content untouched",
			in:   "```bash\n<scheme>\n```",
			want: "```bash\n<scheme>\n```",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeMDXPlaceholders(tc.in)
			if got != tc.want {
				t.Errorf("\nwant: %q\n got: %q", tc.want, got)
			}
		})
	}
}

func TestStripDoubleBackticksInFences(t *testing.T) {
	in := "Outside ``foo`` is preserved.\n" +
		"\n" +
		"```bash\n" +
		"``make open-app-ui``\n" +
		"```\n" +
		"\n" +
		"More ``outside`` prose.\n"
	want := "Outside ``foo`` is preserved.\n" +
		"\n" +
		"```bash\n" +
		"make open-app-ui\n" +
		"```\n" +
		"\n" +
		"More ``outside`` prose.\n"
	if got := stripDoubleBackticksInFences(in); got != want {
		t.Errorf("\nwant: %q\n got: %q", want, got)
	}
}
