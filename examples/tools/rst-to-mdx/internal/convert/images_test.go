// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertImages(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "image with alt and width",
			in: `.. image:: images/01-allow-direnv.png
   :alt: allow direnv
   :width: 600px
`,
			want: `![allow direnv](/images/docs_website/01-allow-direnv.png)
`,
		},
		{
			name: "image without alt falls back",
			in: `.. image:: _static/diagram.png
`,
			want: `![image](/images/docs_website/diagram.png)
`,
		},
		{
			name: "figure with caption",
			in: `.. figure:: diagrams/arch.png
   :alt: Architecture

   The overall architecture.
`,
			want: `<Frame caption="The overall architecture.">
  ![Architecture](/images/docs_website/arch.png)
</Frame>
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertImages(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nwant:\n%s got:\n%s", tc.want, got)
			}
		})
	}
}
