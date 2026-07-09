// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// New validators are added here as they land.
func DefaultValidators() []Validator {
	return []Validator{
		FrontmatterValidator{},
		ComponentValidator{},
		StructureValidator{},
		ImageValidator{},
	}
}

// Runner orchestrates a set of validators across one or more files.
type Runner struct {
	Validators []Validator
}

// Counts holds the per-severity tallies of a run.
type Counts struct {
	Errors   int
	Warnings int
	Files    int
}

// HasBlockingErrors reports whether the run produced any Error findings,
// or whether --strict promotes warnings to errors.
func (c Counts) HasBlockingErrors(strict bool) bool {
	if c.Errors > 0 {
		return true
	}
	return strict && c.Warnings > 0
}

// RunFile reads a single file from disk and runs every validator over it.
// Returns the findings produced by the validators.
func (r Runner) RunFile(path string) ([]Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return r.RunBytes(path, content), nil
}

// RunBytes runs every validator over an in-memory file. Useful for tests.
func (r Runner) RunBytes(path string, content []byte) []Finding {
	p := newParsed(content)
	var out []Finding
	for _, v := range r.Validators {
		out = append(out, v.Validate(path, content, p)...)
	}
	return out
}

// RunPaths walks the given paths (files or directories), validates each
// .mdx file, and returns aggregated findings plus per-run counts.
//
// Per-file read failures (permission denied, transient FS error) become
// "io-error" findings of severity Error rather than aborting the run, so
// one bad file does not discard findings already collected from siblings.
func (r Runner) RunPaths(paths []string) ([]Finding, Counts, error) {
	files, err := expandToMDXFiles(paths)
	if err != nil {
		return nil, Counts{}, err
	}

	var all []Finding
	for _, f := range files {
		findings, err := r.RunFile(f)
		if err != nil {
			all = append(all, Finding{
				Path:     f,
				Line:     0,
				Severity: Error,
				Code:     "io-error",
				Message:  err.Error(),
			})
			continue
		}
		all = append(all, findings...)
	}

	counts := Counts{Files: len(files)}
	for _, f := range all {
		if f.Severity == Error {
			counts.Errors++
		} else if f.Severity == Warning {
			counts.Warnings++
		}
	}
	return all, counts, nil
}

// FormatFindings writes a stable, grouped representation of findings to w.
// Order: path, then severity (errors before warnings), then line, then code.
func FormatFindings(w io.Writer, findings []Finding) {
	sorted := make([]Finding, len(findings))
	copy(sorted, findings)
	sort.SliceStable(sorted, func(i, j int) bool {
		switch {
		case sorted[i].Path != sorted[j].Path:
			return sorted[i].Path < sorted[j].Path
		case sorted[i].Severity != sorted[j].Severity:
			// Lower numeric value sorts first; Error > Warning numerically,
			// so flip with > so Error rows come first within a path.
			return sorted[i].Severity > sorted[j].Severity
		case sorted[i].Line != sorted[j].Line:
			return sorted[i].Line < sorted[j].Line
		default:
			return sorted[i].Code < sorted[j].Code
		}
	})
	for _, f := range sorted {
		fmt.Fprintln(w, f.Format())
	}
}

// expandToMDXFiles resolves the given paths to a sorted, de-duplicated
// list of .mdx files to validate. Directories are walked recursively;
// non-.mdx files and snippet files (see SkipPath) are excluded.
func expandToMDXFiles(paths []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			// os.Stat follows symlinks, so a symlink-to-directory reaches here,
			// but filepath.WalkDir lstats its root and would treat that symlink
			// as a single non-directory entry, walking nothing. Resolve the root
			// symlink so the real directory is walked. Only the explicit symlink
			// target is resolved, so non-symlink paths keep their as-given prefix
			// in reported findings.
			walkRoot := p
			if li, lerr := os.Lstat(p); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
				if resolved, rerr := filepath.EvalSymlinks(p); rerr == nil {
					walkRoot = resolved
				}
			}
			err := filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".mdx") {
					return nil
				}
				if SkipPath(path) {
					return nil
				}
				if _, dup := seen[path]; dup {
					return nil
				}
				seen[path] = struct{}{}
				out = append(out, path)
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			if !strings.HasSuffix(p, ".mdx") {
				continue
			}
			if SkipPath(p) {
				continue
			}
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// SkipPath reports whether a file path should be excluded from validation.
//
// Skips any file under a `snippets/` directory because Mintlify
// snippets are reusable content fragments that don't have frontmatter by
// design (they're meant to be embedded into pages with `<Snippet />`).
// Validating them as standalone pages produces noise without value.
func SkipPath(path string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		if seg == "snippets" {
			return true
		}
	}
	return false
}
