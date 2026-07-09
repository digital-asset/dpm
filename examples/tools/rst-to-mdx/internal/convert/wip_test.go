// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"strings"
	"testing"
)

func TestConvertWip_BasicBlock(t *testing.T) {
	in := `Heading
=======

.. wip::
   Trim this section.
   Add more content.

After.
`
	got := convertWip(in)
	for _, want := range []string{
		"<Info>",
		"**WIP:**",
		"Trim this section.",
		"Add more content.",
		"</Info>",
		"After.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, ".. wip::") {
		t.Errorf("directive not consumed:\n%s", got)
	}
}

func TestConvertWip_DedentsNestedHeadings(t *testing.T) {
	// The body contains an underlined heading. Without dedent, the
	// underline still has 3-space indent and convertHeadings won't
	// match it.
	in := `.. wip::

   Canton 101
   ----------

   Some content.
`
	got := convertWip(in)
	// The dedented heading title should now be at column 0.
	if !strings.Contains(got, "\nCanton 101\n----------\n") {
		t.Errorf("body not dedented; got:\n%s", got)
	}
}

func TestConvertWip_PassthroughWhenAbsent(t *testing.T) {
	in := "No wip directive here.\n\nJust prose.\n"
	if got := convertWip(in); got != in {
		t.Errorf("unrelated content was modified:\n%s", got)
	}
}

func TestConvertWip_HandlesAdjacentBlankLines(t *testing.T) {
	in := `.. wip::

   First paragraph.

   Second paragraph.

Done.
`
	got := convertWip(in)
	for _, want := range []string{"First paragraph.", "Second paragraph.", "Done."} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
