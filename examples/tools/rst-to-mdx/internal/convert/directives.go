// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"regexp"
	"strings"
)

var (
	// Copyright/SPDX header at the very top of the file.
	//   ..
	//      Copyright (c) …
	//   ..
	//      SPDX-License-Identifier: …
	// is converted to an MDX comment so provenance isn't lost.
	reCopyrightHeader = regexp.MustCompile(
		`(?s)\A\.\.\s*\n\s+Copyright\s+[^\n]+\n(?:\.\.\s*\n\s+SPDX[^\n]+\n)?`)

	// :orphan: directive (whole line).
	reOrphan = regexp.MustCompile(`(?m)^:orphan:\s*$\n?`)

	// .. contents:: [title]
	//    :option: value
	//    …
	// Consumes the directive line plus any immediately-following option
	// lines (starting with three+ spaces and a colon).
	reContents = regexp.MustCompile(
		`(?m)^\.\.\s+contents::[^\n]*\n(?:[ \t]+:[^\n]+\n)*`)

	// .. toctree::
	//    :option:
	//    page1
	//    page2
	// Consumes the directive + indented block until a non-indented line.
	reToctree = regexp.MustCompile(
		`(?m)^\.\.\s+toctree::[^\n]*\n(?:[ \t]+[^\n]*\n|\s*\n)*`)

	// .. _label-name:
	reLabel = regexp.MustCompile(
		`(?m)^\.\.\s+_[A-Za-z0-9][A-Za-z0-9._\- ]*:\s*$\n?`)

)

// reTodoStart matches `.. todo::` with an optional inline summary on the
// same line. Body lines (if any) are indented under it. The inline
// portion is often a `<https://github.com/...>` issue link.
var reTodoStart = regexp.MustCompile(`^(\s*)\.\.\s+todo::(?:\s+(.*?))?\s*$`)

// stripCopyrightHeader converts the standard Canton/Daml copyright
// comment at the top of an RST file into an MDX comment so we keep the
// attribution but don't render it.
func stripCopyrightHeader(s string) string {
	m := reCopyrightHeader.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	header := m[0]
	// Extract the copyright + SPDX lines and emit them as one comment.
	var kept []string
	for _, line := range strings.Split(header, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "Copyright") || strings.HasPrefix(t, "SPDX") {
			kept = append(kept, t)
		}
	}
	rest := strings.TrimLeft(s[len(header):], "\n")
	replacement := ""
	if len(kept) > 0 {
		replacement = "{/* " + strings.Join(kept, " — ") + " */}\n\n"
	}
	return replacement + rest
}

// stripSimpleDirectives removes directives that have no MDX counterpart:
// contents (Mintlify auto-TOCs), toctree (docs.json handles nav), orphan
// (not expressible in MDX). `.. todo::` is intentionally NOT stripped —
// see convertTodo, which renders todos as a visible `<Note>` so readers
// can see pending work and follow any linked issue.
func stripSimpleDirectives(s string) string {
	s = reOrphan.ReplaceAllString(s, "")
	s = reContents.ReplaceAllString(s, "")
	s = reToctree.ReplaceAllString(s, "")
	return s
}

// stripLabels removes `.. _label-name:` anchors. Their targets will be
// resolved by the Phase-2 label index; the label line itself produces no
// MDX output because Mintlify auto-generates anchors from headings.
func stripLabels(s string) string {
	return reLabel.ReplaceAllString(s, "")
}

// reRubric matches `.. rubric:: Heading` — a non-TOC inline heading. We
// emit it as bold text so it still stands out without appearing in the
// right-sidebar TOC.
var reRubric = regexp.MustCompile(`(?m)^\.\.\s+rubric::\s+(.+)$`)

// reTableTitle matches `.. table:: Title` — an RST wrapper that
// decorates a following grid/list/csv table with a title. We emit the
// title as bold above the table that follows.
var reTableTitle = regexp.MustCompile(`(?m)^(\s*)\.\.\s+table::\s+(.+?)\s*$`)

// reYoutubeStart matches `.. youtube:: <video-id>` opening the
// directive. The line walker below consumes the directive plus any
// indented `:option:` lines that follow.
var reYoutubeStart = regexp.MustCompile(
	`^(\s*)\.\.\s+youtube::\s+([A-Za-z0-9_\-]+)\s*$`)

// reAnyHeading matches an RST underline heading shape so we can use the
// most recent one as the iframe title.
var reAnyHeadingTitle = regexp.MustCompile(`^([=\-~^"]{3,})\s*$`)

// reToggleStart matches `.. toggle::` — the sphinx-togglebutton directive
// that wraps indented content in a click-to-expand button. An optional
// argument on the same line becomes the accordion title; otherwise we
// use a generic default. Mintlify's `<Accordion>` is the natural target
// because it has the same expand/collapse UX.
var reToggleStart = regexp.MustCompile(`^(\s*)\.\.\s+toggle::\s*(.*)$`)

// reWipStart matches `.. wip::` — Canton's custom Sphinx directive
// for "work in progress" content. Body lines are indented under the
// directive; convertWip wraps them in an Info admonition with a
// `**WIP:**` prefix and dedents the body so any nested headings,
// code blocks, and admonitions flow through the rest of the pipeline
// like ordinary content. We treat the WIP block as a hint to the
// reader, not as content to drop.
var reWipStart = regexp.MustCompile(`^(\s*)\.\.\s+wip::\s*$`)

// reRawHTMLStart matches `.. raw:: html`. Sphinx's `raw` directive
// passes its indented body straight through to the configured output
// format. Most uses in the corpus are `<video>` tags whose unquoted
// attributes (`width=640`) and inline `onclick` handlers don't parse
// as JSX. convertRawHTMLVideo recognizes the video pattern and emits
// a JSX-clean `<video>` with extracted `<source>` children. Other
// raw HTML passes through unchanged.
var reRawHTMLStart = regexp.MustCompile(`^(\s*)\.\.\s+raw::\s+html\s*$`)

// reHTMLVideoTag is just an existence check for a `<video>` element
// inside a raw block. Attribute parsing happens with separate regexes.
var reHTMLVideoTag = regexp.MustCompile(`<video\b`)

// reHTMLSourceTag captures the `src` and optional `type` of every
// `<source>` child of a `<video>`. Attributes can appear in any order
// so we run two narrow searches per match instead of one positional
// pattern.
var reHTMLSourceTag = regexp.MustCompile(`(?i)<source\b[^>]*?>`)
var reHTMLAttrSrc = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
var reHTMLAttrType = regexp.MustCompile(`(?i)\btype\s*=\s*["']([^"']+)["']`)
var reHTMLAttrWidth = regexp.MustCompile(`(?i)\bwidth\s*=\s*["']?(\d+)["']?`)
var reHTMLAttrHeight = regexp.MustCompile(`(?i)\bheight\s*=\s*["']?(\d+)["']?`)

// convertRubric turns `.. rubric:: Text` into `**Text**`.
func convertRubric(s string) string {
	return reRubric.ReplaceAllString(s, "**$1**")
}

// convertWip rewrites `.. wip::` blocks into `<Info>` admonitions
// with a `**WIP:**` prefix. The directive's body is dedented so
// nested headings, code blocks, and other constructs survive the
// rest of the pipeline rather than getting stuck inside an indented
// block-quote rendering.
//
// Two shapes occur in the corpus:
//
//	.. wip::
//	   Body line 1.
//	   Body line 2.
//
//	.. wip:: Inline summary on the same line.
//	   Optional body lines that follow.
//
// We handle both. The directive itself never produces literal output
// — we just emit `<Info>` … `</Info>` brackets around the dedented
// body.
func convertWip(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reWipStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		i++

		// Collect every line that's indented further than the
		// `.. wip::` directive. Blank lines stay in if the next non-
		// blank line is still inside the block.
		var body []string
		for i < len(lines) {
			cur := lines[i]
			if strings.TrimSpace(cur) == "" {
				body = append(body, "")
				i++
				continue
			}
			if !strings.HasPrefix(cur, indent) || !deeperIndentForWip(cur, indent) {
				break
			}
			body = append(body, cur)
			i++
		}
		// Trim trailing blank lines from the body so the closing
		// `</Info>` sits flush against the content.
		for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
		dedented := dedentForWip(body)

		out = append(out, indent+"<Info>")
		out = append(out, indent+"**WIP:**")
		out = append(out, "")
		out = append(out, dedented...)
		out = append(out, indent+"</Info>")
	}
	return strings.Join(out, "\n")
}

// convertToggle rewrites `.. toggle::` blocks into `<Accordion>` so the
// click-to-expand UX from sphinx-togglebutton survives the migration.
// The body is dedented so any nested `.. code-block::` (the dominant
// use in the corpus) reaches convertCodeBlocks at column 0 and emits as
// a regular fenced block inside the accordion.
//
// Title preference, in order:
//
//  1. an explicit argument: `.. toggle:: V3 implementation`
//  2. the literal "Show example" — descriptive enough for the corpus
//     where toggles consistently hide long code samples
func convertToggle(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reToggleStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" {
			title = "Show example"
		}
		i++

		var body []string
		for i < len(lines) {
			cur := lines[i]
			if strings.TrimSpace(cur) == "" {
				body = append(body, "")
				i++
				continue
			}
			if !strings.HasPrefix(cur, indent) || !deeperIndentForWip(cur, indent) {
				break
			}
			body = append(body, cur)
			i++
		}
		for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
		dedented := dedentForWip(body)

		out = append(out, indent+`<Accordion title="`+escapeAttr(title)+`">`)
		out = append(out, "")
		out = append(out, dedented...)
		out = append(out, "")
		out = append(out, indent+"</Accordion>")
	}
	return strings.Join(out, "\n")
}

// convertRawHTMLVideo rewrites `.. raw:: html` blocks that wrap a
// `<video>` element into a JSX-clean `<video>` with extracted `<source>`
// children. It preserves `width`/`height` (forcing them into quoted
// attributes) and drops inline `onclick` handlers — the standard
// `controls` attribute already provides click-to-play behavior in
// every modern browser. Raw blocks that don't contain `<video>` are
// re-emitted unchanged so a human can decide how to handle them.
func convertRawHTMLVideo(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reRawHTMLStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		startIdx := i
		i++

		// Skip blank lines between the directive and its body.
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}

		// Collect the indented body — every line strictly more indented
		// than the directive itself. Blank lines stay in the body only
		// when followed by another indented line; trailing blanks are
		// left for the outer loop so paragraphs after the directive
		// keep their separator.
		var body []string
		for i < len(lines) {
			cur := lines[i]
			if strings.TrimSpace(cur) == "" {
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j >= len(lines) || len(leadingWS(lines[j])) <= len(indent) {
					break
				}
				body = append(body, "")
				i++
				continue
			}
			lws := leadingWS(cur)
			if len(lws) <= len(indent) {
				break
			}
			body = append(body, cur)
			i++
		}

		joined := strings.Join(body, "\n")
		if !reHTMLVideoTag.MatchString(joined) {
			// Not a video — re-emit the directive untouched.
			out = append(out, lines[startIdx:i]...)
			continue
		}

		type source struct{ src, mediaType string }
		var sources []source
		for _, raw := range reHTMLSourceTag.FindAllString(joined, -1) {
			srcMatch := reHTMLAttrSrc.FindStringSubmatch(raw)
			if srcMatch == nil {
				continue
			}
			s := source{src: srcMatch[1]}
			if tm := reHTMLAttrType.FindStringSubmatch(raw); tm != nil {
				s.mediaType = tm[1]
			}
			sources = append(sources, s)
		}
		if len(sources) == 0 {
			// `<video>` with no usable `<source>` — punt back to the
			// original block so nothing is silently dropped.
			out = append(out, lines[startIdx:i]...)
			continue
		}

		attrs := "controls"
		if wm := reHTMLAttrWidth.FindStringSubmatch(joined); wm != nil {
			attrs += ` width="` + wm[1] + `"`
		}
		if hm := reHTMLAttrHeight.FindStringSubmatch(joined); hm != nil {
			attrs += ` height="` + hm[1] + `"`
		}

		// Single source: put `src` directly on the `<video>` element.
		// Mintlify's MDX→React pipeline doesn't reliably propagate
		// `<source>` children, so the nested form renders as "video
		// source is missing" even with a valid URL. For multiple
		// sources we keep the children pattern because that's the only
		// way to offer real format alternatives.
		if len(sources) == 1 {
			out = append(out, indent+`<video src="`+sources[0].src+`" `+attrs+` />`)
			continue
		}
		out = append(out, indent+"<video "+attrs+">")
		for _, s := range sources {
			line := indent + `  <source src="` + s.src + `"`
			if s.mediaType != "" {
				line += ` type="` + s.mediaType + `"`
			}
			line += ` />`
			out = append(out, line)
		}
		out = append(out, indent+"</video>")
	}
	return strings.Join(out, "\n")
}

// convertTodo rewrites `.. todo::` blocks into MDX comments
// (`{/* … */}`) so the work item is preserved in the source for
// developers but never rendered to docs readers. The body is dedented
// before wrapping so any nested constructs (links, fences, etc.) keep
// their natural shape inside the comment, and any `*/` in the body is
// escaped via sanitizeCommentBody so a stray sequence can't terminate
// the comment early.
//
// Two shapes occur in the corpus:
//
//	.. todo:: <https://github.com/DACH-NY/canton/issues/12345>
//	   Summary of the work.
//	   Optional more body.
//
//	.. todo::
//	   Body line 1.
//	   Body line 2.
//
// Single-line todos (inline summary, no indented body) emit a one-line
// comment; everything else emits a multi-line `{/* … */}` block whose
// `*/}` close lines up with the directive's original indent.
func convertTodo(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reTodoStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		inline := strings.TrimSpace(m[2])
		i++

		var body []string
		for i < len(lines) {
			cur := lines[i]
			if strings.TrimSpace(cur) == "" {
				body = append(body, "")
				i++
				continue
			}
			if !strings.HasPrefix(cur, indent) || !deeperIndentForWip(cur, indent) {
				break
			}
			body = append(body, cur)
			i++
		}
		for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
		dedented := dedentForWip(body)

		// Build the comment payload: a "TODO:" header (with the optional
		// inline summary on the same line) followed by any dedented body
		// lines.
		var content []string
		if inline != "" {
			content = append(content, "TODO: "+inline)
		} else {
			content = append(content, "TODO:")
		}
		if len(dedented) > 0 {
			content = append(content, "")
			content = append(content, dedented...)
		}
		joined := sanitizeCommentBody(strings.Join(content, "\n"))

		// Single-line todo with no indented body collapses to one
		// `{/* TODO: … */}` line; otherwise emit a multi-line block so
		// readers see no rendered output and the close lands cleanly
		// after the original directive's content.
		if !strings.Contains(joined, "\n") {
			out = append(out, indent+"{/* "+joined+" */}")
			continue
		}
		out = append(out, indent+"{/*")
		out = append(out, joined)
		out = append(out, indent+"*/}")
	}
	return strings.Join(out, "\n")
}

// deeperIndentForWip reports whether a line is indented strictly
// further than the parent directive. Local copy because the codeblocks
// helper has slightly different semantics.
func deeperIndentForWip(line, parent string) bool {
	if len(line) <= len(parent) {
		return false
	}
	for i := 0; i < len(parent); i++ {
		if line[i] != parent[i] {
			return false
		}
	}
	// At least one further whitespace char before content.
	c := line[len(parent)]
	return c == ' ' || c == '\t'
}

// dedentForWip strips the minimum common leading-whitespace run from
// every non-blank body line so nested headings (== / -- underlines)
// and other indentation-sensitive constructs reach the rest of the
// pipeline at column 0.
func dedentForWip(lines []string) []string {
	min := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := 0
		for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
			n++
		}
		if min == -1 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= min {
			out[i] = line[min:]
		} else {
			out[i] = line
		}
	}
	return out
}

// convertTableTitle turns `.. table:: Title` into a bold title line.
// The directive's content (the table itself) is already separately
// converted by the tables transform, so we only need to handle the
// wrapper line here.
func convertTableTitle(s string) string {
	return reTableTitle.ReplaceAllString(s, "$1**$2**")
}

// convertYoutube turns `.. youtube:: <video-id>` into an iframe embed
// matching the format the human migrators have been using. The iframe
// `title` attribute uses, in order of preference:
//   1. an explicit `:title:` option on the directive
//   2. the closest preceding heading text (any level)
//   3. the literal "YouTube video" fallback
func convertYoutube(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	var lastHeading string

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Track the most recent heading. RST headings here are still
		// in their `Title\n=======` form; we capture the title line by
		// looking ahead one position for the underline marker.
		if i+1 < len(lines) && reAnyHeadingTitle.MatchString(lines[i+1]) &&
			strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimLeft(line, " \t"), "..") {
			if !reAnyHeadingTitle.MatchString(line) {
				lastHeading = strings.TrimSpace(line)
			}
		}

		m := reYoutubeStart.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			i++
			continue
		}
		indent := m[1]
		videoID := m[2]
		i++
		opts, consumed := readOptions(lines[i:])
		i += consumed

		title := opts["title"]
		if title == "" {
			title = lastHeading
		}
		if title == "" {
			title = "YouTube video"
		}
		out = append(out,
			indent+`<iframe`,
			indent+`  width="560"`,
			indent+`  height="315"`,
			indent+`  src="https://www.youtube.com/embed/`+videoID+`"`,
			indent+`  title="`+escapeAttr(title)+`"`,
			indent+`  frameBorder="0"`,
			indent+`  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"`,
			indent+`  allowFullScreen`,
			indent+`/>`,
		)
	}
	return strings.Join(out, "\n")
}

// readOptions consumes indented `:name: value` lines following a
// directive and returns the parsed map plus the count of input lines
// consumed.
func readOptions(lines []string) (map[string]string, int) {
	opts := map[string]string{}
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			break
		}
		m := reOption.FindStringSubmatch(line)
		if m == nil {
			break
		}
		opts[strings.ToLower(m[1])] = strings.TrimSpace(m[2])
		i++
	}
	return opts, i
}
