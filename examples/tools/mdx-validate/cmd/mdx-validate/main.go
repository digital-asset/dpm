// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Command mdx-validate validates Mintlify MDX documentation files. It is
// the dpm component complement to rst-to-mdx: where the converter emits
// MDX, mdx-validate checks that the MDX in tree is valid before it ships.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"daml.com/x/dpm-components/mdx-validate/internal/validate"
)

const version = "0.5.0-dev"

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:]))
}

// run is the testable entry point. It returns the process exit code.
//
// Exit codes:
//
//	0 — clean run (no errors, or only warnings without --strict)
//	1 — blocking findings reported
//	2 — usage error or I/O failure
func run(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("mdx-validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr, fs) }

	strict := fs.Bool("strict", false, "promote warnings to errors")
	staged := fs.Bool("staged", false, "validate only files staged in the git index (pre-commit mode)")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, "mdx-validate", version)
		return 0
	}

	paths := fs.Args()
	if *staged && len(paths) > 0 {
		fmt.Fprintln(stderr, "mdx-validate: --staged cannot be combined with explicit paths")
		return 2
	}

	targets, err := resolveTargets(*staged, paths)
	if err != nil {
		fmt.Fprintf(stderr, "mdx-validate: %v\n", err)
		return 2
	}

	r := validate.Runner{Validators: validate.DefaultValidators()}
	findings, counts, err := r.RunPaths(targets)
	if err != nil {
		fmt.Fprintf(stderr, "mdx-validate: %v\n", err)
		return 2
	}

	// Message when nothing matched — covers both `--staged` with
	// no staged .mdx and explicit paths that resolved to non-mdx files.
	if counts.Files == 0 {
		fmt.Fprintln(stdout, "no .mdx files to validate")
		return 0
	}

	validate.FormatFindings(stdout, findings)
	fmt.Fprintf(stdout, "\n%d error(s), %d warning(s) across %d file(s)\n",
		counts.Errors, counts.Warnings, counts.Files)

	if counts.HasBlockingErrors(*strict) {
		return 1
	}
	return 0
}

// resolveTargets picks the set of paths to validate based on the flag
// choices: --staged → git's staged file list; explicit args → those paths
// as-given; neither → ./docs-main as the default.
//
// When the default ./docs-main is selected and that directory does not
// exist, returns an error so the user gets a clear hint rather than a
// confusing zero-files report from the runner.
func resolveTargets(staged bool, paths []string) ([]string, error) {
	switch {
	case staged:
		return stagedMDXFiles()
	case len(paths) > 0:
		return paths, nil
	default:
		const defaultDir = "./docs-main"
		if _, err := os.Stat(defaultDir); err != nil {
			return nil, fmt.Errorf("%s not found in cwd; run from the repo root or pass an explicit path", defaultDir)
		}
		return []string{defaultDir}, nil
	}
}

// stagedMDXFiles returns the .mdx files in git's staged index. Run from
// anywhere inside the repository.
//
// --diff-filter=ACMR includes Added, Copied, Modified, Renamed entries; a
// pure delete (D) is not validated because there's no content to check.
//
// git reports staged paths relative to the repository root, but the runner
// opens them relative to the current working directory. To keep --staged
// correct when invoked from a subdirectory (e.g. a hook that does not cd to
// the root), each path is joined to the repo root reported by git.
func stagedMDXFiles() ([]string, error) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("git repo root: %w", err)
	}
	rootDir := strings.TrimRight(string(root), "\n")

	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git staged file list: %w", err)
	}
	var mdx []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, ".mdx") {
			continue
		}
		mdx = append(mdx, filepath.Join(rootDir, line))
	}
	return mdx, nil
}

func usage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Usage: dpm mdx-validate [flags] [paths...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Validates Mintlify MDX documentation files. With no paths, validates ./docs-main.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --staged    validate only files staged in the git index (pre-commit mode)")
	fmt.Fprintln(w, "  --strict    promote warnings to errors")
	fmt.Fprintln(w, "  --version   print version and exit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  dpm mdx-validate                          # validate ./docs-main")
	fmt.Fprintln(w, "  dpm mdx-validate docs-main/foo.mdx        # validate one file")
	fmt.Fprintln(w, "  dpm mdx-validate --staged                 # validate staged .mdx files (pre-commit)")
	fmt.Fprintln(w, "  dpm mdx-validate --strict ./docs-main     # warnings become errors")
}
