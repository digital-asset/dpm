// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_ValidFileExitsZero(t *testing.T) {
	path := writeFile(t, "good.mdx", "---\ntitle: T\n---\n\nbody\n")
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{path}); code != 0 {
		t.Errorf("exit=%d stderr=%q", code, stderr.String())
	}
}

func TestRun_MissingTitleExitsOne(t *testing.T) {
	path := writeFile(t, "bad.mdx", "---\ndescription: nope\n---\n\nbody\n")
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{path}); code != 1 {
		t.Errorf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "frontmatter-missing-title") {
		t.Errorf("stdout should mention finding code; got %q", stdout.String())
	}
}

func TestRun_NoFrontmatterExitsOne(t *testing.T) {
	path := writeFile(t, "naked.mdx", "# Page\n\nbody\n")
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{path}); code != 1 {
		t.Errorf("exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "frontmatter-missing") {
		t.Errorf("expected frontmatter-missing in stdout, got %q", stdout.String())
	}
}

func TestRun_NonMDXFileIsDropped(t *testing.T) {
	path := writeFile(t, "notes.txt", "not an MDX file\n")
	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{path})
	if code != 0 {
		t.Errorf("exit=%d, want 0 (non-mdx files are silently dropped)", code)
	}
	if !strings.Contains(stdout.String(), "no .mdx files") {
		t.Errorf("expected 'no .mdx files' message, got %q", stdout.String())
	}
}

func TestRun_StrictPromotesWarnings(t *testing.T) {
	// No warning-emitting validators, so this case is exercised
	// by Counts.HasBlockingErrors directly in runner_test.go. When a
	// warning-emitting validator lands, expand this to invoke run() with
	// a fixture that produces only warnings, both with and without
	// --strict, and assert exit codes 0 vs 1.
	t.Skip("no warning-emitting validators registered yet")
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{"--version"}); code != 0 {
		t.Errorf("exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "mdx-validate") {
		t.Errorf("stdout should mention tool name, got %q", stdout.String())
	}
}

func TestRun_HelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{"-h"}); code != 0 {
		t.Errorf("exit=%d for -h, want 0; stderr=%q", code, stderr.String())
	}
}

func TestRun_StagedRejectsExplicitPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"--staged", "foo.mdx"})
	if code != 2 {
		t.Errorf("exit=%d, want 2 (usage error)", code)
	}
	if !strings.Contains(stderr.String(), "--staged cannot be combined with explicit paths") {
		t.Errorf("expected mutual-exclusion message, got %q", stderr.String())
	}
}

func TestRun_UnknownFlagIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(&stdout, &stderr, []string{"--no-such-flag"}); code != 2 {
		t.Errorf("exit=%d, want 2", code)
	}
}

func TestRun_DefaultDirMissingHintsRepoRoot(t *testing.T) {
	// Run inside an empty tmp dir so ./docs-main does not exist.
	withCwd(t, t.TempDir(), func() {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, nil)
		if code != 2 {
			t.Errorf("exit=%d, want 2 when default dir is missing", code)
		}
		if !strings.Contains(stderr.String(), "run from the repo root") {
			t.Errorf("stderr should hint repo-root, got %q", stderr.String())
		}
	})
}

// writeFile creates a file in t.TempDir() with the given name + content
// and returns its path. Cleaned up automatically by t.TempDir.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// withCwd switches working directory for the duration of fn and restores
// the original on return.
func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}
