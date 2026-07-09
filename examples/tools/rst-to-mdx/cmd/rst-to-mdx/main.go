// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Command rst-to-mdx converts reStructuredText files from docs-website/
// into Mintlify-compatible MDX files for cf-docs/docs-main/.
//
// Conversion logic lives in the internal convert/, labelindex/, and
// pathmap/ packages. This file is the Cobra CLI wrapper: it parses
// flags, opens files, builds the cross-reference index once, and calls
// Convert for each input.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"daml.com/x/dpm-components/rst-to-mdx/internal/convert"
	"daml.com/x/dpm-components/rst-to-mdx/internal/labelindex"
	"daml.com/x/dpm-components/rst-to-mdx/internal/navindex"
	"daml.com/x/dpm-components/rst-to-mdx/internal/pathmap"
)

func mustRegex(p string) *regexp.Regexp { return regexp.MustCompile(p) }

const imagesSubdir = "images/docs_website"

var version = "0.0.1-dev"

type runOptions struct {
	title          string
	description    string
	sourceLabel    string
	batch          bool
	inputDir       string
	outputDir      string
	docsRoot       string
	targetRoot     string
	docsJSON       string
	copyImages     bool
	strict         bool
	dryRun         bool
	verbose        bool
	auditCoverage  bool
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "rst-to-mdx <input.rst> [output.mdx]",
		Short: "Convert reStructuredText to Mintlify MDX",
		Long: `Convert reStructuredText files into Mintlify-compatible MDX. Handles
headings, admonitions, code blocks, cross-references, images, tables,
literalinclude, and frontmatter + provenance markers.

The input RST file may live anywhere on disk. Cross-reference resolution
(:ref:, :doc:, :externalref:) is the only feature that needs an RST
docs tree to read against — pass --docs-root or place the input under a
docs-website/ subtree (auto-detected). Without a docs-root, cross-refs
emit #TODO-resolve-* markers and the rest of the conversion proceeds.

Use --batch to walk a directory tree.`,
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.auditCoverage {
				return runAudit(cmd.OutOrStdout(), opts)
			}
			if opts.batch {
				if opts.inputDir == "" {
					return fmt.Errorf("--batch requires --input-dir")
				}
				if opts.outputDir == "" {
					return fmt.Errorf("--batch requires --output-dir")
				}
				return runBatch(cmd.OutOrStdout(), opts)
			}
			if len(args) == 0 {
				return fmt.Errorf("input file required (or use --batch or --audit-coverage)")
			}
			in := args[0]
			var out string
			if len(args) == 2 {
				out = args[1]
			} else {
				out = deriveOutputPath(in)
			}
			return runSingle(cmd.OutOrStdout(), in, out, opts)
		},
	}

	cmd.Flags().StringVar(&opts.title, "title", "", "override auto-detected page title")
	cmd.Flags().StringVar(&opts.description, "description", "", "set frontmatter description")
	cmd.Flags().StringVar(&opts.sourceLabel, "source-label", "", "provenance source label (auto from path)")
	cmd.Flags().BoolVar(&opts.batch, "batch", false, "convert all .rst files in --input-dir")
	cmd.Flags().StringVar(&opts.inputDir, "input-dir", "", "input directory for --batch")
	cmd.Flags().StringVar(&opts.outputDir, "output-dir", "./converted", "output directory for --batch")
	cmd.Flags().StringVar(&opts.docsRoot, "docs-root", "", "root of an RST docs tree for cross-ref resolution (auto-detects docs-website/ when input lives in one)")
	cmd.Flags().StringVar(&opts.targetRoot, "target-root", "./docs-main", "target docs-main/ root for path derivation")
	cmd.Flags().StringVar(&opts.docsJSON, "docs-json", "", "path to Mintlify docs.json for nav-aware link resolution (auto-detects <target-root>/docs.json)")
	cmd.Flags().BoolVar(&opts.copyImages, "copy-images", false, "copy referenced images into target-root/images/docs_website/")
	cmd.Flags().BoolVar(&opts.strict, "strict", false, "fail on unresolved :ref: or missing literalinclude")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show what would be written without touching the filesystem")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "show detailed conversion progress")
	cmd.Flags().BoolVar(&opts.auditCoverage, "audit-coverage", false, "report which RST files under --docs-root have no matching page in --target-root/docs.json")
	cmd.Flags().Bool("version", false, "print version and exit")

	cmd.PreRun = func(c *cobra.Command, _ []string) {
		if v, _ := c.Flags().GetBool("version"); v {
			fmt.Fprintln(c.OutOrStdout(), "rst-to-mdx", version)
			os.Exit(0)
		}
	}

	return cmd
}

// runContext bundles the indexes and config that are constant across
// every file converted in a single invocation, so batch mode can build
// them once instead of per-file.
type runContext struct {
	labels   *labelindex.Index
	nav      *navindex.Index
	docsRoot string
}

func newRunContext(w io.Writer, anchorPath string, opts *runOptions) (*runContext, error) {
	labels, err := loadLabelIndex(w, anchorPath, opts)
	if err != nil {
		return nil, err
	}
	docsRoot := opts.docsRoot
	if docsRoot == "" && anchorPath != "" {
		docsRoot = autoDetectDocsRoot(anchorPath)
	}
	nav, err := loadNavIndex(w, opts)
	if err != nil {
		return nil, err
	}
	return &runContext{labels: labels, nav: nav, docsRoot: docsRoot}, nil
}

// fileResult is the per-file outcome of a convert + write step. Batch
// mode aggregates these into a summary.
type fileResult struct {
	inputPath  string
	outputPath string
	bytes      int
	images     int
	unknown    int
}

// runConvertOne converts one RST file using a pre-built run context.
// Caller is responsible for picking the output path. Returns the
// per-file stats so batch mode can aggregate them.
func runConvertOne(w io.Writer, ctx *runContext, inputPath, outputPath string, opts *runOptions) (fileResult, error) {
	r := fileResult{inputPath: inputPath, outputPath: outputPath}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return r, fmt.Errorf("read input: %w", err)
	}

	co := convert.Options{
		Title:       opts.title,
		Description: opts.description,
		SourceLabel: firstNonEmpty(opts.sourceLabel, normalizeSourceLabel(inputPath)),
		SourcePath:  inputPath,
		LabelIndex:  ctx.labels,
		NavIndex:    ctx.nav,
		DocsRoot:    ctx.docsRoot,
		Strict:      opts.strict,
	}
	res, err := convert.Convert(data, co)
	if err != nil {
		return r, fmt.Errorf("convert: %w", err)
	}
	r.bytes = len(res.Body)
	r.images = len(res.Images)

	if opts.dryRun {
		fmt.Fprintf(w, "[dry-run] would write %d bytes to %s\n", r.bytes, outputPath)
		if opts.copyImages && r.images > 0 {
			fmt.Fprintf(w, "[dry-run] would copy %d image asset(s)\n", r.images)
		}
		return r, nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return r, err
	}
	if err := os.WriteFile(outputPath, res.Body, 0o644); err != nil {
		return r, fmt.Errorf("write output: %w", err)
	}
	if opts.verbose {
		fmt.Fprintf(w, "wrote %s (%d bytes)\n", outputPath, r.bytes)
	}

	if opts.copyImages {
		if err := copyImageAssets(w, res.Images, opts); err != nil {
			return r, err
		}
	}

	if ctx.nav != nil {
		unknown, err := reportUnknownLinks(w, res.Body, ctx.nav, opts)
		if err != nil {
			return r, err
		}
		r.unknown = unknown
	}
	return r, nil
}

func runSingle(w io.Writer, inputPath, outputPath string, opts *runOptions) error {
	ctx, err := newRunContext(w, inputPath, opts)
	if err != nil {
		return err
	}
	_, err = runConvertOne(w, ctx, inputPath, outputPath, opts)
	return err
}

// reportUnknownLinks walks the converted MDX for absolute internal
// link targets (`(/path/to/page)`) and warns when the target page
// isn't registered in docs.json. Returns the number of unique unknown
// targets so the caller can aggregate counts across a batch run. In
// --strict mode any unknown link is an error.
//
// Asset paths (under /images/) and file-extension paths are filtered
// out — those aren't navigation pages, so docs.json is the wrong
// authority for them.
func reportUnknownLinks(w io.Writer, body []byte, nav *navindex.Index, opts *runOptions) (int, error) {
	matches := reInternalLinkTarget.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return 0, nil
	}
	seen := make(map[string]struct{})
	var unknown []string
	for _, m := range matches {
		page := string(m[1])
		if _, ok := seen[page]; ok {
			continue
		}
		seen[page] = struct{}{}
		if isAssetPath(page) {
			continue
		}
		if !nav.HasPage(page) {
			unknown = append(unknown, page)
		}
	}
	if len(unknown) == 0 {
		if opts.verbose {
			fmt.Fprintf(w, "links: %d internal targets, all registered in docs.json\n", len(seen))
		}
		return 0, nil
	}
	if opts.strict {
		return len(unknown), fmt.Errorf("strict: %d link target(s) not in docs.json: %v", len(unknown), unknown)
	}
	if opts.verbose {
		fmt.Fprintf(w, "warn: %d internal link target(s) not registered in docs.json:\n", len(unknown))
		for _, p := range unknown {
			fmt.Fprintf(w, "  - /%s\n", p)
		}
	}
	return len(unknown), nil
}

// reInternalLinkTarget pulls the page slug out of `(/path#anchor)`
// link targets in the emitted MDX. Captures the slug part before any
// `#fragment`. Mintlify serves the docs-main/ directory as site root
// so internal links are root-relative without a docs-main/ prefix.
var reInternalLinkTarget = mustRegex(`\(/([A-Za-z0-9_\-/.]+?)(?:#[^)]*)?\)`)

// isAssetPath returns true for paths that aren't navigation pages —
// images, public assets, and anything with a file extension. docs.json
// only registers MDX pages, so we skip these to avoid false-positive
// "unknown link" warnings.
func isAssetPath(p string) bool {
	if strings.HasPrefix(p, "images/") {
		return true
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".svg", ".gif",
		".webp", ".ico", ".pdf", ".css", ".js", ".json", ".yaml", ".yml"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// copyImageAssets copies every image referenced by the source RST into
// the target docs tree under <target-root>/images/docs_website/. A
// missing source file is reported (and counted) but doesn't abort the
// run unless --strict is set.
func copyImageAssets(w io.Writer, refs []convert.ImageRef, opts *runOptions) error {
	if len(refs) == 0 {
		return nil
	}
	imagesRoot := filepath.Join(opts.targetRoot, imagesSubdir)
	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir images dir: %w", err)
	}

	copied, missing, collisions := 0, 0, 0
	for _, ref := range refs {
		if ref.SourceAbs == "" {
			missing++
			if opts.verbose {
				fmt.Fprintf(w, "skip image %q: no source path resolved\n", ref.SourceRel)
			}
			continue
		}
		dst := filepath.Join(opts.targetRoot, ref.TargetRel)
		// Detect basename collisions with already-present, differently
		// sourced files. A collision isn't fatal — the second copy
		// just overwrites — but we count it for the operator.
		if existing, err := os.Stat(dst); err == nil {
			if same, err := sameFileContent(existing, ref.SourceAbs); err == nil && !same {
				collisions++
				if opts.verbose {
					fmt.Fprintf(w, "warn: %s already exists with different content (will overwrite)\n", dst)
				}
			}
		}
		if err := copyFile(ref.SourceAbs, dst); err != nil {
			missing++
			if opts.strict {
				return fmt.Errorf("copy %s: %w", ref.SourceAbs, err)
			}
			if opts.verbose {
				fmt.Fprintf(w, "warn: copy %s -> %s: %v\n", ref.SourceAbs, dst, err)
			}
			continue
		}
		copied++
	}
	if opts.verbose || copied > 0 || missing > 0 {
		fmt.Fprintf(w, "images: %d copied, %d missing, %d collisions\n",
			copied, missing, collisions)
	}
	return nil
}

// copyFile reads src and writes its bytes to dst, creating any missing
// parent directories. Cheap byte-copy; image assets are small.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// sameFileContent returns true when the existing target and a candidate
// source have the same size — a cheap proxy for "this is the same
// asset, no collision". A full byte-compare would be more accurate but
// we don't need that for the warning.
func sameFileContent(existing os.FileInfo, srcPath string) (bool, error) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return false, err
	}
	return existing.Size() == srcInfo.Size(), nil
}

// runBatch walks --input-dir for content RST files, converts each
// using a shared run context, and writes the results under
// --output-dir at paths derived by `pathmap.Derive`. The same content
// filters as `--audit-coverage` apply: scaffolding (`index.rst`,
// `conf.py`, `*.inc`), dotfile dirs, build trees, and files outside
// `docs/replicated/` are skipped.
//
// The label index, nav index, and docs-root are loaded ONCE and shared
// across every conversion in the run.
func runBatch(w io.Writer, opts *runOptions) error {
	inputDir := opts.inputDir
	outputDir := opts.outputDir
	if outputDir == "" {
		return fmt.Errorf("--batch requires --output-dir")
	}

	// Anchor the run context on the input directory so docs-root
	// auto-detection still works when --input-dir lives under a
	// docs-website/ tree.
	ctx, err := newRunContext(w, inputDir, opts)
	if err != nil {
		return err
	}

	type batchEntry struct {
		input  string
		output string
	}
	var queue []batchEntry
	var skipped, unsupported int

	walkErr := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "_build" ||
				base == "target" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".rst") {
			return nil
		}
		base := filepath.Base(path)
		if base == "index.rst" || base == "conf.py" ||
			strings.HasSuffix(base, ".inc") || strings.HasSuffix(base, ".inc.rst") {
			skipped++
			return nil
		}
		// Stay inside the replicated/ corpus — the rest of
		// docs-website/ is build scaffolding or vendored content.
		if !strings.Contains(path, "docs/replicated/") {
			skipped++
			return nil
		}
		derived, ok := pathmap.Derive(path)
		if !ok {
			unsupported++
			if opts.verbose {
				fmt.Fprintf(w, "no pathmap rule for %s — skipping\n", path)
			}
			return nil
		}
		out := filepath.Join(outputDir, string(derived)+".mdx")
		queue = append(queue, batchEntry{input: path, output: out})
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if opts.verbose {
		fmt.Fprintf(w, "batch: %d files queued, %d scaffolding skipped, %d without pathmap rule\n",
			len(queue), skipped, unsupported)
	}

	var (
		converted    int
		failed       int
		totalImages  int
		totalUnknown int
		failures     []string
	)
	// Track output paths so we can report when multiple inputs map to
	// the same target. With multiple RST versions side-by-side
	// (canton/3.4, canton/3.5, canton/3.6), pathmap.Derive collapses
	// them onto one MDX slug; the lexically-later version wins.
	writes := make(map[string][]string)
	for _, e := range queue {
		res, err := runConvertOne(w, ctx, e.input, e.output, opts)
		if err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", e.input, err))
			if opts.strict {
				return fmt.Errorf("strict: %w", err)
			}
			continue
		}
		converted++
		totalImages += res.images
		totalUnknown += res.unknown
		writes[e.output] = append(writes[e.output], e.input)
	}

	collisions := 0
	for _, sources := range writes {
		if len(sources) > 1 {
			collisions++
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "batch summary:")
	fmt.Fprintf(w, "  input dir:       %s\n", inputDir)
	fmt.Fprintf(w, "  output dir:      %s\n", outputDir)
	fmt.Fprintf(w, "  converted:       %d (%d unique outputs)\n", converted, len(writes))
	fmt.Fprintf(w, "  failed:          %d\n", failed)
	fmt.Fprintf(w, "  scaffolding:     %d skipped\n", skipped)
	fmt.Fprintf(w, "  no pathmap rule: %d skipped\n", unsupported)
	fmt.Fprintf(w, "  output collisions: %d (multiple inputs → same MDX path; latest wins)\n", collisions)
	fmt.Fprintf(w, "  image refs seen: %d\n", totalImages)
	fmt.Fprintf(w, "  unresolved nav links: %d\n", totalUnknown)

	if collisions > 0 && opts.verbose {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "collisions (output path → contributing sources):")
		for out, sources := range writes {
			if len(sources) <= 1 {
				continue
			}
			fmt.Fprintf(w, "  %s\n", out)
			for _, src := range sources {
				fmt.Fprintf(w, "      ← %s\n", src)
			}
		}
	}

	if len(failures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "failures:")
		for _, f := range failures {
			fmt.Fprintf(w, "  - %s\n", f)
		}
	}
	if failed > 0 {
		return fmt.Errorf("batch finished with %d failures", failed)
	}
	return nil
}

// runAudit walks every .rst file under --docs-root and reports which
// have no matching page in the live docs.json navigation, producing
// an inventory of unmigrated files. It uses the same pathmap +
// NavIndex resolution that the converter applies during cross-ref
// rewriting, so the answer reflects the tool's view of "where would
// this file land if migrated today?"
//
// Buckets:
//   - direct  — pathmap-derived path is registered in docs.json
//   - matched — NavIndex.BestMatch finds a different page that fits
//   - missing — no docs.json hit at all (candidate for migration)
//   - skipped — index/conf/.inc files we never migrate
func runAudit(w io.Writer, opts *runOptions) error {
	root := opts.docsRoot
	if root == "" {
		return fmt.Errorf("--audit-coverage requires --docs-root")
	}
	nav, err := loadNavIndex(w, opts)
	if err != nil {
		return err
	}
	if nav == nil {
		return fmt.Errorf("--audit-coverage requires a docs.json (use --target-root or --docs-json)")
	}

	var direct, matched, missing, skipped []string
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip dotfile dirs (e.g. .venv) and Sphinx build trees.
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "_build" ||
				base == "target" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".rst") {
			return nil
		}
		base := filepath.Base(path)
		// Skip RST files that are scaffolding rather than content.
		if base == "index.rst" || base == "conf.py" ||
			strings.HasSuffix(base, ".inc") || strings.HasSuffix(base, ".inc.rst") {
			skipped = append(skipped, path)
			return nil
		}
		// Only audit files under docs/replicated/ — the rest of the
		// docs-website/ tree is build scaffolding, vendored Sphinx
		// extensions, or alternative manual sources we don't migrate.
		if !strings.Contains(path, "docs/replicated/") {
			skipped = append(skipped, path)
			return nil
		}
		rel := strings.TrimPrefix(path, root)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))

		derived, ok := pathmap.Derive(path)
		if ok && nav.HasPage(string(derived)) {
			direct = append(direct, rel)
			return nil
		}
		if best := nav.BestMatch(stripDocsWebsitePrefixForAudit(path)); best != "" {
			matched = append(matched, fmt.Sprintf("%s  →  %s", rel, best))
			return nil
		}
		missing = append(missing, rel)
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	total := len(direct) + len(matched) + len(missing)
	fmt.Fprintf(w, "audit summary (under %s):\n", root)
	fmt.Fprintf(w, "  total content RST files:  %d\n", total)
	fmt.Fprintf(w, "  direct path match:        %d\n", len(direct))
	fmt.Fprintf(w, "  matched via NavIndex:     %d\n", len(matched))
	fmt.Fprintf(w, "  missing from docs.json:   %d\n", len(missing))
	fmt.Fprintf(w, "  scaffolding skipped:      %d\n", len(skipped))
	fmt.Fprintln(w)

	if len(matched) > 0 && opts.verbose {
		fmt.Fprintln(w, "matched (RST → docs.json page):")
		for _, m := range matched {
			fmt.Fprintf(w, "  %s\n", m)
		}
		fmt.Fprintln(w)
	}

	if len(missing) > 0 {
		fmt.Fprintln(w, "missing — RST files with no docs.json target:")
		for _, m := range missing {
			fmt.Fprintf(w, "  %s\n", m)
		}
	}
	return nil
}

// stripDocsWebsitePrefixForAudit mirrors the CLI version of the prefix
// stripper used by convert/links.go so the audit output applies the
// same NavIndex matching logic as cross-ref resolution.
func stripDocsWebsitePrefixForAudit(p string) string {
	marker := "docs-website/docs/replicated/"
	i := strings.LastIndex(p, marker)
	if i < 0 {
		return p
	}
	return p[i+len(marker):]
}

// loadLabelIndex resolves docs-root (explicit flag, auto-detected, or
// nothing) and builds a label index against it. Returns nil, nil when no
// docs-root is available — conversion still works, cross-refs just fall
// back to TODO markers.
func loadLabelIndex(w io.Writer, inputPath string, opts *runOptions) (*labelindex.Index, error) {
	root := opts.docsRoot
	if root == "" {
		root = autoDetectDocsRoot(inputPath)
	}
	if root == "" {
		if opts.verbose {
			fmt.Fprintln(w, "no docs-website/ root detected; cross-refs will emit TODO markers")
		}
		return nil, nil
	}
	if opts.verbose {
		fmt.Fprintf(w, "building label index under %s…\n", root)
	}
	idx, err := labelindex.Build(root)
	if err != nil {
		return nil, fmt.Errorf("build label index: %w", err)
	}
	if opts.verbose {
		fmt.Fprintf(w, "indexed %d labels (%d definitions across files)\n",
			idx.Size(), idx.TotalDefinitions())
	}
	return idx, nil
}

// loadNavIndex resolves the docs.json path (explicit flag, or
// `<target-root>/docs.json` when the file exists) and parses it.
// Returns nil, nil when no docs.json is locatable — link resolution
// then falls back to the algorithmic pathmap.
func loadNavIndex(w io.Writer, opts *runOptions) (*navindex.Index, error) {
	jsonPath := opts.docsJSON
	if jsonPath == "" && opts.targetRoot != "" {
		candidate := filepath.Join(opts.targetRoot, "docs.json")
		if _, err := os.Stat(candidate); err == nil {
			jsonPath = candidate
		}
	}
	if jsonPath == "" {
		if opts.verbose {
			fmt.Fprintln(w, "no docs.json found; cross-refs will use algorithmic pathmap only")
		}
		return nil, nil
	}
	if opts.verbose {
		fmt.Fprintf(w, "reading nav index from %s…\n", jsonPath)
	}
	idx, err := navindex.Build(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("build nav index: %w", err)
	}
	if opts.verbose {
		fmt.Fprintf(w, "indexed %d pages from docs.json\n", idx.Size())
	}
	return idx, nil
}

// autoDetectDocsRoot walks up from the input file looking for a
// directory named `docs-website`. Returns it when found, or an empty
// string if the file isn't inside one.
func autoDetectDocsRoot(inputPath string) string {
	abs, err := filepath.Abs(inputPath)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	for dir != "/" && dir != "." {
		if filepath.Base(dir) == "docs-website" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func deriveOutputPath(inputPath string) string {
	base := filepath.Base(inputPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", "-")
	base = strings.ToLower(base)
	return filepath.Join("./converted", base+".mdx")
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// normalizeSourceLabel produces the `docs-website:<relpath>` form
// that matches the provenance convention in the migration guide.
// If the path doesn't live under a `docs-website/` directory, the
// original path is returned unchanged.
func normalizeSourceLabel(p string) string {
	marker := "docs-website/"
	idx := strings.LastIndex(p, marker)
	if idx < 0 {
		return p
	}
	rel := p[idx+len(marker):]
	return "docs-website:" + rel
}
