// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertListTable_HeaderAndRows(t *testing.T) {
	in := `.. list-table:: Token mapping
   :header-rows: 1
   :widths: 20 80

   * - Token
     - Description
   * - alpha
     - The first
   * - beta
     - The second
`
	got := convertListTable(in)
	for _, want := range []string{
		"**Token mapping**",
		"| Token | Description |",
		"| --- | --- |",
		"| alpha | The first |",
		"| beta | The second |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "list-table::") {
		t.Errorf("directive line was not consumed:\n%s", got)
	}
}

func TestConvertListTable_NoHeaderRows(t *testing.T) {
	in := `.. list-table::

   * - one
     - two
   * - three
     - four
`
	got := convertListTable(in)
	// Without :header-rows: the first row is still treated as the
	// header so the output is a valid markdown table.
	if !strings.Contains(got, "| one | two |") {
		t.Errorf("first row missing: %s", got)
	}
	if !strings.Contains(got, "| --- | --- |") {
		t.Errorf("alignment row missing: %s", got)
	}
	if !strings.Contains(got, "| three | four |") {
		t.Errorf("body row missing: %s", got)
	}
}

func TestConvertCsvTable_FileOption(t *testing.T) {
	// csv-table with :file: should read the external file relative to
	// the source RST, parse it, and emit a markdown table.
	dir := t.TempDir()
	srcRST := filepath.Join(dir, "page.rst")
	csvPath := filepath.Join(dir, "data.csv")
	csvBody := "Name,Role\nAlice,Lead\nBob,Engineer\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o644); err != nil {
		t.Fatal(err)
	}

	in := `.. csv-table:: People
   :file: data.csv
   :header-rows: 1
`
	got := convertCsvTable(in, Options{SourcePath: srcRST})
	for _, want := range []string{
		"**People**",
		"| Name | Role |",
		"| Alice | Lead |",
		"| Bob | Engineer |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConvertCsvTable_FileWithCustomDelimAndQuote(t *testing.T) {
	// Mirrors the canton/.../lf-value-specification.rst dialect:
	// :delim: ; :quote: $ :escape: ^
	dir := t.TempDir()
	srcRST := filepath.Join(dir, "page.rst")
	csvPath := filepath.Join(dir, "data.csv")
	csvBody := "Name; Role\n$Alice; lead$ ; $primary$\n$Bob; eng$ ; $secondary$\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o644); err != nil {
		t.Fatal(err)
	}

	in := `.. csv-table:: Custom
   :file: data.csv
   :delim: ;
   :quote: $
   :escape: ^
   :header-rows: 1
`
	got := convertCsvTable(in, Options{SourcePath: srcRST})
	if !strings.Contains(got, "| Name | Role |") {
		t.Errorf("expected header row in output:\n%s", got)
	}
	if !strings.Contains(got, "Alice; lead") {
		t.Errorf("expected `$`-quoted multi-token cell to parse intact:\n%s", got)
	}
}

func TestConvertCsvTable_FileMissingProducesMarker(t *testing.T) {
	in := `.. csv-table::
   :file: nonexistent.csv
`
	got := convertCsvTable(in, Options{SourcePath: "/tmp/page.rst"})
	if !strings.Contains(got, "csv-table: nonexistent.csv") {
		t.Errorf("expected unresolved-file marker; got:\n%s", got)
	}
}

func TestConvertCsvTable(t *testing.T) {
	in := `.. csv-table:: Daml LF JSON encoding
   :header: "Daml Type", "JSON"

   "Int64", "string"
   "Numeric", "string"
   "Bool", "boolean"
`
	got := convertCsvTable(in, Options{})
	for _, want := range []string{
		"**Daml LF JSON encoding**",
		"| Daml Type | JSON |",
		"| Int64 | string |",
		"| Numeric | string |",
		"| Bool | boolean |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConvertGridTable_Basic(t *testing.T) {
	in := `+----------+----------+
| Header 1 | Header 2 |
+==========+==========+
| Cell 1   | Cell 2   |
+----------+----------+
| Multi    | Cell     |
+----------+----------+
`
	got := convertGridTable(in)
	for _, want := range []string{
		"| Header 1 | Header 2 |",
		"| --- | --- |",
		"| Cell 1 | Cell 2 |",
		"| Multi | Cell |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "+----") {
		t.Errorf("RST border survived: %s", got)
	}
}

func TestConvertGridTable_NoHeaderSep_KeepsAllAsBody(t *testing.T) {
	// A grid table without `+===+` separator: the parser should still
	// emit a markdown table (treating row 1 as header by default).
	in := `+-----+-----+
| a   | b   |
+-----+-----+
| 1   | 2   |
+-----+-----+
`
	got := convertGridTable(in)
	if !strings.Contains(got, "| a | b |") {
		t.Errorf("first row not in output:\n%s", got)
	}
	if !strings.Contains(got, "| 1 | 2 |") {
		t.Errorf("second row not in output:\n%s", got)
	}
}

func TestConvertGridTable_BadShape_PassthroughDoesNotPanic(t *testing.T) {
	// A line that LOOKS like a border but isn't a real grid table
	// shouldn't crash the converter and shouldn't be silently eaten.
	in := `+--+ this is not a real table
just text
`
	// The first line still matches reGridBorder; the parser will fail
	// to find content rows and pass through.
	got := convertGridTable(in)
	if !strings.Contains(got, "just text") {
		t.Errorf("non-table content was eaten: %s", got)
	}
}

func TestConvertListTable_PipeInCellEscaped(t *testing.T) {
	in := `.. list-table::
   :header-rows: 1

   * - Pattern
     - Match
   * - ` + "`a|b`" + `
     - either
`
	got := convertListTable(in)
	if !strings.Contains(got, `\|`) {
		t.Errorf("pipe in cell was not escaped:\n%s", got)
	}
}
