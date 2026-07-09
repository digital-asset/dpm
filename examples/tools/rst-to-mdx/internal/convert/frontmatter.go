// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"fmt"
	"regexp"
	"strings"
)

// composeOutput wraps the transformed body with YAML frontmatter and
// COPIED_START/COPIED_END provenance markers. The hash is taken from the
// untransformed source bytes so drift detection matches the RST on disk.
func composeOutput(body string, sourceBytes []byte, opts Options) []byte {
	title := opts.Title
	if title == "" {
		title = detectTitle(body)
	}
	if title == "" {
		title = "Untitled"
	}

	description := opts.Description
	if description == "" {
		description = detectDescription(body)
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	if description != "" {
		fmt.Fprintf(&b, "description: %q\n", description)
	}
	b.WriteString("---\n\n")

	if opts.SourceLabel != "" {
		fmt.Fprintf(&b, "{/* COPIED_START source=%q hash=%q */}\n\n",
			opts.SourceLabel, hash8(sourceBytes))
	}

	body = stripDuplicateTitle(body, title)
	b.WriteString(strings.TrimRight(body, "\n"))
	b.WriteString("\n")

	if opts.SourceLabel != "" {
		b.WriteString("\n{/* COPIED_END */}\n")
	}

	return []byte(b.String())
}

// stripDuplicateTitle removes the first heading from the body when it
// matches the frontmatter title exactly. Mintlify renders the
// frontmatter title as the page heading, so a duplicate H1 in the body
// produces the title twice.
func stripDuplicateTitle(body, title string) string {
	m := firstHeading.FindStringIndex(body)
	if m == nil {
		return body
	}
	headingLine := body[m[0]:m[1]]
	sub := firstHeading.FindStringSubmatch(headingLine)
	if sub == nil {
		return body
	}
	if strings.EqualFold(strings.TrimSpace(sub[1]), strings.TrimSpace(title)) {
		// Remove the heading line and any trailing blank line.
		after := body[m[1]:]
		after = strings.TrimLeft(after, "\n")
		return body[:m[0]] + after
	}
	return body
}

// firstHeading matches the first markdown ATX heading at any level.
// Many RST files in the corpus open with a single underline-only `===`
// title (which our convention maps to H2, since the overlined form is
// reserved for H1). Falling back to any heading keeps the frontmatter
// title meaningful instead of defaulting to "Untitled".
var firstHeading = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)

// detectTitle returns the text of the first heading in the converted
// body, regardless of level. Mintlify renders the frontmatter title
// above the body, so we want it to reflect the page topic even when
// no H1 is present.
func detectTitle(body string) string {
	m := firstHeading.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// reIntroSection matches a heading whose text is one of the recognised
// "introductory" labels. Case-insensitive. Heading level is captured so
// we can find the section's content (everything until the next heading
// at the same or shallower level).
var reIntroSection = regexp.MustCompile(
	`(?im)^(#{1,6})\s+(?:overview|introduction|intro)\s*$`)

// reAnyHeading is used to find the boundary that ends the intro section
// (the next heading after the section opener).
var reAnyHeading = regexp.MustCompile(`(?m)^#{1,6}\s+\S`)

// detectDescription tries to extract a usable Mintlify `description`
// frontmatter value from the converted body. The strategy is
// intentionally conservative — we'd rather omit the field than emit a
// poor description.
//
// Algorithm:
//  1. Find a heading whose text is "Overview", "Introduction", or
//     "Intro" (case-insensitive).
//  2. Read content until the next heading at the same or shallower
//     level.
//  3. From that content, take the first non-empty line that is plain
//     prose (not a JSX tag, not a fenced code block, not a list item,
//     not a comment, not a `Frame`).
//  4. Truncate to the first sentence (terminator: `.`, `!`, `?`
//     followed by space-or-end). Trim to ≤200 chars at a word
//     boundary as a final safety net.
//
// Returns "" when no introductory section is present, or when the
// section's first prose line doesn't yield a clean sentence.
func detectDescription(body string) string {
	loc := reIntroSection.FindStringSubmatchIndex(body)
	if loc == nil {
		return ""
	}
	level := loc[3] - loc[2] // length of the captured heading prefix
	sectionStart := loc[1]   // byte position right after the heading line

	// Bound the section by the next heading at depth ≤ level.
	tail := body[sectionStart:]
	end := len(tail)
	for _, m := range reAnyHeading.FindAllStringIndex(tail, -1) {
		nextLevel := 0
		for nextLevel < len(tail[m[0]:]) && tail[m[0]+nextLevel] == '#' {
			nextLevel++
		}
		if nextLevel <= level {
			end = m[0]
			break
		}
	}
	section := tail[:end]

	inFence := false
	inJSXBlock := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		// JSX block boundaries: an opening `<Tag>` on its own line
		// puts us inside a component until we hit `</Tag>` or `/>`.
		if strings.HasPrefix(trimmed, "<") {
			if strings.HasPrefix(trimmed, "</") {
				inJSXBlock = false
			} else if strings.HasSuffix(trimmed, "/>") {
				// self-closing — no nested content to skip
			} else {
				inJSXBlock = true
			}
			continue
		}
		if inJSXBlock {
			if strings.HasPrefix(trimmed, "</") {
				inJSXBlock = false
			}
			continue
		}
		if !isProseLine(trimmed) {
			continue
		}
		sentence := firstSentence(trimmed)
		sentence = stripInlineMarkdown(sentence)
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}
		return clampDescription(sentence)
	}
	return ""
}

// isProseLine reports whether a trimmed line is plain narrative
// prose suitable for the description field. It rejects empty lines,
// fence markers, JSX components, list items, headings, and the
// COPIED_* provenance comments.
func isProseLine(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~") {
		return false
	}
	if strings.HasPrefix(s, "<") {
		return false
	}
	if strings.HasPrefix(s, "{/*") {
		return false
	}
	if strings.HasPrefix(s, "#") {
		return false
	}
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") ||
		strings.HasPrefix(s, "+ ") || strings.HasPrefix(s, "> ") {
		return false
	}
	if strings.HasPrefix(s, "|") {
		return false
	}
	// Numbered list `1. `, `2. ` etc.
	if len(s) >= 2 && s[0] >= '0' && s[0] <= '9' {
		i := 1
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i < len(s) && (s[i] == '.' || s[i] == ')') {
			return false
		}
	}
	return true
}

// firstSentence returns everything in s up to and including the first
// terminal punctuation followed by space or end-of-string. If no
// terminator is found, returns the whole string.
func firstSentence(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}
		// Don't end on `e.g.`, `i.e.`, abbreviation-like tokens.
		// Heuristic: require the next char to be a space or end-of-
		// string AND the previous char to NOT be a single letter
		// preceded by `.` (catches `e.g.`).
		next := byte(' ')
		if i+1 < len(s) {
			next = s[i+1]
		}
		if next != ' ' && next != '\t' && i+1 != len(s) {
			continue
		}
		// "e.g." / "i.e." / "U.S." style — prev-prev is a letter that
		// is itself preceded by `.`.
		if i >= 2 && s[i-2] == '.' {
			continue
		}
		return s[:i+1]
	}
	return s
}

// stripInlineMarkdown removes inline formatting markers from text so
// the resulting description is plain prose suitable for SEO meta tags.
// We strip: **bold**, *italic*, `code`, [link](url) → link text only,
// and any backslash escapes added by the MDX-escape pass (e.g.
// `\<scheme\>` → `<scheme>` is wrong for descriptions; we keep the
// backslashes off and let the literal `<scheme>` appear).
func stripInlineMarkdown(s string) string {
	// Markdown links: [text](url) → text
	s = reMDLink.ReplaceAllString(s, "$1")
	// Bold: **text** → text
	s = reMDBold.ReplaceAllString(s, "$1")
	// Italic: *text* → text  (after bold so we don't eat the bold ones)
	s = reMDItalic.ReplaceAllString(s, "$1")
	// Inline code: `text` → text
	s = reMDCode.ReplaceAllString(s, "$1")
	// MDX-escape sequences (\<word\> / \[\<word\>\]) — emit clean text.
	s = strings.ReplaceAll(s, `\<`, "<")
	s = strings.ReplaceAll(s, `\>`, ">")
	s = strings.ReplaceAll(s, `\[`, "[")
	s = strings.ReplaceAll(s, `\]`, "]")
	// Collapse whitespace runs the strip may have introduced.
	s = strings.Join(strings.Fields(s), " ")
	return s
}

var (
	reMDLink   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reMDBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reMDItalic = regexp.MustCompile(`\*([^*]+)\*`)
	reMDCode   = regexp.MustCompile("`([^`]+)`")
)

// clampDescription truncates over-long descriptions at a word boundary.
// 200 chars is a comfortable upper bound for SEO meta descriptions
// (Google typically renders ~155-160). We don't pad short ones.
func clampDescription(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	cut := strings.LastIndexByte(s[:max], ' ')
	if cut <= 0 {
		cut = max
	}
	return strings.TrimRight(s[:cut], ", ;:") + "…"
}
