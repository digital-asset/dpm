// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ImageValidator checks that local image references resolve to a real file on
// disk. It looks at Markdown image syntax (`![alt](path)`) and JSX/HTML
// `src="…"` attributes that point at an image. For each local image reference
// it resolves the target and reports `image-not-found` (Error) when the file is
// absent.
//
// Resolution rules:
//   - A path beginning with `/` is rooted at the docs root: the nearest
//     ancestor directory of the file that contains `docs.json` (the Mintlify
//     site root). If no docs root is found, absolute references are skipped
//     rather than guessed at.
//   - Any other path is resolved relative to the directory of the .mdx file
//     (so `../images/x.png` walks up from the page as written).
//
// Existence is checked by reading the target's directory and matching the exact
// file name. That makes the check case-sensitive on every OS, so a reference to
// `Foo.png` whose file is actually `foo.png` is reported even on a
// case-insensitive macOS filesystem (where it would silently break the Linux
// build).
//
// Deliberately out of scope (keeps the check false-positive-free and fast):
//   - External references (http/https, protocol-relative `//`, `data:`,
//     `mailto:`) — those are an external-link concern, not file existence.
//   - Expression sources (`src={…}`) — not a static path.
//   - Non-image extensions — only known image extensions are checked, so a
//     `<iframe src="…">` or other non-image source is never flagged.
//   - Markdown reference-style images (`![alt][ref]`).
type ImageValidator struct{}

// Name implements Validator.
func (ImageValidator) Name() string { return "images" }

// reMarkdownImage captures the URL of a Markdown image: ![alt](url "title").
// The URL is the first run of non-space, non-')' characters, optionally wrapped
// in angle brackets, which also skips any trailing "title".
var reMarkdownImage = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)\s>]+)>?`)

// reSrcAttr captures the value of a double- or single-quoted src attribute.
// Expression sources (src={…}) do not match and are therefore skipped.
var reSrcAttr = regexp.MustCompile(`\bsrc\s*=\s*"([^"]*)"|\bsrc\s*=\s*'([^']*)'`)

// imageExts is the set of extensions treated as images.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".webp": true, ".avif": true, ".bmp": true, ".ico": true,
}

// Validate implements Validator.
func (v ImageValidator) Validate(path string, content []byte, _ *parsed) []Finding {
	fileDir := filepath.Dir(path)
	root := docsRoot(fileDir)
	dirCache := map[string]map[string]bool{}

	var findings []Finding
	for _, ref := range imageRefs(content) {
		target, ok := resolveImage(ref.url, fileDir, root)
		if !ok {
			continue // external, expression, non-image, or unresolvable-absolute
		}
		if !fileInDir(dirCache, target) {
			findings = append(findings, Finding{
				Path:     path,
				Line:     ref.line,
				Severity: Error,
				Code:     "image-not-found",
				Message:  fmt.Sprintf("image not found: %q (resolved to %s)", ref.url, target),
			})
		}
	}
	return findings
}

// imageRef is a single image reference with the 1-based line it appears on.
type imageRef struct {
	url  string
	line int
}

// imageRefs extracts Markdown image URLs and src attribute values from content,
// each tagged with its line number.
func imageRefs(content []byte) []imageRef {
	var out []imageRef
	add := func(loc []int, group int) {
		// loc is FindAllSubmatchIndex output; group*2 / group*2+1 bound the
		// capture. A negative start means that alternation branch didn't match.
		s, e := loc[group*2], loc[group*2+1]
		if s < 0 {
			return
		}
		out = append(out, imageRef{url: string(content[s:e]), line: lineAt(content, s)})
	}
	for _, loc := range reMarkdownImage.FindAllSubmatchIndex(content, -1) {
		add(loc, 1)
	}
	for _, loc := range reSrcAttr.FindAllSubmatchIndex(content, -1) {
		add(loc, 1) // double-quoted branch
		add(loc, 2) // single-quoted branch
	}
	return out
}

// resolveImage decides whether a reference is a local image worth checking and,
// if so, returns its absolute filesystem path. The bool is false for anything
// skipped (external, expression, non-image, or absolute with no docs root).
func resolveImage(ref, fileDir, root string) (string, bool) {
	if ref == "" || strings.HasPrefix(ref, "{") {
		return "", false
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "//") || strings.HasPrefix(ref, "data:") ||
		strings.HasPrefix(ref, "mailto:") {
		return "", false
	}
	// Strip query and fragment before extension and existence checks.
	clean := ref
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if !imageExts[strings.ToLower(filepath.Ext(clean))] {
		return "", false
	}
	if strings.HasPrefix(clean, "/") {
		if root == "" {
			return "", false // can't resolve an absolute path without a docs root
		}
		return filepath.Join(root, filepath.FromSlash(clean)), true
	}
	return filepath.Join(fileDir, filepath.FromSlash(clean)), true
}

// docsRoot returns the nearest ancestor of dir (inclusive) that contains a
// docs.json file, or "" if none is found.
func docsRoot(dir string) string {
	d := dir
	for {
		if info, err := os.Stat(filepath.Join(d, "docs.json")); err == nil && !info.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// fileInDir reports whether target exists, matching the file name case-exactly
// by reading the containing directory. Directory listings are cached per call.
func fileInDir(cache map[string]map[string]bool, target string) bool {
	dir := filepath.Dir(target)
	set, ok := cache[dir]
	if !ok {
		set = map[string]bool{}
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				set[e.Name()] = true
			}
		}
		cache[dir] = set
	}
	return set[filepath.Base(target)]
}

// lineAt returns the 1-based line number of byte offset off in content.
func lineAt(content []byte, off int) int {
	line := 1
	for i := 0; i < off && i < len(content); i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}
