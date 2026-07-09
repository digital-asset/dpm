// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestDetectDescription_FromOverview(t *testing.T) {
	body := `## Some Page

## Overview

This guide walks through installing the Canton Network Quickstart on macOS and Linux.
There is more detail below.

## Prerequisites

Other content here.
`
	got := detectDescription(body)
	want := "This guide walks through installing the Canton Network Quickstart on macOS and Linux."
	if got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestDetectDescription_FromIntroduction(t *testing.T) {
	body := `## Page

## Introduction

The Quickstart application helps developers learn Canton.
`
	want := "The Quickstart application helps developers learn Canton."
	if got := detectDescription(body); got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestDetectDescription_OmittedWhenAbsent(t *testing.T) {
	body := `## Page

## Prerequisites

This page lists requirements.
`
	if got := detectDescription(body); got != "" {
		t.Errorf("expected empty (no overview/introduction); got %q", got)
	}
}

func TestDetectDescription_SkipsAdmonitionsAndCode(t *testing.T) {
	body := `## Page

## Overview

<Note>
You should read this carefully.
</Note>

` + "```bash" + `
echo not-this
` + "```" + `

The actual narrative starts here.
`
	want := "The actual narrative starts here."
	if got := detectDescription(body); got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestDetectDescription_StopsAtNextHeadingAtSameLevel(t *testing.T) {
	body := `## Top

## Overview

### Subsection

Content of subsection (still inside Overview).

## Some Other Section

This sentence should NOT be picked.
`
	got := detectDescription(body)
	want := "Content of subsection (still inside Overview)."
	if got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestDetectDescription_CaseInsensitive(t *testing.T) {
	body := `## OVERVIEW

This sentence wins.
`
	if got := detectDescription(body); got == "" {
		t.Errorf("expected case-insensitive match; got empty")
	}
}

func TestDetectDescription_SkipsListsAndTables(t *testing.T) {
	body := `## Overview

- bullet one
- bullet two

| col | col |
| --- | --- |

A real prose sentence appears here.
`
	want := "A real prose sentence appears here."
	if got := detectDescription(body); got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestFirstSentence_Abbreviations(t *testing.T) {
	in := "We support i.e. Linux, macOS, etc. The list grows."
	got := firstSentence(in)
	want := "We support i.e. Linux, macOS, etc."
	if got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestStripInlineMarkdown(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain text passes through", "plain text passes through"},
		{"text with **bold** word", "text with bold word"},
		{"text with *italic* word", "text with italic word"},
		{"text with `code` word", "text with code word"},
		{"text with [a link](https://example.com) embedded", "text with a link embedded"},
		{`escaped \<scheme\> stays`, "escaped <scheme> stays"},
		{`bracket \[\<x\>\] stays`, "bracket [<x>] stays"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := stripInlineMarkdown(tc.in); got != tc.want {
				t.Errorf("\nwant %q\n got %q", tc.want, got)
			}
		})
	}
}

func TestDetectDescription_StripsBoldAndCode(t *testing.T) {
	body := "## Overview\n\nThe **Quickstart** uses `dpm` to set up scaffolding.\n"
	want := "The Quickstart uses dpm to set up scaffolding."
	if got := detectDescription(body); got != want {
		t.Errorf("\nwant %q\n got %q", want, got)
	}
}

func TestClampDescription_Truncates(t *testing.T) {
	long := "This is a very long sentence that runs on and on and contains so many words that it eventually exceeds the description limit and needs to be truncated at a sensible word boundary somewhere in the middle of all this prose."
	got := clampDescription(long)
	if len(got) > 220 {
		t.Errorf("clamp didn't shorten: len=%d", len(got))
	}
	if got[len(got)-len("…"):] != "…" {
		t.Errorf("expected ellipsis suffix; got %q", got)
	}
}
