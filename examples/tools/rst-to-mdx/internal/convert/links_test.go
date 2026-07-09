// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/dpm-components/rst-to-mdx/internal/labelindex"
)

func TestConvertLinks_StaticForms(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "anonymous external link",
			in:   "`Docker Desktop <https://www.docker.com/>`__",
			want: `[Docker Desktop](https://www.docker.com/)`,
		},
		{
			name: "named external link",
			in:   "`Docker Desktop <https://www.docker.com/>`_",
			want: `[Docker Desktop](https://www.docker.com/)`,
		},
		{
			name: "doc directive with text",
			in:   ":doc:`Canton Console <./canton-console>`",
			want: `[Canton Console](./canton-console)`,
		},
		{
			name: "download link",
			in:   ":download:`AsyncAPI <api/asyncapi.yaml>`",
			want: `[AsyncAPI](api/asyncapi.yaml)`,
		},
		{
			name: "brokenref always unresolved",
			in:   ":brokenref:`Known broken <some-label>`",
			want: `[Known broken](#TODO-broken-ref-some-label)`,
		},
		{
			name: "https autolink becomes markdown link",
			in:   "see <https://github.com/DACH-NY/canton/issues/25689> for status",
			want: "see [https://github.com/DACH-NY/canton/issues/25689](https://github.com/DACH-NY/canton/issues/25689) for status",
		},
		{
			name: "http autolink",
			in:   "<http://example.com/x>",
			want: "[http://example.com/x](http://example.com/x)",
		},
		{
			name: "mailto autolink",
			in:   "contact <mailto:ops@example.com>",
			want: "contact [mailto:ops@example.com](mailto:ops@example.com)",
		},
		{
			name: "JSX tag without scheme is left alone",
			in:   "<Note>body</Note>",
			want: "<Note>body</Note>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLinks(tc.in, Options{})
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}

func TestConvertLinks_UnresolvedPlaceholders(t *testing.T) {
	// No LabelIndex provided; refs should fall back to TODO markers.
	cases := []struct {
		name, in, want string
	}{
		{
			name: "ref bare placeholder",
			in:   ":ref:`quickstart-explore-the-demo`",
			want: `[quickstart-explore-the-demo](#TODO-resolve-ref-quickstart-explore-the-demo)`,
		},
		{
			name: "ref with text placeholder",
			in:   ":ref:`explore the demo <quickstart-explore-the-demo>`",
			want: `[explore the demo](#TODO-resolve-ref-quickstart-explore-the-demo)`,
		},
		{
			name: "externalref placeholder",
			in:   ":externalref:`Canton docker <install-with-docker>`",
			want: `[Canton docker](#TODO-resolve-externalref-install-with-docker)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLinks(tc.in, Options{})
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}

func TestConvertLinks_Resolved(t *testing.T) {
	// Build a tiny docs-website-like fixture so the pathmap rules
	// activate against a real canton/participant tree.
	root := t.TempDir()
	rstRel := "docs-website/docs/replicated/canton/3.5/participant/tutorials/getting_started.rst"
	rstPath := filepath.Join(root, rstRel)
	if err := os.MkdirAll(filepath.Dir(rstPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `.. _canton-getting-started:

Getting Started
===============

Some content.
`
	if err := os.WriteFile(rstPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := labelindex.Build(root)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	cases := []struct {
		name, in, want string
	}{
		{
			name: "ref bare resolves to pretty URL without redundant page anchor",
			in:   ":ref:`canton-getting-started`",
			// Label is defined right before the FIRST heading in the
			// fixture, so it's a page-level reference and the URL
			// fragment is redundant. Mintlify serves docs-main/ as
			// site root so the URL has no docs-main/ prefix.
			want: "[Getting Started](/appdev/tutorials/canton-getting-started)",
		},
		{
			name: "ref with text preserves display text",
			in:   ":ref:`tutorial intro <canton-getting-started>`",
			want: "[tutorial intro](/appdev/tutorials/canton-getting-started)",
		},
	}
	opts := Options{
		LabelIndex: idx,
		SourcePath: rstPath,
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLinks(tc.in, opts)
			if got != tc.want {
				t.Errorf("\nwant %q\n got %q", tc.want, got)
			}
		})
	}
}

func TestHeadingToAnchor(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Getting Started", "getting-started"},
		{"Canton Admin APIs", "canton-admin-apis"},
		{"What is a `Daml Contract`?", "what-is-a-daml-contract"},
		{"  Trimmed & weird !!  ", "trimmed-weird"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := headingToAnchor(tc.in); got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}
