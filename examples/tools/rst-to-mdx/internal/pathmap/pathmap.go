// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package pathmap maps RST source paths under docs-website/docs/replicated/
// to target MDX paths under docs/docs-main/ using a deterministic
// convention. The mapping mirrors where already-migrated files live.
//
// The convention (see plans/dpm-rst-to-mdx-component-plan.md §6.2):
//
//	RST under docs-website/docs/replicated/           → MDX under docs-main/
//	-------------------------------------------------   ---------------------
//	canton/<ver>/participant/tutorials/foo.rst        → appdev/tutorials/canton-foo.mdx
//	canton/<ver>/participant/howtos/foo.rst           → appdev/howtos/canton-foo.mdx
//	canton/<ver>/participant/explanations/foo.rst     → appdev/reference/canton-foo.mdx
//	canton/<ver>/participant/**/foo.rst               → appdev/reference/canton-foo.mdx
//	canton/<ver>/synchronizer/**/foo.rst              → global-synchronizer/canton-foo.mdx
//	canton/<ver>/sdk/**/foo.rst                       → appdev/sdk/canton-foo.mdx
//	daml/<ver>/sdk/tutorials/foo.rst                  → appdev/tutorials/daml-foo.mdx
//	daml/<ver>/sdk/howtos/foo.rst                     → appdev/howtos/daml-foo.mdx
//	daml/<ver>/sdk/**/foo.rst                         → appdev/reference/daml-foo.mdx
//	pqs/<ver>/**/foo.rst                              → integrations/pqs/foo.mdx
//	splice-wallet-kernel/**/foo.rst                   → integrations/splice/foo.mdx
//	quickstart/<ver>/sdk/quickstart/**/foo.rst        → appdev/quickstart/foo.mdx
//	canton-network-utilities/**/foo.rst               → integrations/canton-network-utilities/foo.mdx
//	daml-shell/**/foo.rst                             → integrations/daml-shell/foo.mdx
//	dpm/**/foo.rst                                    → appdev/dpm/foo.mdx
//
// Filenames are kebab-cased (underscores → hyphens, all lowercase).
package pathmap

import (
	"path/filepath"
	"strings"
)

// DerivedPath is the mapped target path, relative to docs-main/, without
// leading slash and without the .mdx extension.
//
// Note on URL space: Mintlify serves the docs-main/ directory AS the
// site root. Internal links must NOT include a `docs-main/` segment —
// that's a filesystem detail, not part of the URL. A link like
// `/docs-main/appdev/foo` 404s in production. The URL() helper omits
// the prefix accordingly.
type DerivedPath string

// URL returns the `/<path>` form used in MDX internal links —
// docs-site-root-relative, no `docs-main/` prefix.
func (d DerivedPath) URL() string { return "/" + string(d) }

// File returns the absolute on-disk path under `targetRoot` where the
// MDX file should be written (with the .mdx extension).
func (d DerivedPath) File(targetRoot string) string {
	return filepath.Join(targetRoot, string(d)+".mdx")
}

// Derive maps an RST source path to the corresponding DerivedPath. If
// the input isn't under a known subtree, Derive returns ok=false and the
// caller must decide what to do (typically: fall back to a TODO marker).
func Derive(rstPath string) (DerivedPath, bool) {
	rel := relativeToReplicated(rstPath)
	if rel == "" {
		return "", false
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return "", false
	}

	filename := filenameToKebab(parts[len(parts)-1])
	stem := strings.TrimSuffix(filename, ".mdx")

	switch parts[0] {
	case "canton":
		return mapCanton(parts, stem)
	case "daml":
		return mapDaml(parts, stem)
	case "pqs":
		return DerivedPath("integrations/pqs/" + stem), true
	case "splice-wallet-kernel":
		return DerivedPath("integrations/splice/" + stem), true
	case "quickstart":
		return mapQuickstart(parts, stem)
	case "canton-network-utilities":
		return DerivedPath("integrations/canton-network-utilities/" + stem), true
	case "daml-shell":
		return DerivedPath("integrations/daml-shell/" + stem), true
	case "dpm":
		return DerivedPath("appdev/dpm/" + stem), true
	}
	return "", false
}

// relativeToReplicated returns the portion of rstPath after the
// docs-website/docs/replicated/ prefix, or empty if the prefix isn't
// present.
func relativeToReplicated(p string) string {
	marker := "docs-website/docs/replicated/"
	i := strings.LastIndex(p, marker)
	if i < 0 {
		return ""
	}
	return filepath.ToSlash(p[i+len(marker):])
}

// filenameToKebab converts an RST filename to an MDX kebab-case filename.
//
//	getting_started.rst → getting-started.mdx
//	conf_file.rst       → conf-file.mdx
//	FAQ.rst             → faq.mdx
func filenameToKebab(name string) string {
	name = strings.TrimSuffix(name, ".rst")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ToLower(name)
	return name + ".mdx"
}

// mapCanton dispatches canton/<ver>/<subsite>/... paths.
// parts[0] == "canton", parts[1] is the version, parts[2] is the subsite.
func mapCanton(parts []string, stem string) (DerivedPath, bool) {
	if len(parts) < 3 {
		return "", false
	}
	subsite := parts[2]
	leafDir := ""
	if len(parts) >= 5 {
		leafDir = parts[3]
	}

	prefixed := "canton-" + stem

	switch subsite {
	case "participant":
		switch leafDir {
		case "tutorials":
			return DerivedPath("appdev/tutorials/" + prefixed), true
		case "howtos":
			return DerivedPath("appdev/howtos/" + prefixed), true
		default:
			return DerivedPath("appdev/reference/" + prefixed), true
		}
	case "synchronizer":
		return DerivedPath("global-synchronizer/" + prefixed), true
	case "sdk":
		return DerivedPath("appdev/sdk/" + prefixed), true
	case "overview":
		return DerivedPath("overview/learn/" + prefixed), true
	default:
		return DerivedPath("appdev/reference/" + prefixed), true
	}
}

// mapDaml dispatches daml/<ver>/<section>/... paths.
func mapDaml(parts []string, stem string) (DerivedPath, bool) {
	prefixed := "daml-" + stem
	if len(parts) < 4 {
		return DerivedPath("appdev/reference/" + prefixed), true
	}
	section := parts[2]
	leafDir := parts[3]
	switch section {
	case "sdk":
		switch leafDir {
		case "tutorials":
			return DerivedPath("appdev/tutorials/" + prefixed), true
		case "howtos":
			return DerivedPath("appdev/howtos/" + prefixed), true
		case "modules":
			return DerivedPath("appdev/modules/" + prefixed), true
		default:
			return DerivedPath("appdev/reference/" + prefixed), true
		}
	}
	return DerivedPath("appdev/reference/" + prefixed), true
}

// mapQuickstart dispatches quickstart/<ver>/sdk/quickstart/... paths.
func mapQuickstart(parts []string, stem string) (DerivedPath, bool) {
	// Everything under quickstart/ collapses to appdev/quickstart/<stem>.
	// The intermediate directories (download/, explore/, structure/) are
	// already reflected in the filenames in the existing migrated corpus.
	return DerivedPath("appdev/quickstart/" + stem), true
}
