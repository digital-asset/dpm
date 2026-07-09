// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"

	"daml.com/x/dpm-components/rst-to-mdx/internal/labelindex"
	"daml.com/x/dpm-components/rst-to-mdx/internal/pathmap"
)

// RST has several link forms:
//
//	`text <url>`__         anonymous external link
//	`text <url>`_          named external link
//	:ref:`label`           internal reference by label
//	:ref:`text <label>`    internal reference with display text
//	:doc:`path`            internal reference by doc path
//	:doc:`text <path>`     internal reference with display text
//	:externalref:`text <label>`  cross-subsite reference
//	:subsiteref:`text <label>`   cross-subsite reference
//	:download:`text <file>` file download link
//
// External links and :doc: are resolvable locally; the Sphinx label
// roles resolve against a LabelIndex (see package labelindex). When the
// index is provided and the label is found, the link target becomes a
// real `/<path>#<anchor>` URL (docs-main-relative — Mintlify serves
// docs-main/ as site root, so the prefix is omitted from the URL). When
// the label isn't found (or the index is absent), the link falls back
// to a `#TODO-…` marker.

// All link regexes constrain their character classes to exclude `\n`
// so a single backtick or angle bracket at the start of a match can't
// pair up with one many lines later. RST link syntax never spans
// multiple physical lines, so the constraint is correct as well as
// safe.
var (
	reExternalLinkAnon  = regexp.MustCompile("`([^<`\n]+?)\\s*<([^>\n]+)>`__")
	reExternalLinkNamed = regexp.MustCompile("`([^<`\n]+?)\\s*<([^>\n]+)>`_")
	reDownload          = regexp.MustCompile(":download:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reRefWithText       = regexp.MustCompile(":ref:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reRefBare           = regexp.MustCompile(":ref:`([^`\n]+)`")
	reDocWithText       = regexp.MustCompile(":doc:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reDocBare           = regexp.MustCompile(":doc:`([^`\n]+)`")
	reExternalrefWText  = regexp.MustCompile(":externalref:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reExternalrefBare   = regexp.MustCompile(":externalref:`([^`\n]+)`")
	reSubsiterefWText   = regexp.MustCompile(":subsiteref:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reSubsiterefBare    = regexp.MustCompile(":subsiteref:`([^`\n]+)`")
	reBrokenrefWText    = regexp.MustCompile(":brokenref:`([^<`\n]+?)\\s*<([^>\n]+)>`")
	reBrokenrefBare     = regexp.MustCompile(":brokenref:`([^`\n]+)`")

	// reAutolink matches a bare URL autolink, `<https://...>`,
	// `<http://...>`, or `<mailto:...>`. RST and CommonMark accept this
	// shorthand, but MDX parses `<` as the start of a JSX tag and aborts
	// on the `/` in `https://`. We rewrite to `[url](url)` so the URL
	// renders identically and the MDX parser sees a normal markdown link.
	reAutolink = regexp.MustCompile(`<((?:https?|mailto):[^\s<>]+)>`)
)

// convertLinks rewrites RST link forms into MDX equivalents. When
// opts.LabelIndex is set, label-based refs resolve to concrete paths.
// Otherwise they become `#TODO-resolve-…` markers a human can grep for.
func convertLinks(s string, opts Options) string {
	s = reExternalLinkAnon.ReplaceAllString(s, "[$1]($2)")
	s = reExternalLinkNamed.ReplaceAllString(s, "[$1]($2)")
	s = reDownload.ReplaceAllString(s, "[$1]($2)")

	s = resolveRefWithText(s, reRefWithText, opts, "ref")
	s = resolveRefBare(s, reRefBare, opts, "ref")
	s = resolveRefWithText(s, reExternalrefWText, opts, "externalref")
	s = resolveRefBare(s, reExternalrefBare, opts, "externalref")
	s = resolveRefWithText(s, reSubsiterefWText, opts, "subsiteref")
	s = resolveRefBare(s, reSubsiterefBare, opts, "subsiteref")

	// :doc: is a path-based reference, not a label. We convert it
	// directly without consulting the label index.
	s = reDocWithText.ReplaceAllString(s, "[$1]($2)")
	s = reDocBare.ReplaceAllString(s, "[$1]($1)")

	// :brokenref: is always unresolved by design — it's an explicit
	// author annotation that a link is known-broken.
	s = reBrokenrefWText.ReplaceAllString(s, "[$1](#TODO-broken-ref-$2)")
	s = reBrokenrefBare.ReplaceAllString(s, "[$1](#TODO-broken-ref-$1)")

	// Bare URL autolinks survive into the body of `.. todo::` notes and
	// other prose. MDX rejects `<https://...>` as a malformed JSX tag, so
	// rewrite to `[url](url)`. Run last so any backtick-quoted RST link
	// form has already been consumed.
	s = reAutolink.ReplaceAllString(s, "[$1]($1)")

	return s
}

// resolveRefWithText handles `:kind:`text <label>``. `text` is the
// display label the reader sees; `label` is the identifier we look up.
func resolveRefWithText(s string, re *regexp.Regexp, opts Options, kind string) string {
	return re.ReplaceAllStringFunc(s, func(match string) string {
		m := re.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		text := strings.TrimSpace(m[1])
		label := strings.TrimSpace(m[2])
		if url, ok := resolveLabel(label, opts); ok {
			return "[" + text + "](" + url + ")"
		}
		return "[" + text + "](#TODO-resolve-" + kind + "-" + label + ")"
	})
}

// resolveRefBare handles `:kind:`label``. The label doubles as the
// display text until the index gives us a real heading.
func resolveRefBare(s string, re *regexp.Regexp, opts Options, kind string) string {
	return re.ReplaceAllStringFunc(s, func(match string) string {
		m := re.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		label := strings.TrimSpace(m[1])
		if url, heading, ok := resolveLabelWithHeading(label, opts); ok {
			return "[" + heading + "](" + url + ")"
		}
		return "[" + label + "](#TODO-resolve-" + kind + "-" + label + ")"
	})
}

// resolveLabel resolves label to a docs-main-relative `/<path>#<anchor>` URL.
// Returns ok=false if the index is absent, the label is unknown, or
// the label's file isn't in a mapped subtree.
func resolveLabel(label string, opts Options) (string, bool) {
	url, _, ok := resolveLabelWithHeading(label, opts)
	return url, ok
}

func resolveLabelWithHeading(label string, opts Options) (string, string, bool) {
	if opts.LabelIndex == nil {
		return "", "", false
	}
	loc, ok := opts.LabelIndex.Resolve(label, opts.SourcePath)
	if !ok {
		return "", "", false
	}

	// Preference order:
	//   1. docs.json hit — the target page exists in the live site,
	//      either at the path the pathmap derives or at a related
	//      slug (humans organize differently than the pathmap rule).
	//   2. pathmap-derived path — algorithmic fallback that may or
	//      may not point at a registered page.
	var url string
	if opts.NavIndex != nil {
		if page := opts.NavIndex.BestMatch(stripDocsWebsitePrefix(loc.RSTPath)); page != "" {
			// Mintlify serves docs-main/ as site root, so links are
			// docs-main-relative without that prefix.
			url = "/" + page
		} else if derived, ok := pathmap.Derive(loc.RSTPath); ok {
			// Fall back to pathmap, but only emit it as a hit if the
			// derived target is actually present in NavIndex. This
			// stops us from generating links to pages that don't
			// exist in the published site.
			if opts.NavIndex.HasPage(string(derived)) {
				url = derived.URL()
			}
		}
	}
	if url == "" {
		derived, ok := pathmap.Derive(loc.RSTPath)
		if !ok {
			return "", "", false
		}
		url = derived.URL()
	}

	// Page-level labels (the ones that anchor the file's FIRST
	// heading) shouldn't get a `#fragment` — the link already
	// targets that page, and Mintlify auto-generates the title
	// anchor anyway. Section-level labels still need the anchor so
	// the reader lands on the right place inside the page.
	if !loc.IsPageTitle {
		anchor := headingToAnchor(loc.Heading)
		if anchor != "" {
			url += "#" + anchor
		}
	}
	return url, loc.Heading, true
}

// stripDocsWebsitePrefix removes the leading docs-website/docs/replicated/
// segment from an absolute RST path so the remainder can be fed to
// NavIndex.BestMatch. NavIndex compares path components and we want the
// "interesting" parts (canton/version/participant/...) not the corpus
// scaffolding.
func stripDocsWebsitePrefix(p string) string {
	marker := "docs-website/docs/replicated/"
	if i := strings.Index(p, marker); i >= 0 {
		return p[i+len(marker):]
	}
	return p
}

// headingToAnchor converts a heading string to the anchor id Mintlify
// auto-generates: lowercase, spaces and punctuation collapsed to hyphens.
func headingToAnchor(heading string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// Compile-time usage check so the import doesn't go stale if someone
// removes all resolver calls during refactoring.
var _ = (&labelindex.Index{})
