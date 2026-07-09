// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// FrontmatterValidator checks the YAML frontmatter block at the top of an
// MDX file. Enforces:
//
//   - The file must open with `---` and have a closing `---`.
//   - The block must contain a non-empty `title:` value.
type FrontmatterValidator struct{}

// Name implements Validator.
func (FrontmatterValidator) Name() string { return "frontmatter" }

// reTitleLine matches a `title:` line at the start of a line. Captured
// group is the right-hand side as written, before quote stripping.
//
// `[ \t]*` (not `\s*`) keeps the match anchored to a single line — `\s`
// would eat across `\n` and let an empty `title:` line silently absorb
// the next line's value.
var reTitleLine = regexp.MustCompile(`(?m)^title:[ \t]*(.*?)[ \t]*$`)

// Validate implements Validator.
//
// Known regex-parsing limitations:
//   - Block scalars whose body contains a `title:` line (e.g. `description: |`
//     followed by an indented `title:`) can match the inner string.
//   - Duplicate `title:` keys (which YAML rejects as invalid) are accepted;
//     the first match wins.
//
// If/when these limitations bite real pages, switch to a real YAML parser
// (likely gopkg.in/yaml.v3) — the interface and tests stay the same.
func (v FrontmatterValidator) Validate(path string, content []byte, _ *parsed) []Finding {
	block, err := extractFrontmatterBlock(content)
	if err != nil {
		return []Finding{{
			Path:     path,
			Line:     1,
			Severity: Error,
			Code:     "frontmatter-unreadable",
			Message:  fmt.Sprintf("failed to scan frontmatter: %v", err),
		}}
	}
	if block == nil {
		return []Finding{{
			Path:     path,
			Line:     1,
			Severity: Error,
			Code:     "frontmatter-missing",
			Message:  "MDX file has no YAML frontmatter (expected leading `---` block)",
		}}
	}

	m := reTitleLine.FindSubmatch(block.body)
	if m == nil {
		return []Finding{{
			Path:     path,
			Line:     block.startLine,
			Severity: Error,
			Code:     "frontmatter-missing-title",
			Message:  "frontmatter must declare a `title:` field",
		}}
	}

	if isEmptyTitle(string(m[1])) {
		return []Finding{{
			Path:     path,
			Line:     block.startLine,
			Severity: Error,
			Code:     "frontmatter-empty-title",
			Message:  "frontmatter `title:` is empty",
		}}
	}

	return nil
}

// frontmatterBlock is the slice of bytes between (and excluding) the two
// `---` delimiter lines, plus the 1-based line number of the opening
// delimiter for diagnostic attribution.
type frontmatterBlock struct {
	body      []byte
	startLine int
}

// extractFrontmatterBlock returns the YAML body between the leading and
// trailing `---` markers.
//
// Returns:
//   - (*block, nil) when a complete frontmatter block was found.
//   - (nil, nil)    when no frontmatter is present (legitimate "missing" case).
//   - (nil, err)    when the scanner fails (e.g. a single line longer than
//     the 1 MiB buffer). Callers should report this as a
//     distinct, attributable finding rather than confusing it
//     with a missing block.
func extractFrontmatterBlock(content []byte) (*frontmatterBlock, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if scanner.Text() != "---" {
		return nil, nil
	}

	var body bytes.Buffer
	startLine := 1
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			return &frontmatterBlock{body: body.Bytes(), startLine: startLine}, nil
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// EOF without seeing the closing `---` — treat as missing block, not error.
	return nil, nil
}

// isEmptyTitle returns true if the captured title value resolves to an
// empty string after quote and whitespace stripping. Accepts unquoted,
// double-quoted, and single-quoted forms.
func isEmptyTitle(raw string) bool {
	v := strings.TrimSpace(raw)
	v = strings.TrimSuffix(strings.TrimPrefix(v, `"`), `"`)
	v = strings.TrimSuffix(strings.TrimPrefix(v, `'`), `'`)
	return strings.TrimSpace(v) == ""
}
