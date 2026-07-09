// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RST has three table flavors that we convert to markdown tables:
//
//  1. .. list-table::    — bullet-list body, most common (~301 in corpus)
//  2. .. csv-table::     — CSV body with an inline :header: option
//  3. grid tables (+---+) — ASCII borders, also common (~600 in corpus)
//
// All three emit GitHub-flavored markdown tables. For cells with
// multi-line content we fall back to HTML tables so the renderer can
// handle the complexity; the migration guide already recommends that.

// convertTables is the single entry point; it runs all three parsers in
// order. The list-table and csv-table parsers are directive-driven so
// they run before the grid-table parser, which is shape-driven and
// would otherwise greedily match borders inside an already-converted
// table.
//
// `opts` is forwarded to the csv-table parser, which needs SourcePath
// and DocsRoot to resolve the `:file:` option for external CSV files.
func convertTables(s string, opts Options) string {
	s = convertListTable(s)
	s = convertCsvTable(s, opts)
	s = convertGridTable(s)
	s = convertSimpleTables(s)
	return s
}

// ---------------------------------------------------------------------
// .. list-table::
// ---------------------------------------------------------------------

var (
	reListTableStart = regexp.MustCompile(
		`^(\s*)\.\.\s+list-table::\s*(.*)$`)
	reBulletRow  = regexp.MustCompile(`^(\s*)\*\s+-\s*(.*)$`)
	reCellCont   = regexp.MustCompile(`^(\s*)-\s*(.*)$`)
	reOptionLn   = regexp.MustCompile(`^(\s+):([A-Za-z][A-Za-z0-9_\-]*):\s*(.*)$`)
)

// convertListTable finds every `.. list-table::` directive and rewrites
// it as a markdown table. Nested inline markup inside cells is
// preserved verbatim — downstream inline-role transforms fix it up.
func convertListTable(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		m := reListTableStart.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		indent := m[1]
		title := strings.TrimSpace(m[2])
		i++

		opts, consumed := readTableOptions(lines[i:])
		i += consumed

		// Skip blank lines before the body.
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}

		// Collect indented body lines.
		var body []string
		for i < len(lines) {
			line := lines[i]
			if strings.TrimSpace(line) == "" {
				body = append(body, "")
				i++
				continue
			}
			if !strings.HasPrefix(line, indent) || !deeperIndent(line, indent) {
				break
			}
			body = append(body, line)
			i++
		}

		rows := parseListTableBody(body)
		headerRows := parseInt(opts["header-rows"], 0)
		md := renderMarkdownTable(title, rows, headerRows, indent)
		out = append(out, md...)
	}
	return strings.Join(out, "\n")
}

// parseListTableBody walks the directive body and produces a [][]string
// where outer slice is rows, inner slice is cells. A new row starts
// with `* -` at the least-indented level; subsequent `-` bullets at the
// next indent level add cells to the current row. Cell content can
// span multiple lines (indented under the `-`).
func parseListTableBody(lines []string) [][]string {
	var rows [][]string
	var currentRow []string
	var currentCell []string
	flushCell := func() {
		if len(currentCell) > 0 || currentRow != nil {
			currentRow = append(currentRow, strings.TrimSpace(strings.Join(currentCell, " ")))
			currentCell = currentCell[:0]
		}
	}
	flushRow := func() {
		flushCell()
		if currentRow != nil {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := reBulletRow.FindStringSubmatch(line); m != nil {
			flushRow()
			currentRow = []string{}
			if txt := strings.TrimSpace(m[2]); txt != "" {
				currentCell = append(currentCell, txt)
			}
			continue
		}
		if m := reCellCont.FindStringSubmatch(line); m != nil {
			flushCell()
			if txt := strings.TrimSpace(m[2]); txt != "" {
				currentCell = append(currentCell, txt)
			}
			continue
		}
		// Continuation of the current cell's content on a wrapped
		// line. Append trimmed text so cells collapse into one line.
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			currentCell = append(currentCell, trimmed)
		}
	}
	flushRow()
	return rows
}

// ---------------------------------------------------------------------
// .. csv-table::
// ---------------------------------------------------------------------

var reCsvTableStart = regexp.MustCompile(
	`^(\s*)\.\.\s+csv-table::\s*(.*)$`)

// convertCsvTable handles `.. csv-table::` directives. Body sources:
//   - `:file: <path>` — external CSV file, resolved relative to the
//     including RST file (or DocsRoot for `/`-prefixed paths). Supports
//     `:delim:`, `:quote:`, `:escape:` to handle non-standard CSV
//     dialects (Sphinx allows arbitrary single-character overrides).
//   - inline body — indented CSV rows directly under the directive.
//
// Multi-line cells in the source (CSV fields containing literal `\n`)
// are rendered as `<br>`-separated text in the markdown cell, since
// GitHub-flavored markdown tables don't support real multi-line cells.
func convertCsvTable(s string, callerOpts Options) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		m := reCsvTableStart.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		indent := m[1]
		title := strings.TrimSpace(m[2])
		i++

		dopts, consumed := readTableOptions(lines[i:])
		i += consumed

		// Skip blank lines before any inline body.
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}

		// Inline body lines (collected only when there is no :file:).
		var body []string
		if dopts["file"] == "" {
			for i < len(lines) {
				line := lines[i]
				if strings.TrimSpace(line) == "" {
					break
				}
				if !deeperIndent(line, indent) {
					break
				}
				body = append(body, strings.TrimLeft(line, " \t"))
				i++
			}
		}

		// Build the table body rows, either from the file or inline.
		var rows [][]string
		var headerRows int
		if file := dopts["file"]; file != "" {
			fileRows, err := loadCSVFile(file, dopts, callerOpts)
			if err != nil {
				out = append(out, indent+
					fmt.Sprintf("{/* csv-table: %s (%v) */}", file, err))
				continue
			}
			rows = fileRows
		} else {
			if h := dopts["header"]; h != "" {
				if hdr, err := parseCSVRow(h, ','); err == nil {
					rows = append(rows, hdr)
				}
			}
			for _, row := range body {
				parsed, err := parseCSVRow(row, ',')
				if err != nil {
					continue
				}
				rows = append(rows, parsed)
			}
		}
		// Determine how many leading rows are header rows.
		switch {
		case dopts["header"] != "":
			headerRows = 1
		case dopts["header-rows"] != "":
			if n := atoiOr(dopts["header-rows"], 0); n > 0 {
				headerRows = n
			}
		}

		md := renderMarkdownTable(title, rows, headerRows, indent)
		out = append(out, md...)
	}
	return strings.Join(out, "\n")
}

// parseCSVRow parses a single CSV row string with the given delimiter.
// LazyQuotes is on so unbalanced quotes (common in inline RST cells with
// backticks) don't abort parsing.
func parseCSVRow(line string, delim rune) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.Comma = delim
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	record, err := r.Read()
	if err != nil {
		return nil, err
	}
	return record, nil
}

// loadCSVFile resolves and parses an external CSV file referenced by
// `:file: <path>`. Path resolution mirrors literalinclude: relative to
// the including RST file, or to DocsRoot for `/`-prefixed absolutes.
//
// `:delim:` overrides the field separator (default `,`).
// `:quote:` and `:escape:` substitute for the standard `"` and `\`
// before parsing — Go's csv package only knows about `"`-quoted fields,
// so we normalize first. This is best-effort and works for the corpus's
// 5 csv-table sites; if a file's chosen quote or escape character also
// appears as literal data, expect noise.
//
// Multi-line CSV fields (literal `\n` between matching quotes) are
// returned with the newline preserved in the cell value; the renderer
// translates them to `<br>` for the markdown cell.
func loadCSVFile(rawPath string, dopts map[string]string, callerOpts Options) ([][]string, error) {
	abs, err := resolveCsvFilePath(rawPath, callerOpts)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rawPath, err)
	}
	body := stripUTF8BOM(data)

	// Normalize non-standard quote / escape characters into the form
	// the Go csv parser expects.
	if q := dopts["quote"]; q != "" && q != `"` {
		body = replaceFirstRune(body, q, `"`)
	}
	if e := dopts["escape"]; e != "" && e != `\` {
		body = replaceFirstRune(body, e, `\`)
	}

	delim := ','
	if d := dopts["delim"]; d != "" {
		// Sphinx allows any single character; fall back to comma if
		// the user provided multi-byte rubbish.
		runes := []rune(d)
		if len(runes) == 1 {
			delim = runes[0]
		}
	}

	r := csv.NewReader(strings.NewReader(body))
	r.Comma = delim
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1 // tolerate ragged rows
	return r.ReadAll()
}

func resolveCsvFilePath(raw string, opts Options) (string, error) {
	if strings.HasPrefix(raw, "/") {
		if opts.DocsRoot == "" {
			return "", fmt.Errorf("absolute :file: path %q requires --docs-root", raw)
		}
		return filepath.Join(opts.DocsRoot, strings.TrimPrefix(raw, "/")), nil
	}
	if opts.SourcePath == "" {
		return "", fmt.Errorf("relative :file: path %q requires a source path", raw)
	}
	return filepath.Join(filepath.Dir(opts.SourcePath), raw), nil
}

func stripUTF8BOM(b []byte) string {
	const bom = "\xEF\xBB\xBF" // UTF-8 BOM byte sequence
	s := string(b)
	return strings.TrimPrefix(s, bom)
}

// replaceFirstRune swaps every occurrence of one single-rune string with
// another. Cheap and adequate because :quote:/:escape: chars are
// single-byte in practice.
func replaceFirstRune(s, from, to string) string {
	return strings.ReplaceAll(s, from, to)
}

// atoiOr parses s as an integer, falling back to dflt on any error.
func atoiOr(s string, dflt int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return dflt
	}
	return n
}

// ---------------------------------------------------------------------
// Grid tables: +---+---+
// ---------------------------------------------------------------------

// reGridBorder matches the `+---+---+` shape that opens/closes a grid
// table. The equals-sign form (`+===+`) separates a header row from
// body rows.
var (
	reGridBorder  = regexp.MustCompile(`^\s*\+[-+]+\+\s*$`)
	reGridHeader  = regexp.MustCompile(`^\s*\+=+\+[=+]*\s*$`)
	reGridContent = regexp.MustCompile(`^\s*\|`)
)

func convertGridTable(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		if !reGridBorder.MatchString(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}

		// Collect the full grid table (until a line that isn't a
		// border, content row, or header separator).
		start := i
		tableLines := []string{lines[i]}
		i++
		for i < len(lines) {
			l := lines[i]
			if reGridBorder.MatchString(l) || reGridHeader.MatchString(l) || reGridContent.MatchString(l) {
				tableLines = append(tableLines, l)
				i++
				continue
			}
			break
		}

		rendered, ok := renderGridTable(tableLines)
		if !ok {
			// Parse failure — pass the original lines through. Better
			// a stray RST grid table than a destroyed document.
			out = append(out, lines[start:i]...)
			continue
		}
		out = append(out, rendered...)
	}
	return strings.Join(out, "\n")
}

// renderGridTable converts a block of grid-table lines into a markdown
// table. Returns false if the block can't be parsed cleanly — the
// caller then leaves the block untouched.
//
// Simplification: we treat each `| … | … |` content row as ONE row of
// the final table and split on `|`. Cells that span multiple lines
// are joined with a space. This won't handle arbitrary merged cells
// (the RST grid-table spec allows row/column spans), but the Canton
// corpus uses simple rectangular tables where this is enough.
func renderGridTable(lines []string) ([]string, bool) {
	var rows [][]string
	headerRows := 0
	sawHeaderSep := false

	// Figure out the column boundary positions from the first border.
	var colBreaks []int
	for _, line := range lines {
		if reGridBorder.MatchString(line) {
			colBreaks = bordersInLine(line)
			break
		}
	}
	if len(colBreaks) < 2 {
		return nil, false
	}

	var pendingRow []string
	for _, line := range lines {
		if reGridHeader.MatchString(line) {
			// Flush whatever was accumulating as a header row first
			// so the "=" separator marks the boundary.
			if pendingRow != nil {
				rows = append(rows, pendingRow)
				pendingRow = nil
			}
			sawHeaderSep = true
			if headerRows == 0 {
				headerRows = len(rows)
			}
			continue
		}
		if reGridBorder.MatchString(line) {
			if pendingRow != nil {
				rows = append(rows, pendingRow)
				pendingRow = nil
			}
			continue
		}
		if reGridContent.MatchString(line) {
			cells := splitGridRow(line, colBreaks)
			if pendingRow == nil {
				pendingRow = cells
			} else {
				// Merge wrapped content with prior row's cells.
				for j := 0; j < len(cells) && j < len(pendingRow); j++ {
					if cells[j] = strings.TrimSpace(cells[j]); cells[j] != "" {
						pendingRow[j] = strings.TrimSpace(pendingRow[j]) + " " + cells[j]
					}
				}
			}
			continue
		}
	}
	if pendingRow != nil {
		rows = append(rows, pendingRow)
	}
	if len(rows) == 0 {
		return nil, false
	}
	if sawHeaderSep && headerRows == 0 {
		headerRows = 1
	}

	leadingIndent := leadingWS(lines[0])
	return renderMarkdownTable("", rows, headerRows, leadingIndent), true
}

// bordersInLine returns the rune offsets of every `+` in a grid
// border line, which mark the column boundaries. We work in rune-space
// so multibyte UTF-8 characters in subsequent content rows (e.g. `¹`
// or other superscripts in Canton's crypto-scheme tables) don't shift
// the alignment.
func bordersInLine(line string) []int {
	var out []int
	for i, r := range []rune(line) {
		if r == '+' {
			out = append(out, i)
		}
	}
	return out
}

// splitGridRow extracts cell contents from a content row
// (`| cell1  | cell2 |`) using rune-space column boundary offsets.
// Returns trimmed cells.
func splitGridRow(line string, breaks []int) []string {
	runes := []rune(line)
	var cells []string
	for k := 0; k < len(breaks)-1; k++ {
		start := breaks[k] + 1
		end := breaks[k+1]
		if start >= len(runes) {
			cells = append(cells, "")
			continue
		}
		if end > len(runes) {
			end = len(runes)
		}
		cells = append(cells, strings.TrimSpace(string(runes[start:end])))
	}
	return cells
}

// ---------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------

// readTableOptions consumes `:name: value` lines at the start of a
// directive body and returns them as a map.
func readTableOptions(lines []string) (map[string]string, int) {
	opts := map[string]string{}
	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			break
		}
		m := reOptionLn.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		opts[strings.ToLower(m[2])] = strings.TrimSpace(m[3])
		i++
	}
	return opts, i
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func deeperIndent(line, parentIndent string) bool {
	lws := leadingWS(line)
	return len(lws) > len(parentIndent)
}

// renderMarkdownTable turns rows into a GitHub-flavored markdown table
// preceded by a blank line and (optionally) a bold title line. The
// first `headerRows` rows become the header; when headerRows is 0, the
// first row is treated as the header (markdown requires one).
//
// The `_indent` parameter is intentionally ignored: markdown tables
// indented 4+ spaces render as code blocks, not tables. Always emit at
// column 0 even when the RST source nested the table inside a
// directive.
func renderMarkdownTable(title string, rows [][]string, headerRows int, _indent string) []string {
	if len(rows) == 0 {
		return []string{""}
	}

	// Normalize column count to the widest row.
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return []string{""}
	}

	padRow := func(r []string) []string {
		if len(r) < cols {
			r = append(r, make([]string, cols-len(r))...)
		}
		return r
	}

	var out []string
	out = append(out, "")
	if title != "" {
		out = append(out, "**"+title+"**")
		out = append(out, "")
	}

	if headerRows == 0 {
		headerRows = 1
	}
	if headerRows > len(rows) {
		headerRows = len(rows)
	}
	// Merge multi-row headers into a single `|`-joined line (markdown
	// can't express stacked headers directly).
	headerCells := make([]string, cols)
	for h := 0; h < headerRows && h < len(rows); h++ {
		for c, cell := range padRow(rows[h]) {
			if headerCells[c] == "" {
				headerCells[c] = cell
			} else if cell != "" {
				headerCells[c] += " — " + cell
			}
		}
	}

	out = append(out, "| "+strings.Join(escapeCells(headerCells), " | ")+" |")
	sep := make([]string, cols)
	for i := range sep {
		sep[i] = "---"
	}
	out = append(out, "| "+strings.Join(sep, " | ")+" |")

	for _, row := range rows[headerRows:] {
		out = append(out, "| "+strings.Join(escapeCells(padRow(row)), " | ")+" |")
	}
	out = append(out, "")
	return out
}

// ---------------------------------------------------------------------
// RST simple tables (=== === ruler format)
// ---------------------------------------------------------------------

// reSimpleTableRuler matches a line that is entirely runs of `=`
// separated by 1+ spaces — the column ruler of an RST simple table.
var reSimpleTableRuler = regexp.MustCompile(`^(\s*)(=+(?:\s+=+)+)\s*$`)

// convertSimpleTables detects RST simple tables and converts them to
// markdown tables. A simple table is delimited by ruler lines of `===`
// segments. The column boundaries are inferred from the ruler.
//
// Structure:
//
//	======= ============
//	Header1 Header2         ← header row(s)
//	======= ============    ← second ruler separates header from body
//	cell1   cell2           ← body rows
//	        continuation
//	cell3   cell4
//	======= ============    ← closing ruler
//
// When there is no second ruler (only open + close), the first content
// row is promoted to the header (markdown requires one).
func convertSimpleTables(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0

	for i < len(lines) {
		m := reSimpleTableRuler.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}

		indent := m[1]
		cols := parseRulerColumns(lines[i], len(indent))
		if len(cols) < 2 {
			out = append(out, lines[i])
			i++
			continue
		}

		tableLines, consumed := collectSimpleTable(lines[i:])
		if consumed < 3 {
			out = append(out, lines[i])
			i++
			continue
		}

		rows, headerRows := parseSimpleTableBody(tableLines, cols)
		if len(rows) == 0 {
			out = append(out, lines[i])
			i++
			continue
		}

		out = append(out, renderMarkdownTable("", rows, headerRows, indent)...)
		i += consumed
	}
	return strings.Join(out, "\n")
}

// rulerCol represents a column span in the ruler: byte offsets [start, end).
type rulerCol struct {
	start, end int
}

// parseRulerColumns extracts column boundaries from a ruler line. Each
// run of `=` defines a column; gaps of whitespace separate them.
func parseRulerColumns(ruler string, indentLen int) []rulerCol {
	var cols []rulerCol
	j := indentLen
	for j < len(ruler) {
		// Skip whitespace between columns.
		for j < len(ruler) && ruler[j] == ' ' {
			j++
		}
		if j >= len(ruler) {
			break
		}
		start := j
		for j < len(ruler) && ruler[j] == '=' {
			j++
		}
		if j > start {
			cols = append(cols, rulerCol{start, j})
		}
	}
	return cols
}

// collectSimpleTable gathers all lines from the opening ruler through
// the closing ruler. A simple table has 2 rulers (open + close) or 3
// (open + header-sep + close). We scan forward and stop at the ruler
// that is followed by EOF, a blank line, or a line that doesn't look
// like table content.
func collectSimpleTable(lines []string) ([]string, int) {
	if len(lines) < 3 {
		return nil, 0
	}
	var table []string
	table = append(table, lines[0])
	for i := 1; i < len(lines); i++ {
		table = append(table, lines[i])
		if reSimpleTableRuler.MatchString(lines[i]) {
			// This ruler is the closing ruler if it's followed by
			// EOF, a blank line, or a line that doesn't start with
			// whitespace within the table columns.
			if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" {
				return table, i + 1
			}
			// If the next line is another ruler, keep going (shouldn't
			// happen, but be safe).
			if reSimpleTableRuler.MatchString(lines[i+1]) {
				continue
			}
			// If the next line has content, this is a header separator
			// — keep collecting.
			continue
		}
		// Stop if we hit a completely empty line that's followed by
		// something that doesn't look like it belongs to the table
		// (a non-indented, non-empty line with no content in the
		// column range). But blank lines between rows ARE valid in
		// RST simple tables, so only break on double-blank.
	}
	// We ran off the end without finding a closing ruler. Not a valid
	// simple table.
	return nil, 0
}

// parseSimpleTableBody reads the content lines between rulers and
// splits them into rows and cells using the column boundaries. Returns
// the rows and the number of header rows.
func parseSimpleTableBody(tableLines []string, cols []rulerCol) ([][]string, int) {
	var headerLines, bodyLines []string
	// Find the ruler positions.
	var rulerPositions []int
	for i, line := range tableLines {
		if reSimpleTableRuler.MatchString(line) {
			rulerPositions = append(rulerPositions, i)
		}
	}

	switch len(rulerPositions) {
	case 2:
		// open ruler + close ruler: everything between is body,
		// first row promoted to header by renderMarkdownTable.
		bodyLines = tableLines[rulerPositions[0]+1 : rulerPositions[1]]
		return splitSimpleRows(bodyLines, cols), 0
	case 3:
		// open + header-sep + close: content between open and
		// header-sep is header, between header-sep and close is body.
		headerLines = tableLines[rulerPositions[0]+1 : rulerPositions[1]]
		bodyLines = tableLines[rulerPositions[1]+1 : rulerPositions[2]]
	default:
		// Unexpected ruler count; try best-effort with first and last.
		if len(rulerPositions) >= 2 {
			bodyLines = tableLines[rulerPositions[0]+1 : rulerPositions[len(rulerPositions)-1]]
			return splitSimpleRows(bodyLines, cols), 0
		}
		return nil, 0
	}

	headerRows := splitSimpleRows(headerLines, cols)
	bodyRows := splitSimpleRows(bodyLines, cols)
	all := append(headerRows, bodyRows...)
	return all, len(headerRows)
}

// splitSimpleRows groups content lines into logical rows and extracts
// cell text by column position. A new row starts when a line has
// non-whitespace in the first column's range. Continuation lines
// (leading whitespace in the first column) append to the current row.
func splitSimpleRows(lines []string, cols []rulerCol) [][]string {
	var rows [][]string
	var currentCells []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cells := extractCells(line, cols)

		firstColText := ""
		if len(cells) > 0 {
			firstColText = strings.TrimSpace(cells[0])
		}

		if firstColText != "" && currentCells == nil {
			// First row.
			currentCells = cells
		} else if firstColText != "" {
			// New row — flush previous.
			rows = append(rows, currentCells)
			currentCells = cells
		} else {
			// Continuation line — append to current cells.
			if currentCells == nil {
				currentCells = make([]string, len(cols))
			}
			for c, cell := range cells {
				text := strings.TrimSpace(cell)
				if text != "" && c < len(currentCells) {
					if currentCells[c] != "" {
						currentCells[c] += " " + text
					} else {
						currentCells[c] = text
					}
				}
			}
		}
	}
	if currentCells != nil {
		rows = append(rows, currentCells)
	}
	return rows
}

// extractCells slices a line into cell strings based on column
// boundaries. Characters beyond the line length are treated as empty.
func extractCells(line string, cols []rulerCol) []string {
	cells := make([]string, len(cols))
	for i, col := range cols {
		if col.start >= len(line) {
			cells[i] = ""
			continue
		}
		end := col.end
		// For the last column, extend to end of line to capture long values.
		if i == len(cols)-1 && len(line) > end {
			end = len(line)
		}
		if end > len(line) {
			end = len(line)
		}
		cells[i] = strings.TrimSpace(line[col.start:end])
	}
	return cells
}

// escapeCells replaces `|` inside cell text with `\|` so it doesn't
// break the table, and collapses runs of whitespace.
func escapeCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		c = strings.ReplaceAll(c, "|", `\|`)
		c = strings.Join(strings.Fields(c), " ")
		if c == "" {
			c = " "
		}
		out[i] = c
	}
	return out
}
