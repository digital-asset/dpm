// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolvePath
func ResolvePath(basePath, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(basePath, p))
}

func DirExists(path string) (bool, error) {
	s, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return s.IsDir(), nil
}

func EnsureDirs(dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, os.ModePerm); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func CopyFile(src, dst string) error {
	if err := EnsureDirs(filepath.Dir(dst)); err != nil {
		return err
	}
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return destinationFile.Sync()
}

// MkdirTemp is like os.MkdirTemp but returns a cleanup function for deleting the created dir
func MkdirTemp(dir, pattern string) (string, func() error, error) {
	d, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", nil, err
	}
	fn := func() error {
		return os.RemoveAll(d)
	}
	return d, fn, err
}

var unsafeSegmentChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeSegment(s string) string {
	out := unsafeSegmentChars.ReplaceAllString(s, "_")

	// avoid Windows path normalization quirks: trailing dots/spaces in a path
	// segment may be rejected or treated the same as if they were not present
	out = strings.TrimRight(out, " .")

	if out == "" {
		return "_"
	}

	return out
}

// UrlToFilePath converts given url (without the scheme) to string usable as filepath.
// It escapes unsafe chars like ":".
// example:
//
//	foo.com:8080/a-b/b:c --> foo.com_8080/a-b/c_d
func UrlToFilePath(u string) string {
	parts := strings.Split(u, "/")

	for i, part := range parts {
		parts[i] = safeSegment(part)
	}

	return strings.Join(parts, "/")
}
