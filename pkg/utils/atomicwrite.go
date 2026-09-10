package utils

import (
	"io"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes r to dst via a temporary file in the same directory.
func AtomicWriteFile(dst string, r io.Reader) error {
	if err := EnsureDirs(filepath.Dir(dst)); err != nil {
		return err
	}

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, r)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if syncErr != nil {
		_ = os.Remove(tmp)
		return syncErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// AtomicCopyFile copies src to dst via a temporary file in the same directory.
func AtomicCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return AtomicWriteFile(dst, in)
}
