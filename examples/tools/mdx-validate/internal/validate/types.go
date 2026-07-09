// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package validate implements the validators that mdx-validate runs over
// Mintlify MDX files. Each validator is a small, independently testable
// unit; the runner composes them and aggregates findings.
package validate

import "fmt"

// Severity classifies a finding as a blocking error or a non-blocking warning.
//
// The zero value is SeverityUnspecified so a Finding constructed without an
// explicit severity does not silently default to Error or Warning.
type Severity int

const (
	// SeverityUnspecified is the zero value; never produced by a real validator.
	SeverityUnspecified Severity = iota
	// Warning is reported but does not affect exit status unless --strict is set.
	Warning
	// Error makes the validator exit non-zero.
	Error
)

// String returns the lowercase human label for a severity.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "unspecified"
	}
}

// Finding is a single problem reported by a validator.
type Finding struct {
	Path     string   // file path, relative to wherever the runner was invoked
	Line     int      // 1-based line number; 0 if not applicable
	Severity Severity
	Code     string   // short stable identifier, e.g. "frontmatter-missing-title"
	Message  string   // human-readable explanation
}

// Format returns a single-line representation of the finding.
//
//	docs-main/foo.mdx:0: error frontmatter-missing-title: ...
func (f Finding) Format() string {
	return fmt.Sprintf("%s:%d: %s %s: %s",
		f.Path, f.Line, f.Severity, f.Code, f.Message)
}

// Validator inspects a single MDX file's bytes and reports findings.
// The runner currently invokes validators sequentially per file; the
// interface itself is goroutine-friendly (validators should not retain
// state across calls), but no claim of cross-file parallelism is made
// today.
type Validator interface {
	// Name is a short identifier for diagnostics and tests.
	Name() string
	// Validate returns zero or more findings for the given file. p carries the
	// shared per-file parse (element scan and local-name set) so validators
	// that need it do not re-tokenize the same content; validators that don't
	// need it ignore p.
	Validate(path string, content []byte, p *parsed) []Finding
}
