// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"path"
	"regexp"
	"strings"
)

// RST has two image directives:
//
//	.. image:: path
//	   :alt: Alt text
//	   :width: 400px
//
//	.. figure:: path
//	   :alt: Alt text
//
//	   Caption text.
//
// Images become `![alt](/images/docs_website/file.png)`; figures become
// a `<Frame caption="...">` wrapping the image. The image asset copy
// itself (moving files to `docs-main/images/docs_website/`) is Phase 5.
// This file only rewrites the reference paths.
//
// The image path in RST is typically relative to the source file or to a
// docs-website `_static` dir. We collapse all paths to the target
// Mintlify convention: `/images/docs_website/<basename>`.

var (
	reImage = regexp.MustCompile(
		`(?m)^(\s*)\.\.\s+image::\s+([^\n]+)\n((?:[ \t]+:[^\n]+\n)*)`)
	reFigure = regexp.MustCompile(
		`(?m)^(\s*)\.\.\s+figure::\s+([^\n]+)\n((?:[ \t]+:[^\n]+\n)*)(?:\n((?:[ \t]+[^\n]+\n)+))?`)
	reOption = regexp.MustCompile(`(?m)^\s+:([A-Za-z][A-Za-z0-9_\-]*):\s*([^\n]*)$`)
)

func convertImages(s string) string {
	// Figures first so their options don't get eaten by the plain-image
	// pattern.
	s = reFigure.ReplaceAllStringFunc(s, func(match string) string {
		m := reFigure.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		indent, src, opts, caption := m[1], strings.TrimSpace(m[2]), m[3], m[4]
		alt := extractOption(opts, "alt")
		if alt == "" {
			alt = "image"
		}
		cap := strings.TrimSpace(caption)
		target := toMintlifyImagePath(src)
		if cap == "" {
			return indent + "![" + alt + "](" + target + ")\n"
		}
		return indent + `<Frame caption="` + escapeAttr(cap) + `">` + "\n" +
			indent + "  ![" + alt + "](" + target + ")\n" +
			indent + "</Frame>\n"
	})

	s = reImage.ReplaceAllStringFunc(s, func(match string) string {
		m := reImage.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		indent, src, opts := m[1], strings.TrimSpace(m[2]), m[3]
		alt := extractOption(opts, "alt")
		if alt == "" {
			alt = "image"
		}
		target := toMintlifyImagePath(src)
		return indent + "![" + alt + "](" + target + ")\n"
	})
	return s
}

// toMintlifyImagePath takes the RST image reference and returns the
// target MDX reference. Paths are normalized by:
//   - stripping any leading directory components
//   - placing the basename under `/images/docs_website/`
//
// Phase 5 will also copy the asset itself.
func toMintlifyImagePath(src string) string {
	base := path.Base(src)
	return "/images/docs_website/" + base
}

// extractOption finds `:name: value` inside the raw options block. If
// `name` is missing it returns the empty string.
func extractOption(block, name string) string {
	for _, line := range strings.Split(block, "\n") {
		m := reOption.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == name {
			return strings.TrimSpace(m[2])
		}
	}
	return ""
}

// escapeAttr escapes a string for use inside a JSX attribute value.
func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
