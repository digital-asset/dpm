// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"strings"
	"testing"
)

func TestConvertCodeBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "code-block directive with language",
			in: `Before.

.. code-block:: bash

   dpm install 3.4.11
   dpm version

After.`,
			want: `Before.


` + "```bash" + `
dpm install 3.4.11
dpm version
` + "```" + `

After.`,
		},
		{
			name: "code directive alias",
			in: `.. code:: yaml

   sdk-version: 3.4.11
   dependencies: []`,
			want: `
` + "```yaml" + `
sdk-version: 3.4.11
dependencies: []
` + "```" + `
`,
		},
		{
			name: "multi-line :: literal block fences as text",
			in: `Clone the repo::

   git clone https://example.com/x.git
   cd x

Then build it.`,
			want: `Clone the repo:

` + "```text" + `
git clone https://example.com/x.git
cd x
` + "```" + `

Then build it.`,
		},
		{
			name: "single-line :: literal block uses 4-space indent",
			in: `Open the URL:

::

   app-provider.localhost:3000

Then continue.`,
			want: `Open the URL:


    app-provider.localhost:3000

Then continue.`,
		},
		{
			name: "single-line :: with text prefix uses 4-space indent",
			in: `Run from quickstart/::

   make open-app-ui

Done.`,
			want: `Run from quickstart/:

    make open-app-ui

Done.`,
		},
		{
			name: "options are stripped",
			in: `.. code-block:: python
   :linenos:
   :emphasize-lines: 2

   def foo():
       pass`,
			want: `
` + "```python" + `
def foo():
    pass
` + "```" + `
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertCodeBlocks(tc.in)
			if !equalTrim(got, tc.want) {
				t.Errorf("mismatch\nwant:\n%s\n got:\n%s", tc.want, got)
			}
		})
	}
}

// equalTrim ignores trailing whitespace differences because the various
// transform phases each fiddle with surrounding blank lines. The cleanup
// pass collapses them; these unit tests care about structural equality.
func equalTrim(a, b string) bool {
	return strings.TrimRight(a, "\n ") == strings.TrimRight(b, "\n ")
}
