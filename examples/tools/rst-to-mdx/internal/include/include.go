// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package include resolves two RST directives that pull content from
// other files:
//
//   - `.. literalinclude:: <path>` — inline the file as a code block,
//     with optional language tagging and line/marker slicing.
//   - `.. include:: <path>` — splice the referenced RST directly; the
//     included content then flows through the rest of the conversion
//     pipeline as if it had been written in-place.
//
// Resolution happens BEFORE the main transform pipeline so that
// `.. include::` fragments get the same heading/codeblock/admonition
// treatment as the root file. `literalinclude` is rewritten into a
// `.. code-block:: lang\n\n<content>` directive which the existing
// codeblocks transform then handles.
//
// File paths are resolved relative to the including file. Paths that
// begin with `/` are resolved relative to the docs root when one is
// provided; otherwise they fall back to the filesystem root (which
// typically misses, producing a TODO marker in lenient mode or an
// error in strict mode).
package include

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Resolver pulls referenced files off disk. In production this is just
// os.ReadFile; tests swap in an in-memory implementation.
type Resolver interface {
	Read(path string) ([]byte, error)
}

// osResolver reads the real filesystem.
type osResolver struct{}

func (osResolver) Read(p string) ([]byte, error) { return os.ReadFile(p) }

// OSResolver returns the default filesystem-backed resolver.
func OSResolver() Resolver { return osResolver{} }

// Options controls include resolution.
type Options struct {
	// SourcePath is the path of the RST file whose directives we're
	// resolving. Relative include paths are joined to its directory.
	SourcePath string
	// DocsRoot is the root of the docs tree. Absolute include paths
	// (starting with `/`) are joined to it.
	DocsRoot string
	// Strict causes missing files, out-of-range lines, or unfound
	// markers to return an error. When false, they leave a TODO
	// marker in the output and continue.
	Strict bool
	// MaxDepth caps recursive include expansion; defaults to 32.
	MaxDepth int
	// Resolver defaults to OSResolver if nil.
	Resolver Resolver
}

// Resolve walks the input RST and expands `.. include::` and
// `.. literalinclude::` directives. The include expansion recurses (up
// to MaxDepth) so a chain of include/include/include works. Literal
// includes are emitted as RST `.. code-block:: lang` so the downstream
// code-blocks transform handles the fencing uniformly.
func Resolve(rst string, opts Options) (string, error) {
	if opts.Resolver == nil {
		opts.Resolver = osResolver{}
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 32
	}
	return resolveAtDepth(rst, opts, 0)
}

func resolveAtDepth(rst string, opts Options, depth int) (string, error) {
	if depth > opts.MaxDepth {
		return "", fmt.Errorf("include depth exceeded (%d)", opts.MaxDepth)
	}

	lines := strings.Split(rst, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]

		// .. literalinclude:: <path>
		if m := reLiteralInclude.FindStringSubmatch(line); m != nil {
			indent, rawPath := m[1], strings.TrimSpace(m[2])
			i++
			options, consumed := readOptions(lines[i:])
			i += consumed

			block, err := handleLiteralInclude(indent, rawPath, options, opts)
			if err != nil {
				return "", err
			}
			out = append(out, block...)
			continue
		}

		// .. include:: <path>
		if m := reInclude.FindStringSubmatch(line); m != nil {
			rawPath := strings.TrimSpace(m[1])
			i++
			// `.. include::` doesn't use indented options, but skip
			// any just in case.
			_, consumed := readOptions(lines[i:])
			i += consumed

			spliced, err := handleInclude(rawPath, opts, depth+1)
			if err != nil {
				return "", err
			}
			out = append(out, spliced...)
			continue
		}

		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n"), nil
}

var (
	reLiteralInclude = regexp.MustCompile(
		`^(\s*)\.\.\s+literalinclude::\s+(.+)$`)
	reInclude = regexp.MustCompile(
		`^\s*\.\.\s+include::\s+(.+)$`)
	reOptionLine = regexp.MustCompile(
		`^\s+:([A-Za-z][A-Za-z0-9_\-]*):\s*(.*)$`)
)

// readOptions reads the indented `:name: value` lines that follow a
// directive. Returns the key/value map and the count of lines consumed.
func readOptions(lines []string) (map[string]string, int) {
	opts := map[string]string{}
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// A blank line terminates the options block.
			i++
			break
		}
		m := reOptionLine.FindStringSubmatch(line)
		if m == nil {
			break
		}
		opts[strings.ToLower(m[1])] = strings.TrimSpace(m[2])
		i++
	}
	return opts, i
}

// handleLiteralInclude resolves the target file, applies any line or
// marker filters, and emits an RST `.. code-block:: lang` directive so
// the downstream codeblocks transform can fence it.
func handleLiteralInclude(indent, rawPath string, options map[string]string, opts Options) ([]string, error) {
	abs, err := resolveIncludePath(rawPath, opts)
	if err != nil {
		return marker(indent, "literalinclude-resolve", rawPath, err, opts.Strict)
	}

	data, err := opts.Resolver.Read(abs)
	if err != nil {
		return marker(indent, "literalinclude-missing", rawPath, err, opts.Strict)
	}

	body := string(data)
	body, err = applySliceOptions(body, options)
	if err != nil {
		return marker(indent, "literalinclude-slice", rawPath, err, opts.Strict)
	}

	lang := options["language"]
	if lang == "" {
		lang = inferLanguage(abs)
	}

	// Emit as a code-block directive for the downstream transform.
	var out []string
	out = append(out, indent+".. code-block:: "+lang)
	out = append(out, "")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		out = append(out, indent+"   "+line)
	}
	out = append(out, "")
	return out, nil
}

// handleInclude reads the target RST file and splices its content,
// recursively resolving nested includes.
func handleInclude(rawPath string, opts Options, depth int) ([]string, error) {
	abs, err := resolveIncludePath(rawPath, opts)
	if err != nil {
		return marker("", "include-resolve", rawPath, err, opts.Strict)
	}

	data, err := opts.Resolver.Read(abs)
	if err != nil {
		return marker("", "include-missing", rawPath, err, opts.Strict)
	}

	// Recurse with the included file as the new source path so its
	// relative includes resolve correctly.
	childOpts := opts
	childOpts.SourcePath = abs
	expanded, err := resolveAtDepth(string(data), childOpts, depth)
	if err != nil {
		return nil, err
	}
	return strings.Split(expanded, "\n"), nil
}

// resolveIncludePath joins a raw directive path to the appropriate
// base. Paths starting with `/` are anchored to DocsRoot; everything
// else is relative to the including file.
func resolveIncludePath(raw string, opts Options) (string, error) {
	if strings.HasPrefix(raw, "/") {
		if opts.DocsRoot == "" {
			return "", fmt.Errorf("absolute include path %q requires --docs-root", raw)
		}
		return filepath.Join(opts.DocsRoot, strings.TrimPrefix(raw, "/")), nil
	}
	if opts.SourcePath == "" {
		return "", fmt.Errorf("relative include path %q requires a source path", raw)
	}
	return filepath.Join(filepath.Dir(opts.SourcePath), raw), nil
}

// marker emits an MDX comment that records the unresolved include so a
// human can grep for it, or returns an error in strict mode.
func marker(indent, kind, path string, cause error, strict bool) ([]string, error) {
	if strict {
		return nil, fmt.Errorf("%s %s: %w", kind, path, cause)
	}
	line := fmt.Sprintf("%s{/* %s: %s (%v) */}", indent, kind, path, cause)
	return []string{line}, nil
}

// applySliceOptions filters the included content per the directive's
// :lines:, :start-after:, :end-before:, :start-at:, :end-at:, :dedent:
// options. The semantics match Sphinx.
func applySliceOptions(body string, opts map[string]string) (string, error) {
	lines := strings.Split(body, "\n")

	// :lines: (has highest precedence when present)
	if spec := opts["lines"]; spec != "" {
		selected, err := selectLines(lines, spec)
		if err != nil {
			return "", err
		}
		lines = selected
	}

	// :start-after: / :end-before: (exclusive markers)
	if marker := opts["start-after"]; marker != "" {
		var err error
		lines, err = sliceAfter(lines, marker, false)
		if err != nil {
			return "", fmt.Errorf("start-after: %w", err)
		}
	}
	if marker := opts["end-before"]; marker != "" {
		var err error
		lines, err = sliceBefore(lines, marker, false)
		if err != nil {
			return "", fmt.Errorf("end-before: %w", err)
		}
	}
	// :start-at: / :end-at: (inclusive markers)
	if marker := opts["start-at"]; marker != "" {
		var err error
		lines, err = sliceAfter(lines, marker, true)
		if err != nil {
			return "", fmt.Errorf("start-at: %w", err)
		}
	}
	if marker := opts["end-at"]; marker != "" {
		var err error
		lines, err = sliceBefore(lines, marker, true)
		if err != nil {
			return "", fmt.Errorf("end-at: %w", err)
		}
	}

	// :dedent: (remove common leading whitespace)
	if _, ok := opts["dedent"]; ok {
		lines = dedent(lines)
	}

	return strings.Join(lines, "\n"), nil
}

// selectLines picks lines per a Sphinx-style spec:
//
//	"1-20"       → lines 1..20 inclusive
//	"5,10-15,20" → lines 5, 10..15, 20
//	"1"          → line 1 only
//	"4-"         → line 4 to end of file (open-ended range)
//
// Line numbers are 1-based.
func selectLines(lines []string, spec string) ([]string, error) {
	var out []string
	for _, token := range strings.Split(spec, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			parts := strings.SplitN(token, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil || start < 1 {
				return nil, fmt.Errorf("invalid :lines: range %q", token)
			}
			// An empty right-hand side means "from start to EOF" — a
			// common Sphinx idiom (e.g. ":lines: 4-").
			end := len(lines)
			if rhs := strings.TrimSpace(parts[1]); rhs != "" {
				n, err := strconv.Atoi(rhs)
				if err != nil || n < start {
					return nil, fmt.Errorf("invalid :lines: range %q", token)
				}
				end = n
			}
			for i := start; i <= end && i-1 < len(lines); i++ {
				out = append(out, lines[i-1])
			}
		} else {
			n, err := strconv.Atoi(token)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid :lines: value %q", token)
			}
			if n-1 < len(lines) {
				out = append(out, lines[n-1])
			}
		}
	}
	return out, nil
}

// sliceAfter returns lines AFTER the first occurrence of marker. When
// inclusive is true, the marker line itself is kept.
func sliceAfter(lines []string, marker string, inclusive bool) ([]string, error) {
	for i, line := range lines {
		if strings.Contains(line, marker) {
			if inclusive {
				return lines[i:], nil
			}
			return lines[i+1:], nil
		}
	}
	return nil, fmt.Errorf("marker %q not found", marker)
}

// sliceBefore returns lines BEFORE the first occurrence of marker.
// When inclusive is true, the marker line itself is kept.
func sliceBefore(lines []string, marker string, inclusive bool) ([]string, error) {
	for i, line := range lines {
		if strings.Contains(line, marker) {
			if inclusive {
				return lines[:i+1], nil
			}
			return lines[:i], nil
		}
	}
	return nil, fmt.Errorf("marker %q not found", marker)
}

// dedent strips the common leading-whitespace run from every non-blank
// line. Mirrors Sphinx's `:dedent:` without an explicit count.
func dedent(lines []string) []string {
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := leadingSpaces(line)
		if minIndent == -1 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= minIndent {
			out[i] = line[minIndent:]
		} else {
			out[i] = line
		}
	}
	return out
}

func leadingSpaces(s string) int {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return len(s)
}

// inferLanguage picks a fenced-code language tag from a file
// extension. Conservative: unknown extensions yield the empty string,
// which downstream code renders as an untagged fence.
func inferLanguage(abs string) string {
	ext := strings.ToLower(filepath.Ext(abs))
	switch ext {
	case ".daml":
		return "daml"
	case ".scala":
		return "scala"
	case ".java":
		return "java"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	case ".proto":
		return "protobuf"
	case ".hocon", ".conf":
		return "hocon"
	default:
		return ""
	}
}

// sanity-only so the bufio import isn't dropped if a refactor removes
// the only caller — bufio is still useful when we add a streaming path.
var _ = bufio.ScanLines
