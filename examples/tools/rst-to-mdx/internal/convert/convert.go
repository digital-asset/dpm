// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package convert turns a reStructuredText document into a Mintlify MDX
// document. The pipeline is a sequence of text transforms, each of which
// lives in its own file alongside a focused test suite.
//
// Pipeline order matters. Transforms that wrap content into fenced blocks
// or JSX components (code blocks, admonitions) run before inline role
// transforms so we don't rewrite content that will end up inside a fence.
// Comment conversion runs last so it doesn't swallow real directives.
package convert

import (
	"fmt"
	"strings"

	"daml.com/x/dpm-components/rst-to-mdx/internal/include"
	"daml.com/x/dpm-components/rst-to-mdx/internal/labelindex"
	"daml.com/x/dpm-components/rst-to-mdx/internal/navindex"
)

// Options controls a single conversion.
type Options struct {
	// Title overrides the auto-detected page title. Empty means
	// auto-detect from the first RST heading.
	Title string
	// Description sets the frontmatter `description:` field.
	Description string
	// SourceLabel is the provenance label (typically the source RST
	// path relative to docs-website/).
	SourceLabel string
	// SourcePath is the on-disk path of the RST file being converted.
	// Used for resolving relative paths in literalinclude and images
	// (Phase 3+) and for the cross-reference resolver to prefer
	// same-version-tree label definitions.
	SourcePath string
	// LabelIndex, if non-nil, resolves `:ref:`, `:externalref:`,
	// `:subsiteref:`, and `:brokenref:` targets to concrete MDX URLs
	// via the Phase-2 label index + pathmap.
	LabelIndex *labelindex.Index
	// NavIndex, if non-nil, holds the page paths registered in the
	// target docs site's docs.json. When supplied, cross-reference
	// resolution prefers a NavIndex hit over a pathmap-derived path
	// so links land on real pages.
	NavIndex *navindex.Index
	// DocsRoot is the filesystem root of docs-website/, used to
	// resolve absolute `.. include::` and `.. literalinclude::` paths
	// (i.e. paths beginning with `/`). Optional.
	DocsRoot string
	// Strict fails the conversion on unresolved :ref:, missing
	// literalinclude targets, or unrecognized directives.
	Strict bool
}

// Result is what Convert returns: the rewritten MDX bytes plus any
// side data the caller needs to act on after conversion (currently
// just the list of image references found in the source so the CLI
// can copy assets).
type Result struct {
	// Body is the converted MDX file content.
	Body []byte
	// Images lists every `.. image::` and `.. figure::` directive
	// the converter saw. Populated regardless of `--copy-images`;
	// it's the CLI's job to act on them.
	Images []ImageRef
}

// Convert transforms an RST document into an MDX document. The byte
// output is deterministic — same input + same options produces the same
// bytes — so callers can use it for golden-file testing.
func Convert(rst []byte, opts Options) (*Result, error) {
	if len(rst) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	// Normalize line endings so every downstream transform only has to
	// worry about '\n'.
	body := strings.ReplaceAll(string(rst), "\r\n", "\n")

	// Capture image references off the RAW RST before transforms run,
	// so we still see the original directive shapes (including option
	// blocks for :alt: text). The rewriter in images.go then mutates
	// the directive lines independently — they only need to agree on
	// the basename.
	images := extractImageRefs(body, opts.SourcePath)

	// Resolve file-system includes BEFORE any transform runs so the
	// spliced content flows through the whole pipeline. literalinclude
	// is rewritten as a `.. code-block::` directive that the downstream
	// codeblocks transform handles.
	body, err := include.Resolve(body, include.Options{
		SourcePath: opts.SourcePath,
		DocsRoot:   opts.DocsRoot,
		Strict:     opts.Strict,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve includes: %w", err)
	}

	// The pipeline. Order matters — see package doc.
	body = stripCopyrightHeader(body)
	body = stripSimpleDirectives(body)
	body = stripLabels(body)
	// Collapse RST `..` comment blocks before any JSX-emitting transform
	// runs. If we let admonitions/figures/headings rewrite content that
	// lives inside an RST comment, we end up with `<Note>` / `<Frame>`
	// tags whose closes straddle the comment boundary — Mintlify then
	// errors with "unexpected closing slash" or "expected an open tag".
	body = convertComments(body)
	// `.. wip::` runs before heading detection because its body is
	// indented; dedenting first lets nested `=== underlines` register
	// as real headings downstream.
	body = convertWip(body)
	body = convertTodo(body)
	body = convertToggle(body)
	body = convertRawHTMLVideo(body)
	body = convertYoutube(body)
	body = convertTabs(body)
	body = convertTableTitle(body)
	body = convertTables(body, opts)
	body = convertHeadings(body)
	body = convertCodeBlocks(body)
	body = convertAdmonitions(body)
	body = convertGenericAdmonition(body)
	body = convertImages(body)
	// Everything from here on runs on a document that already contains
	// fenced code blocks. Wrap each transform so it only touches prose.
	body = transformOutsideFences(body, func(s string) string { return convertLinks(s, opts) })
	body = transformOutsideFences(body, convertInlineRoles)
	body = transformOutsideFences(body, convertLists)
	body = transformOutsideFences(body, convertRubric)
	body = normalizeLanguages(body)
	body = stripDoubleBackticksInFences(body)
	body = escapeMDXPlaceholders(body)
	body = cleanupWhitespace(body)

	return &Result{
		Body:   composeOutput(body, rst, opts),
		Images: images,
	}, nil
}
