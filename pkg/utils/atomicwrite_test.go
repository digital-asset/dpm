package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFile_concurrentWritersDoNotCorrupt(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.dar")

	const n = 8
	payloads := make([][]byte, n)
	for i := range payloads {
		payloads[i] = bytes.Repeat([]byte{byte(i + 1)}, 64*1024)
	}

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range payloads {
		wg.Add(1)
		go func(payload []byte) {
			defer wg.Done()
			errs <- AtomicWriteFile(dst, bytes.NewReader(payload))
		}(payloads[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	matched := false
	for _, payload := range payloads {
		if bytes.Equal(got, payload) {
			matched = true
			break
		}
	}
	require.True(t, matched, "published file must be one complete writer payload")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// A file's mode is a deliberate invariant in dpm: it travels as the OCI
// file-mode annotation on push and is applied again on pull (pkg/utils/fileinfo).
// An artifact that arrives by another route should land with the mode it would
// have had coming from a registry. os.CreateTemp always creates 0600 and ignores
// the umask, so without an explicit chmod the rename publishes every destination
// 0600.
func TestAtomicWriteFile_newDestinationIsNotOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "new.dar")

	require.NoError(t, AtomicWriteFile(dst, bytes.NewReader([]byte("payload"))))

	fi, err := os.Stat(dst)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0644), fi.Mode().Perm(),
		"a new destination must be readable, not 0600 from the temporary file")
}

// Overwriting must not narrow a mode the caller chose. A cache populated once
// and then refreshed would otherwise become unreadable to everyone but the user
// who happened to refresh it.
func TestAtomicWriteFile_keepsTheExistingDestinationMode(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "existing.dar")
	// 0640 deliberately, not 0644: a hardcoded default would satisfy 0644 and
	// this test would then pass without reading the destination at all.
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0640))
	require.NoError(t, os.Chmod(dst, 0640))

	require.NoError(t, AtomicWriteFile(dst, bytes.NewReader([]byte("new"))))

	fi, err := os.Stat(dst)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0640), fi.Mode().Perm(),
		"an existing destination must keep its mode")
}

// AtomicCopyFile is a copy, so the destination should be what the source is.
// An executable copied this way must stay executable.
func TestAtomicCopyFile_preservesTheSourceMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tool")
	require.NoError(t, os.WriteFile(src, []byte("#!/bin/sh\nexit 0\n"), 0755))
	require.NoError(t, os.Chmod(src, 0755))
	dst := filepath.Join(dir, "tool-copy")

	require.NoError(t, AtomicCopyFile(src, dst))

	fi, err := os.Stat(dst)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), fi.Mode().Perm(),
		"a copy must carry the source's mode")
}
