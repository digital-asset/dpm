// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memResolver is an in-memory Resolver for hermetic tests.
type memResolver map[string][]byte

func (m memResolver) Read(p string) ([]byte, error) {
	if b, ok := m[p]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("not found: %s", p)
}

func TestLiteralInclude_WholeFile(t *testing.T) {
	rst := `Before.

.. literalinclude:: code/foo.json
    :language: json

After.`
	fs := memResolver{
		"/src/code/foo.json": []byte(`{"a": 1}
{"b": 2}
`),
	}
	out, err := Resolve(rst, Options{
		SourcePath: "/src/main.rst",
		Resolver:   fs,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `Before.

.. code-block:: json

   {"a": 1}
   {"b": 2}

After.`
	if !equalTrim(out, want) {
		t.Errorf("mismatch:\nwant:\n%s\n got:\n%s", want, out)
	}
}

func TestLiteralInclude_LanguageInferredFromExt(t *testing.T) {
	rst := `.. literalinclude:: snippets/foo.daml`
	fs := memResolver{
		"/src/snippets/foo.daml": []byte("template Foo\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, ".. code-block:: daml") {
		t.Errorf("expected inferred 'daml' tag; got:\n%s", out)
	}
}

func TestLiteralInclude_LinesSpec(t *testing.T) {
	rst := `.. literalinclude:: code.txt
    :lines: 2-3`
	fs := memResolver{
		"/src/code.txt": []byte("line one\nline two\nline three\nline four\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "line two") || !strings.Contains(out, "line three") {
		t.Errorf("expected lines 2-3; got:\n%s", out)
	}
	if strings.Contains(out, "line one") || strings.Contains(out, "line four") {
		t.Errorf("lines outside range leaked through; got:\n%s", out)
	}
}

func TestLiteralInclude_LinesOpenEndedRange(t *testing.T) {
	// `:lines: N-` (no upper bound) is a common Sphinx idiom meaning
	// "from line N to end of file." Real corpus content uses it (e.g.
	// docs-website/.../dpm/3.5/manual-install.rst).
	rst := `.. literalinclude:: code.txt
    :lines: 3-`
	fs := memResolver{
		"/src/code.txt": []byte("alpha\nbeta\ngamma\ndelta\nepsilon\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gamma", "delta", "epsilon"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in open-ended slice output; got:\n%s", want, out)
		}
	}
	for _, dontWant := range []string{"alpha", "beta"} {
		if strings.Contains(out, dontWant) {
			t.Errorf("line %q should be excluded by :lines: 3-; got:\n%s", dontWant, out)
		}
	}
}

func TestLiteralInclude_LinesInvalidRangeStillErrors(t *testing.T) {
	rst := `.. literalinclude:: code.txt
    :lines: 5-2`
	fs := memResolver{
		"/src/code.txt": []byte("a\nb\nc\nd\ne\nf\n"),
	}
	_, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs, Strict: true})
	if err == nil {
		t.Error("expected error on inverted range :lines: 5-2 in strict mode")
	}
}

func TestLiteralInclude_StartAfterEndBefore(t *testing.T) {
	rst := `.. literalinclude:: code.txt
    :start-after: BEGIN
    :end-before: END`
	fs := memResolver{
		"/src/code.txt": []byte("ignored\nBEGIN\nkept\nalso kept\nEND\nignored again\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kept", "also kept"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, banned := range []string{"ignored", "BEGIN", "END"} {
		if strings.Contains(out, banned) {
			t.Errorf("unwanted %q in:\n%s", banned, out)
		}
	}
}

func TestLiteralInclude_Dedent(t *testing.T) {
	rst := `.. literalinclude:: code.ts
    :dedent:`
	fs := memResolver{
		"/src/code.ts": []byte("    function foo() {\n      return 1\n    }\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	// After dedent, the content should start with "function foo", not
	// "    function foo".
	if !strings.Contains(out, "   function foo") { // 3-space indent inside the directive body
		t.Errorf("dedent did not normalize leading whitespace:\n%s", out)
	}
	if strings.Contains(out, "       function foo") { // 7+ spaces = not dedented
		t.Errorf("content was not dedented:\n%s", out)
	}
}

func TestInclude_SplicesContent(t *testing.T) {
	rst := `Before include.

.. include:: fragment.rst.inc

After include.`
	fs := memResolver{
		"/src/fragment.rst.inc": []byte(`Spliced content line 1.
Spliced content line 2.
`),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Before include.", "Spliced content line 1.", "Spliced content line 2.", "After include."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestInclude_Recursive(t *testing.T) {
	rst := `.. include:: outer.rst.inc`
	fs := memResolver{
		"/src/outer.rst.inc": []byte("Before nested.\n\n.. include:: inner.rst.inc\n\nAfter nested.\n"),
		"/src/inner.rst.inc": []byte("Innermost content.\n"),
	}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Before nested.", "Innermost content.", "After nested."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestInclude_MissingStrictErrors(t *testing.T) {
	rst := `.. include:: does-not-exist.rst.inc`
	fs := memResolver{}
	_, err := Resolve(rst, Options{
		SourcePath: "/src/main.rst",
		Resolver:   fs,
		Strict:     true,
	})
	if err == nil {
		t.Error("expected strict missing-include to error")
	}
}

func TestInclude_MissingLenientMarker(t *testing.T) {
	rst := `.. include:: does-not-exist.rst.inc`
	fs := memResolver{}
	out, err := Resolve(rst, Options{SourcePath: "/src/main.rst", Resolver: fs})
	if err != nil {
		t.Fatalf("lenient mode should not error: %v", err)
	}
	if !strings.Contains(out, "include-missing") {
		t.Errorf("expected include-missing marker; got:\n%s", out)
	}
}

func TestResolveIncludePath_Absolute(t *testing.T) {
	rst := `.. include:: /shared/common.rst.inc`
	fs := memResolver{
		"/docs/shared/common.rst.inc": []byte("from docs root\n"),
	}
	out, err := Resolve(rst, Options{
		SourcePath: "/docs/sub/page.rst",
		DocsRoot:   "/docs",
		Resolver:   fs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "from docs root") {
		t.Errorf("expected absolute-path resolution; got:\n%s", out)
	}
}

// Smoke against a real file on disk to confirm OSResolver works.
func TestLiteralInclude_OSResolver(t *testing.T) {
	dir := t.TempDir()
	codePath := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(codePath, []byte(`{"ok": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.rst")
	rst := `.. literalinclude:: sample.json
    :language: json
`
	out, err := Resolve(rst, Options{SourcePath: mainPath})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `{"ok": true}`) {
		t.Errorf("OSResolver did not read file:\n%s", out)
	}
}

func equalTrim(a, b string) bool {
	return strings.TrimRight(a, "\n ") == strings.TrimRight(b, "\n ")
}
