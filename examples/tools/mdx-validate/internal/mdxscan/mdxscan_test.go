// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package mdxscan

import (
	"reflect"
	"testing"
)

func TestElements(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Element
	}{
		{
			name: "open with attr and matching close",
			in:   "<Accordion title=\"x\">\ntext\n</Accordion>\n",
			want: []Element{
				{Name: "Accordion", Kind: Open, Attrs: []Attr{{Name: "title", Value: "x"}}, Line: 1},
				{Name: "Accordion", Kind: Close, Line: 3},
			},
		},
		{
			name: "self closing icon",
			in:   "<Icon icon=\"home\" />\n",
			want: []Element{
				{Name: "Icon", Kind: SelfClose, Attrs: []Attr{{Name: "icon", Value: "home"}}, Line: 1},
			},
		},
		{
			name: "tag inside backtick fence ignored",
			in:   "before\n```\n<Accordion title=\"x\">\n```\nafter\n",
			want: nil,
		},
		{
			name: "HOST:PORT in bash fence returns nothing",
			in:   "Run it:\n```bash\ncurl http://<HOST:PORT>/api\n```\ndone\n",
			want: nil,
		},
		{
			name: "tag inside inline code span ignored",
			in:   "use the `<Card>` component here\n",
			want: nil,
		},
		{
			name: "multi-line opening tag",
			in:   "<Card\n  title=\"a\"\n  icon=\"home\"\n>\n</Card>\n",
			want: []Element{
				{Name: "Card", Kind: Open, Attrs: []Attr{
					{Name: "title", Value: "a"},
					{Name: "icon", Value: "home"},
				}, Line: 1},
				{Name: "Card", Kind: Close, Line: 5},
			},
		},
		{
			name: "gt inside attribute value does not truncate",
			in:   "<Card title=\"a > b\">x</Card>\n",
			want: []Element{
				{Name: "Card", Kind: Open, Attrs: []Attr{{Name: "title", Value: "a > b"}}, Line: 1},
				{Name: "Card", Kind: Close, Line: 1},
			},
		},
		{
			name: "expression attribute",
			in:   "<Card count={2} />\n",
			want: []Element{
				{Name: "Card", Kind: SelfClose, Attrs: []Attr{{Name: "count", IsExpr: true}}, Line: 1},
			},
		},
		{
			name: "gt inside expression does not truncate",
			in:   "<Card show={a > b} />\n",
			want: []Element{
				{Name: "Card", Kind: SelfClose, Attrs: []Attr{{Name: "show", IsExpr: true}}, Line: 1},
			},
		},
		{
			name: "lowercase and fragments ignored",
			in:   "<br>\n<>\n</>\n<div class=\"x\">\n",
			want: nil,
		},
		{
			name: "boolean attribute",
			in:   "<Tab disabled>\n",
			want: []Element{
				{Name: "Tab", Kind: Open, Attrs: []Attr{{Name: "disabled"}}, Line: 1},
			},
		},
		{
			name: "single quoted value",
			in:   "<Icon icon='home' />\n",
			want: []Element{
				{Name: "Icon", Kind: SelfClose, Attrs: []Attr{{Name: "icon", Value: "home"}}, Line: 1},
			},
		},
		{
			name: "tilde fence masks tag",
			in:   "~~~\n<Card />\n~~~\n<Note />\n",
			want: []Element{
				{Name: "Note", Kind: SelfClose, Line: 4},
			},
		},
		{
			name: "indented fence up to three spaces",
			in:   "   ```\n<Card />\n   ```\n",
			want: nil,
		},
		{
			name: "four space indent is not a fence",
			in:   "    ```\n<Card />\n    ```\n",
			want: []Element{
				{Name: "Card", Kind: SelfClose, Line: 2},
			},
		},
		{
			name: "closing fence longer than opener",
			in:   "```\n<Card />\n````\n<Note />\n",
			want: []Element{
				{Name: "Note", Kind: SelfClose, Line: 4},
			},
		},
		{
			name: "info string on opening fence",
			in:   "```js title=\"x\"\n<Card />\n```\n",
			want: nil,
		},
		{
			name: "double backtick span with single inside",
			in:   "text ``<Card> ` more`` end <Note />\n",
			want: []Element{
				{Name: "Note", Kind: SelfClose, Line: 1},
			},
		},
		{
			name: "multiple elements line numbers",
			in:   "line1\n<Tip>a</Tip>\nline3\n<Warning>b</Warning>\n",
			want: []Element{
				{Name: "Tip", Kind: Open, Line: 2},
				{Name: "Tip", Kind: Close, Line: 2},
				{Name: "Warning", Kind: Open, Line: 4},
				{Name: "Warning", Kind: Close, Line: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Elements([]byte(tt.in))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Elements() mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}
