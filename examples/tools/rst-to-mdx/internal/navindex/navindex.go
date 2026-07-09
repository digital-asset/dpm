// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package navindex parses a Mintlify docs.json file and exposes the
// flat list of page paths registered in the navigation tree. It's the
// authoritative source of truth for what pages actually exist on the
// site, so cross-reference resolution can prefer real docs.json hits
// over algorithmically-derived target paths that may not match the
// human-curated navigation layout.
//
// docs.json shape (Mintlify):
//
//	{
//	  "navigation": {
//	    "dropdowns": [
//	      {
//	        "dropdown": "App Development",
//	        "versions": [
//	          {"version": "MainNet", "groups": [
//	            {"group": "Get Started", "pages": ["appdev/get-started/foo", ...]}
//	          ]}
//	        ]
//	      }
//	    ]
//	  }
//	}
//
// Pages can also be nested groups, so we walk the whole tree generically
// and collect every string value that looks like a page slug (contains
// `/` and isn't an external URL).
package navindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

// Index holds the flat list of page paths from docs.json plus a few
// indices for fast lookup.
type Index struct {
	pages       []string
	pageSet     map[string]struct{}
	byBasename  map[string][]string
	byStem      map[string][]string
}

// Build reads jsonPath and constructs an Index. The JSON parser is
// lenient about the navigation shape: any string that looks like a
// relative page path (contains `/`, doesn't start with `http`) is
// collected.
func Build(jsonPath string) (*Index, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", jsonPath, err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", jsonPath, err)
	}

	idx := &Index{
		pageSet:    make(map[string]struct{}),
		byBasename: make(map[string][]string),
		byStem:     make(map[string][]string),
	}
	idx.collect(raw)

	// Deterministic ordering helps test stability and makes the
	// FindBy* lookups predictable.
	sort.Strings(idx.pages)
	for k := range idx.byBasename {
		sort.Strings(idx.byBasename[k])
	}
	for k := range idx.byStem {
		sort.Strings(idx.byStem[k])
	}
	return idx, nil
}

// collect walks the JSON tree and harvests page-path-shaped strings.
func (i *Index) collect(node any) {
	switch v := node.(type) {
	case map[string]any:
		for _, child := range v {
			i.collect(child)
		}
	case []any:
		for _, child := range v {
			i.collect(child)
		}
	case string:
		if isPagePath(v) {
			i.add(v)
		}
	}
}

// isPagePath returns true for strings that look like Mintlify page
// slugs: relative path with at least one `/`, no scheme prefix.
func isPagePath(s string) bool {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		// Mintlify navigation paths are conventionally relative; an
		// absolute path is more likely a config value (favicon path
		// etc.) than a page slug.
		return false
	}
	if !strings.Contains(s, "/") {
		return false
	}
	// Reject obvious non-pages.
	if strings.HasSuffix(s, ".png") || strings.HasSuffix(s, ".jpg") ||
		strings.HasSuffix(s, ".svg") || strings.HasSuffix(s, ".ico") ||
		strings.HasSuffix(s, ".css") || strings.HasSuffix(s, ".js") {
		return false
	}
	return true
}

func (i *Index) add(p string) {
	if _, seen := i.pageSet[p]; seen {
		return
	}
	i.pageSet[p] = struct{}{}
	i.pages = append(i.pages, p)
	base := path.Base(p)
	i.byBasename[base] = append(i.byBasename[base], p)
	stem := strings.TrimSuffix(base, path.Ext(base))
	i.byStem[stem] = append(i.byStem[stem], p)
}

// HasPage reports whether the given relative path is registered in
// the navigation. The check is exact and case-sensitive.
func (i *Index) HasPage(p string) bool {
	if i == nil {
		return false
	}
	_, ok := i.pageSet[p]
	return ok
}

// FindByBasename returns every page whose final segment exactly
// matches `name`. Used as a fallback when a derived path isn't an
// exact hit but a same-named page lives somewhere else in the tree.
func (i *Index) FindByBasename(name string) []string {
	if i == nil {
		return nil
	}
	return append([]string(nil), i.byBasename[name]...)
}

// FindByStem is like FindByBasename but matches the basename without
// any extension. Useful when comparing a page slug against an RST
// filename's stem.
func (i *Index) FindByStem(stem string) []string {
	if i == nil {
		return nil
	}
	return append([]string(nil), i.byStem[stem]...)
}

// BestMatch tries to pick the single most likely Mintlify page path
// for an RST source by comparing path components. It returns "" when
// no candidate looks plausible. The heuristic:
//
//  1. Exact path match wins.
//  2. Otherwise we collect candidates by stem of the RST filename.
//  3. Among candidates, we score by how many path segments overlap
//     with the RST source path (preferring nearer parents). If a
//     single candidate has the highest score, return it; ties leave
//     the resolution ambiguous so we return "" and let the caller
//     decide.
func (i *Index) BestMatch(rstRel string) string {
	if i == nil {
		return ""
	}
	if i.HasPage(rstRel) {
		return rstRel
	}
	stem := strings.TrimSuffix(path.Base(rstRel), path.Ext(rstRel))
	stem = strings.ReplaceAll(stem, "_", "-")
	stem = strings.ToLower(stem)

	candidates := i.byStem[stem]
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	rstParts := splitParts(rstRel)
	bestScore := -1
	bestList := []string{}
	for _, cand := range candidates {
		score := segmentOverlap(rstParts, splitParts(cand))
		if score > bestScore {
			bestScore = score
			bestList = []string{cand}
		} else if score == bestScore {
			bestList = append(bestList, cand)
		}
	}
	if len(bestList) == 1 {
		return bestList[0]
	}
	return ""
}

// Pages returns a copy of every page path in the index. Sorted.
func (i *Index) Pages() []string {
	if i == nil {
		return nil
	}
	return append([]string(nil), i.pages...)
}

// Size returns the number of unique pages.
func (i *Index) Size() int {
	if i == nil {
		return 0
	}
	return len(i.pages)
}

func splitParts(p string) []string {
	out := strings.Split(p, "/")
	res := out[:0]
	for _, s := range out {
		if s != "" {
			res = append(res, s)
		}
	}
	return res
}

// segmentOverlap counts how many path components from `a` appear in
// `b` (multiset intersection). It's a cheap proxy for "these paths
// describe related pages"; precise enough to disambiguate basename
// collisions across version trees.
func segmentOverlap(a, b []string) int {
	bag := map[string]int{}
	for _, p := range b {
		bag[p]++
	}
	score := 0
	for _, p := range a {
		if bag[p] > 0 {
			bag[p]--
			score++
		}
	}
	return score
}
