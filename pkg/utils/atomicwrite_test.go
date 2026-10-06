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
