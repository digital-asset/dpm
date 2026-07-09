// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractImageRefs(t *testing.T) {
	src := filepath.Join(string(filepath.Separator), "src", "page.rst")
	rst := `Some prose.

.. image:: images/01-allow-direnv.png
   :alt: allow direnv
   :width: 600px

More prose.

.. figure:: diagrams/arch.png
   :alt: architecture

   The high-level architecture.
`
	refs := extractImageRefs(rst, src)
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d: %+v", len(refs), refs)
	}

	first := refs[0]
	if first.SourceRel != "images/01-allow-direnv.png" {
		t.Errorf("SourceRel = %q", first.SourceRel)
	}
	if !strings.HasSuffix(first.SourceAbs, "/src/images/01-allow-direnv.png") {
		t.Errorf("SourceAbs = %q", first.SourceAbs)
	}
	if first.TargetRel != filepath.Join("images", "docs_website", "01-allow-direnv.png") {
		t.Errorf("TargetRel = %q", first.TargetRel)
	}
	if first.Alt != "allow direnv" {
		t.Errorf("Alt = %q", first.Alt)
	}

	second := refs[1]
	if second.SourceRel != "diagrams/arch.png" {
		t.Errorf("figure SourceRel = %q", second.SourceRel)
	}
	if second.Alt != "architecture" {
		t.Errorf("figure Alt = %q", second.Alt)
	}
}

func TestExtractImageRefs_NoSourcePath_AbsEmpty(t *testing.T) {
	refs := extractImageRefs(".. image:: foo.png\n", "")
	if len(refs) != 1 {
		t.Fatalf("want 1, got %d", len(refs))
	}
	if refs[0].SourceAbs != "" {
		t.Errorf("expected empty SourceAbs, got %q", refs[0].SourceAbs)
	}
}

func TestExtractImageRefs_NoneFound(t *testing.T) {
	refs := extractImageRefs("just prose, no images", "")
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}
