// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunner_RunBytes(t *testing.T) {
	r := Runner{Validators: DefaultValidators()}

	good := []byte(`---
title: Hello
---

body
`)
	if got := r.RunBytes("a.mdx", good); len(got) != 0 {
		t.Errorf("expected no findings on good MDX, got %v", got)
	}

	bad := []byte(`# missing frontmatter
`)
	got := r.RunBytes("b.mdx", bad)
	if len(got) != 1 || got[0].Code != "frontmatter-missing" {
		t.Errorf("expected one frontmatter-missing finding, got %v", got)
	}
}

func TestRunner_RunPaths(t *testing.T) {
	dir := t.TempDir()

	// Three files: one valid, one missing title, one not .mdx (ignored).
	files := map[string]string{
		"good.mdx": "---\ntitle: Good\n---\n\nbody\n",
		"bad.mdx":  "---\ndescription: no title here\n---\n\nbody\n",
		"skip.txt": "not an MDX file",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r := Runner{Validators: DefaultValidators()}
	findings, counts, err := r.RunPaths([]string{dir})
	if err != nil {
		t.Fatalf("RunPaths: %v", err)
	}
	if counts.Files != 2 {
		t.Errorf("expected 2 .mdx files walked, got %d", counts.Files)
	}
	if counts.Errors != 1 {
		t.Errorf("expected 1 error, got %d", counts.Errors)
	}
	if counts.Warnings != 0 {
		t.Errorf("expected 0 warnings, got %d", counts.Warnings)
	}
	if len(findings) != 1 || findings[0].Code != "frontmatter-missing-title" {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestRunner_HasBlockingErrors(t *testing.T) {
	cases := []struct {
		name    string
		counts  Counts
		strict  bool
		want    bool
	}{
		{"clean", Counts{Errors: 0, Warnings: 0}, false, false},
		{"clean strict", Counts{Errors: 0, Warnings: 0}, true, false},
		{"warnings only", Counts{Errors: 0, Warnings: 3}, false, false},
		{"warnings strict", Counts{Errors: 0, Warnings: 3}, true, true},
		{"errors", Counts{Errors: 1, Warnings: 0}, false, true},
		{"errors strict", Counts{Errors: 1, Warnings: 5}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.counts.HasBlockingErrors(tc.strict); got != tc.want {
				t.Errorf("HasBlockingErrors(strict=%v) = %v, want %v",
					tc.strict, got, tc.want)
			}
		})
	}
}

func TestSkipPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"docs-main/foo.mdx", false},
		{"docs-main/appdev/quickstart.mdx", false},
		{"docs-main/snippets/header.mdx", true},
		{"docs-main/snippets/external/foo.mdx", true},
		{"docs-main/appdev/snippets/example.mdx", true},
		{"snippets/top-level.mdx", true},
		{"docs-main/snippets-archive/foo.mdx", false}, // segment match, not prefix
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := SkipPath(tc.path); got != tc.want {
				t.Errorf("SkipPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestRunPaths_SkipsSnippets(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "snippets"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"page.mdx":             "---\ntitle: P\n---\nbody\n",
		"snippets/partial.mdx": "no frontmatter, but a snippet, so skip\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := Runner{Validators: DefaultValidators()}
	findings, counts, err := r.RunPaths([]string{dir})
	if err != nil {
		t.Fatalf("RunPaths: %v", err)
	}
	if counts.Files != 1 {
		t.Errorf("expected 1 file (snippet skipped), got %d", counts.Files)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings on a valid page when snippet is skipped, got %v", findings)
	}
}

func TestFormatFindings_StableOrder(t *testing.T) {
	findings := []Finding{
		{Path: "z.mdx", Line: 1, Severity: Error, Code: "x", Message: "z"},
		{Path: "a.mdx", Line: 2, Severity: Error, Code: "x", Message: "a2"},
		{Path: "a.mdx", Line: 1, Severity: Error, Code: "x", Message: "a1"},
		{Path: "a.mdx", Line: 1, Severity: Error, Code: "y", Message: "a1y"},
	}
	var buf bytes.Buffer
	FormatFindings(&buf, findings)
	got := strings.TrimRight(buf.String(), "\n")
	want := strings.Join([]string{
		"a.mdx:1: error x: a1",
		"a.mdx:1: error y: a1y",
		"a.mdx:2: error x: a2",
		"z.mdx:1: error x: z",
	}, "\n")
	if got != want {
		t.Errorf("format order mismatch:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
