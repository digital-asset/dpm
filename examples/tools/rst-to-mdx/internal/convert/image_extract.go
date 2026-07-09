// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ImageRef captures one image reference found in an RST source so the
// caller can copy the asset into the Mintlify tree. The text-level
// rewrite (in images.go) and the asset copy are decoupled because
// copying touches the filesystem and we want the conversion library to
// stay pure.
type ImageRef struct {
	// SourceRel is the path as written in the RST directive, before any
	// rewriting. Always relative to the directory of the RST file that
	// referenced it.
	SourceRel string
	// SourceAbs is the resolved absolute path of the asset on disk.
	// Empty when SourcePath wasn't supplied.
	SourceAbs string
	// TargetRel is the relative path under the target docs root where
	// the asset should live. Always `images/docs_website/<basename>`.
	TargetRel string
	// Alt is the alt-text from the directive's `:alt:` option, or
	// empty if none was set.
	Alt string
}

var (
	reImageDirective = regexp.MustCompile(
		`(?m)^\s*\.\.\s+image::\s+([^\n]+)\n((?:\s+:[^\n]+\n)*)`)
	reFigureDirective = regexp.MustCompile(
		`(?m)^\s*\.\.\s+figure::\s+([^\n]+)\n((?:\s+:[^\n]+\n)*)`)
)

// extractImageRefs walks raw RST text and returns every image and
// figure directive's source path, resolved to an absolute filesystem
// path when sourcePath is non-empty. The function runs against the RST
// before any pipeline transforms so the directive shape is intact.
func extractImageRefs(rst, sourcePath string) []ImageRef {
	var out []ImageRef
	collect := func(matches [][]string) {
		for _, m := range matches {
			rel := strings.TrimSpace(m[1])
			alt := extractOption(m[2], "alt")
			abs := ""
			if sourcePath != "" {
				abs = filepath.Join(filepath.Dir(sourcePath), rel)
			}
			out = append(out, ImageRef{
				SourceRel: rel,
				SourceAbs: abs,
				TargetRel: filepath.Join("images", "docs_website", filepath.Base(rel)),
				Alt:       alt,
			})
		}
	}
	collect(reImageDirective.FindAllStringSubmatch(rst, -1))
	collect(reFigureDirective.FindAllStringSubmatch(rst, -1))
	return out
}
