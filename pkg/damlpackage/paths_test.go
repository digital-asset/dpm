package damlpackage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRejectSymlinkOutsideRoot_regularFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "loyalty.dar")
	require.NoError(t, os.WriteFile(path, []byte("dar"), 0o644))

	require.NoError(t, RejectSymlinkOutsideRoot(root, path))
}

func TestRejectSymlinkOutsideRoot_symlinkInsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "real.dar")
	require.NoError(t, os.WriteFile(target, []byte("dar"), 0o644))

	link := filepath.Join(root, "loyalty.dar")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, RejectSymlinkOutsideRoot(root, link))
}

func TestRejectSymlinkOutsideRoot_symlinkOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.dar")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))

	link := filepath.Join(root, "loyalty.dar")
	require.NoError(t, os.Symlink(outsideFile, link))

	err := RejectSymlinkOutsideRoot(root, link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink points outside the repository")
}

func TestRejectSymlinkOutsideRoot_intermediateSymlinkOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.dar")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))

	linkDir := filepath.Join(root, "dist")
	require.NoError(t, os.Symlink(outside, linkDir))

	err := RejectSymlinkOutsideRoot(root, filepath.Join(linkDir, "secret.dar"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink points outside the repository")
}
