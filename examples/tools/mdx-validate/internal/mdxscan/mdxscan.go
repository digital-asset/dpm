// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package mdxscan is a code-fence-aware JSX element scanner. It finds
// capitalized-name JSX elements that appear *outside* of fenced code blocks and
// inline code spans, so the "<HOST:PORT>-in-a-fence" class of false positive
// never reaches a validator.
//
// Masking (what is treated as code and skipped) is exactly two things: fenced
// code blocks (``` or ~~~) and inline code spans (runs of backticks). Nothing
// else — indented code, HTML comments, and {…} expression blocks are not masked.
// Masking preserves byte and line positions (masked characters become spaces,
// newlines are kept) so reported line numbers stay accurate.
package mdxscan

// Kind classifies a JSX tag.
type Kind int

const (
	// Open is an opening tag, e.g. <Accordion>.
	Open Kind = iota
	// Close is a closing tag, e.g. </Accordion>.
	Close
	// SelfClose is a self-closing tag, e.g. <Icon />.
	SelfClose
)

// Attr is a single parsed attribute of an Open/SelfClose element.
type Attr struct {
	Name   string
	Value  string // unquoted literal value; "" for boolean/expression attrs
	IsExpr bool   // true when the value is a {…} expression (skip literal checks)
}

// Element is a JSX element discovered outside of code.
type Element struct {
	Name  string // tag name, e.g. "Accordion"
	Kind  Kind
	Attrs []Attr // empty for Close
	Line  int    // 1-based line of the opening '<'
}

// Elements returns every capitalized-name JSX element found outside of fenced
// code blocks and inline code spans, in source order.
func Elements(content []byte) []Element {
	masked := mask(content)
	return scan(masked)
}

// isCapital reports whether b is an ASCII uppercase letter.
func isCapital(b byte) bool { return b >= 'A' && b <= 'Z' }

// isNameByte reports whether b can appear in a JSX tag name after the first
// character (i.e. matches [A-Za-z0-9]).
func isNameByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// mask returns a copy of content with all fenced-code and inline-code bytes
// replaced by spaces, preserving newlines (and thus all line and byte offsets).
func mask(content []byte) []byte {
	out := make([]byte, len(content))
	copy(out, content)

	// Phase 1: fenced code blocks, line by line.
	lines := splitLines(content)
	var (
		inFence   bool
		fenceChar byte
		fenceLen  int
	)
	for _, ln := range lines {
		fc, flen := fenceMarker(content, ln.start, ln.end)
		if !inFence {
			if fc != 0 {
				// Opening fence: mask the fence line, enter fence.
				maskRange(out, content, ln.start, ln.end)
				inFence = true
				fenceChar = fc
				fenceLen = flen
			}
			continue
		}
		// Inside a fence: mask everything until a matching closing fence.
		maskRange(out, content, ln.start, ln.end)
		if fc == fenceChar && flen >= fenceLen {
			inFence = false
			fenceChar = 0
			fenceLen = 0
		}
	}

	// Phase 2: inline code spans, over the fence-masked output, per line. A span
	// opens at a run of N backticks and closes at the next run of exactly N
	// backticks on the same line. Bytes inside fences are already spaces, so they
	// won't be mistaken for span delimiters.
	for _, ln := range lines {
		maskInlineSpans(out, ln.start, ln.end)
	}

	return out
}

// lineSpan is a half-open byte range [start, end) for a single line, excluding
// the trailing newline.
type lineSpan struct {
	start int
	end   int
}

// splitLines splits content into line spans (newline excluded from each span).
func splitLines(content []byte) []lineSpan {
	var lines []lineSpan
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, lineSpan{start: start, end: i})
			start = i + 1
		}
	}
	lines = append(lines, lineSpan{start: start, end: len(content)})
	return lines
}

// fenceMarker examines the line [start, end) and, if it is a fence line,
// returns the fence character ('`' or '~') and the length of the marker run.
// Otherwise it returns (0, 0). A fence line has ≤3 leading spaces followed by a
// run of ≥3 of the same fence character; the rest of the line is the info
// string and is ignored.
func fenceMarker(content []byte, start, end int) (byte, int) {
	i := start
	spaces := 0
	for i < end && content[i] == ' ' && spaces < 4 {
		spaces++
		i++
	}
	if spaces > 3 || i >= end {
		return 0, 0
	}
	ch := content[i]
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	run := 0
	for i < end && content[i] == ch {
		run++
		i++
	}
	if run < 3 {
		return 0, 0
	}
	return ch, run
}

// maskRange replaces out[start:end) with spaces, but leaves bytes that are
// newlines untouched (there are none within a line span, but this keeps the
// helper safe).
func maskRange(out, content []byte, start, end int) {
	for i := start; i < end; i++ {
		if content[i] != '\n' {
			out[i] = ' '
		}
	}
}

// maskInlineSpans masks inline code spans within the line span [start, end) of
// out. It scans for runs of backticks; a run of N opens a span that closes at
// the next run of exactly N backticks on the same line.
func maskInlineSpans(out []byte, start, end int) {
	i := start
	for i < end {
		if out[i] != '`' {
			i++
			continue
		}
		// Measure the opening run length.
		openStart := i
		n := 0
		for i < end && out[i] == '`' {
			n++
			i++
		}
		// Search for a closing run of exactly n backticks.
		j := i
		for j < end {
			if out[j] != '`' {
				j++
				continue
			}
			m := 0
			for j < end && out[j] == '`' {
				m++
				j++
			}
			if m == n {
				// Mask from the opening run through the closing run inclusive.
				for k := openStart; k < j; k++ {
					out[k] = ' '
				}
				i = j
				break
			}
			// Not a match; the run we just consumed is content, continue
			// scanning from j.
		}
		if j >= end {
			// No closing run found; the backticks are literal content, leave as is.
			i = end
		}
	}
}

// scan extracts JSX elements from the (already masked) content.
func scan(content []byte) []Element {
	var elements []Element
	line := 1
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '\n' {
			line++
			continue
		}
		if c != '<' {
			continue
		}
		// Possible tag start. Look at the following bytes.
		j := i + 1
		isClose := false
		if j < len(content) && content[j] == '/' {
			isClose = true
			j++
		}
		if j >= len(content) || !isCapital(content[j]) {
			// Not a capitalized tag (lowercase, '<>', '</>' etc.). Skip.
			continue
		}
		// Read the name.
		nameStart := j
		for j < len(content) && isNameByte(content[j]) {
			j++
		}
		name := string(content[nameStart:j])
		startLine := line

		// Scan to the terminating '>', respecting "…", '…', and {…}.
		body, endIdx, bodyNewlines := scanTagBody(content, j)
		if endIdx < 0 {
			// Unterminated tag; stop scanning meaningfully but keep line count
			// accurate by advancing past what we consumed.
			line += bodyNewlines
			i = len(content)
			continue
		}

		el := Element{Name: name, Line: startLine}
		if isClose {
			el.Kind = Close
		} else {
			selfClose := false
			trimmed := body
			// A '/' immediately before the terminating '>' marks self-close.
			for k := len(trimmed) - 1; k >= 0; k-- {
				if trimmed[k] == ' ' || trimmed[k] == '\t' || trimmed[k] == '\n' || trimmed[k] == '\r' {
					continue
				}
				if trimmed[k] == '/' {
					selfClose = true
					trimmed = trimmed[:k]
				}
				break
			}
			if selfClose {
				el.Kind = SelfClose
			} else {
				el.Kind = Open
			}
			el.Attrs = parseAttrs(trimmed)
		}

		elements = append(elements, el)
		line += bodyNewlines
		i = endIdx // loop's i++ moves past '>'
	}
	return elements
}

// scanTagBody scans from index start (just past the tag name) to the
// terminating '>', respecting quoted strings and {…} expressions. It returns
// the body bytes (between the name and the '>'), the index of the terminating
// '>', and the number of newlines consumed. If unterminated, endIdx is -1.
func scanTagBody(content []byte, start int) (body []byte, endIdx int, newlines int) {
	i := start
	var (
		inDouble   bool
		inSingle   bool
		braceDepth int
	)
	for i < len(content) {
		c := content[i]
		switch {
		case c == '\n':
			newlines++
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case braceDepth > 0:
			switch c {
			case '{':
				braceDepth++
			case '}':
				braceDepth--
			}
		case c == '"':
			inDouble = true
		case c == '\'':
			inSingle = true
		case c == '{':
			braceDepth++
		case c == '>':
			return content[start:i], i, newlines
		}
		i++
	}
	return nil, -1, newlines
}

// parseAttrs parses the attribute portion of a tag body into Attrs.
func parseAttrs(body []byte) []Attr {
	var attrs []Attr
	i := 0
	n := len(body)
	for i < n {
		// Skip whitespace.
		for i < n && isSpace(body[i]) {
			i++
		}
		if i >= n {
			break
		}
		// Attribute name: [A-Za-z][A-Za-z0-9_-]* (be permissive on later bytes).
		if !isAttrNameStart(body[i]) {
			i++
			continue
		}
		nameStart := i
		i++
		for i < n && isAttrNameByte(body[i]) {
			i++
		}
		name := string(body[nameStart:i])

		// Skip whitespace before a possible '='.
		k := i
		for k < n && isSpace(body[k]) {
			k++
		}
		if k >= n || body[k] != '=' {
			// Boolean attribute.
			attrs = append(attrs, Attr{Name: name})
			i = k
			continue
		}
		// Consume '=' and following whitespace.
		k++
		for k < n && isSpace(body[k]) {
			k++
		}
		if k >= n {
			attrs = append(attrs, Attr{Name: name})
			i = k
			continue
		}
		switch body[k] {
		case '"':
			k++
			valStart := k
			for k < n && body[k] != '"' {
				k++
			}
			attrs = append(attrs, Attr{Name: name, Value: string(body[valStart:k])})
			if k < n {
				k++ // past closing quote
			}
		case '\'':
			k++
			valStart := k
			for k < n && body[k] != '\'' {
				k++
			}
			attrs = append(attrs, Attr{Name: name, Value: string(body[valStart:k])})
			if k < n {
				k++ // past closing quote
			}
		case '{':
			depth := 0
			for k < n {
				if body[k] == '{' {
					depth++
				} else if body[k] == '}' {
					depth--
					if depth == 0 {
						k++
						break
					}
				}
				k++
			}
			attrs = append(attrs, Attr{Name: name, IsExpr: true})
		default:
			// Unquoted value, read until whitespace.
			valStart := k
			for k < n && !isSpace(body[k]) {
				k++
			}
			attrs = append(attrs, Attr{Name: name, Value: string(body[valStart:k])})
		}
		i = k
	}
	return attrs
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isAttrNameStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isAttrNameByte(b byte) bool {
	return isAttrNameStart(b) || (b >= '0' && b <= '9') || b == '-' || b == ':'
}
