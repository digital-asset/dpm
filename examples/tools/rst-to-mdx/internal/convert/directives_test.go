// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestStripCopyrightHeader(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "copyright + SPDX",
			in: `..
   Copyright (c) 2026 Digital Asset.
..
   SPDX-License-Identifier: Apache-2.0

Content
`,
			want: `{/* Copyright (c) 2026 Digital Asset. — SPDX-License-Identifier: Apache-2.0 */}

Content
`,
		},
		{
			name: "copyright only",
			in: `..
   Copyright (c) 2026 Digital Asset.

Content
`,
			want: `{/* Copyright (c) 2026 Digital Asset. */}

Content
`,
		},
		{
			name: "no header passes through",
			in: `Content starts immediately
`,
			want: `Content starts immediately
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripCopyrightHeader(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nwant:\n%q\n got:\n%q", tc.want, got)
			}
		})
	}
}

func TestStripSimpleDirectives(t *testing.T) {
	// `.. todo::` is intentionally NOT stripped here — convertTodo
	// renders it as a visible Note so the body isn't lost.
	in := `:orphan:

.. contents:: Table of contents
   :depth: 2
   :local:

.. toctree::
   :maxdepth: 2

   tutorials/install
   tutorials/first-steps

Real content follows.
`
	got := stripSimpleDirectives(in)
	if got == in {
		t.Fatal("directives were not stripped")
	}
	for _, banned := range []string{":orphan:", "contents::", "toctree::"} {
		if contains(got, banned) {
			t.Errorf("%q still present in:\n%s", banned, got)
		}
	}
	if !contains(got, "Real content follows.") {
		t.Errorf("real content was eaten:\n%s", got)
	}
}

func TestStripLabels(t *testing.T) {
	in := `.. _canton-getting-started:

Getting Started
===============
`
	want := `Getting Started
===============
`
	if got := stripLabels(in); got != want {
		t.Errorf("want:\n%q\n got:\n%q", want, got)
	}
}

func TestConvertRubric(t *testing.T) {
	in := `.. rubric:: Footnotes`
	want := `**Footnotes**`
	if got := convertRubric(in); got != want {
		t.Errorf("want %q got %q", want, got)
	}
}

func TestConvertToggle_DefaultTitle(t *testing.T) {
	in := `.. toggle::

    .. code-block:: none

        line one
        line two
`
	got := convertToggle(in)
	for _, want := range []string{
		`<Accordion title="Show example">`,
		`.. code-block:: none`,
		`line one`,
		`</Accordion>`,
	} {
		if !contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if contains(got, ".. toggle::") {
		t.Errorf("toggle directive not consumed:\n%s", got)
	}
}

func TestConvertToggle_ExplicitTitle(t *testing.T) {
	in := `.. toggle:: V3 implementation

    body content here
`
	got := convertToggle(in)
	if !contains(got, `<Accordion title="V3 implementation">`) {
		t.Errorf("title not used; got:\n%s", got)
	}
}

func TestConvertToggle_BodyDedentedForCodeBlock(t *testing.T) {
	// The inner code-block must end up at column 0 so convertCodeBlocks
	// downstream picks it up.
	in := `.. toggle::

    .. code-block:: python

        x = 1
`
	got := convertToggle(in)
	if !contains(got, "\n.. code-block:: python\n") {
		t.Errorf("inner code-block not dedented; got:\n%s", got)
	}
}

func TestConvertRawHTMLVideo_CantonDemoPattern(t *testing.T) {
	in := `.. raw:: html

    <video id="clip" controls="controls" preload="none" onclick="this.paused ? this.play() : this.pause();" width=640 height=400 data-setup="{}">
        <source src="https://www.canton.io/videos/canton-demo.mp4" type='video/mp4'/>
    </video>
`
	got := convertRawHTMLVideo(in)
	want := `<video src="https://www.canton.io/videos/canton-demo.mp4" controls width="640" height="400" />
`
	if got != want {
		t.Errorf("mismatch\nwant:\n%q\n got:\n%q", want, got)
	}
}

func TestConvertRawHTMLVideo_NonVideoPassesThrough(t *testing.T) {
	in := `.. raw:: html

    <div class="custom">hello</div>

After.
`
	if got := convertRawHTMLVideo(in); got != in {
		t.Errorf("non-video raw block was rewritten:\n%s", got)
	}
}

func TestConvertRawHTMLVideo_MultipleSources(t *testing.T) {
	in := `.. raw:: html

    <video controls width=320 height=240>
      <source src="a.mp4" type="video/mp4">
      <source src="b.webm" type="video/webm">
    </video>
`
	got := convertRawHTMLVideo(in)
	for _, want := range []string{
		`<video controls width="320" height="240">`,
		`<source src="a.mp4" type="video/mp4" />`,
		`<source src="b.webm" type="video/webm" />`,
		`</video>`,
	} {
		if !contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConvertTodo_InlineSummaryBecomesComment(t *testing.T) {
	// Single-line todos (inline summary, no indented body) collapse to
	// one MDX-comment line that readers never see.
	in := `.. todo:: Write this section <https://github.com/DACH-NY/canton/issues/25689>
`
	got := convertTodo(in)
	want := "{/* TODO: Write this section <https://github.com/DACH-NY/canton/issues/25689> */}"
	if !contains(got, want) {
		t.Errorf("want substring:\n%s\ngot:\n%s", want, got)
	}
	// And nothing renders.
	for _, banned := range []string{"<Note>", "**TODO:**", ".. todo::"} {
		if contains(got, banned) {
			t.Errorf("%q leaked into output:\n%s", banned, got)
		}
	}
}

func TestConvertTodo_MultiLineBodyBecomesBlockComment(t *testing.T) {
	// Multi-line todos emit `{/*` and `*/}` on their own lines, with
	// the dedented body in between.
	in := `.. todo::
   Repeat integrity, consensus, transparency from protocols section.

   Define them formally.
`
	got := convertTodo(in)
	for _, want := range []string{
		"{/*",
		"TODO:",
		"Repeat integrity, consensus, transparency from protocols section.",
		"Define them formally.",
		"*/}",
	} {
		if !contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if contains(got, "<Note>") {
		t.Errorf("Note tag leaked into output:\n%s", got)
	}
}

func TestConvertTodo_EscapesCommentTerminatorInBody(t *testing.T) {
	// A stray `*/` in the body would prematurely close the MDX comment.
	// sanitizeCommentBody escapes it; verify by ensuring the output
	// has at most one `*/` (the real close).
	in := `.. todo::
   See https://example.com/*/foo for context.
`
	got := convertTodo(in)
	count := 0
	for i := 0; i+1 < len(got); i++ {
		if got[i] == '*' && got[i+1] == '/' {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one unescaped `*/` (the close); got %d:\n%s", count, got)
	}
}

// contains is a local helper so we don't import strings just for Contains
// in tests — keeps the file self-contained.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
