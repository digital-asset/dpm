package utils

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// windowsAccessDenied is syscall.ERROR_ACCESS_DENIED. That name is declared
// only when building for Windows.
const windowsAccessDenied syscall.Errno = 5

// destinationLocks serializes the final replace for one path. Windows
// MoveFileEx returns "Access is denied" when two renames replace the same
// destination at once.
var destinationLocks sync.Map // map[string]*sync.Mutex

// AtomicWriteFile writes r to dst via a temporary file in the same directory.
func AtomicWriteFile(dst string, r io.Reader) error {
	if err := EnsureDirs(filepath.Dir(dst)); err != nil {
		return err
	}

	out, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := out.Name()

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
	if err := moveIntoPlace(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// moveIntoPlace publishes `tmp` as `dst`. Callers must not remove `tmp` until this
// returns: Windows may retry the replace while `tmp` is still in place.
//
// Retries happen at most 8 times, with a delay of 5ms, 10ms, 15ms, 20ms, 25ms, 30ms, 35ms, 40ms.
func moveIntoPlace(tmp, dst string) error {
	unlock := lockDestination(dst)
	defer unlock()

	const attempts = 8
	var err error
	for attempt := range attempts {
		err = os.Rename(tmp, dst)
		if err == nil || !renameShouldRetry(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return err
}

func renameShouldRetry(err error) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == windowsAccessDenied
}

func lockDestination(dst string) func() {
	key := filepath.Clean(dst)
	mu, _ := destinationLocks.LoadOrStore(key, new(sync.Mutex))
	mu.(*sync.Mutex).Lock()
	return func() { mu.(*sync.Mutex).Unlock() }
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
