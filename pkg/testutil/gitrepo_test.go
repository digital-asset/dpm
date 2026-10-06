package testutil

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileCloneURLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cloneURL := FileCloneURL(dir)

	u, err := url.Parse(cloneURL)
	require.NoError(t, err)
	assert.Equal(t, "file", u.Scheme)
	assert.Empty(t, u.Host, "a drive letter must stay in the path, not the host")
	assert.True(t, len(u.Path) > 0 && u.Path[0] == '/')
	assert.Equal(t, dir, DirFromFileCloneURL(cloneURL))
}

func TestFileCloneURLKeepsWindowsDriveInPath(t *testing.T) {
	// Join("C:", "Users") is the drive-relative path "C:Users" on Windows, not "C:\Users".
	cloneURL := FileCloneURL(filepath.FromSlash("C:/Users/repo"))
	assert.Equal(t, "file:///C:/Users/repo", cloneURL)

	u, err := url.Parse(cloneURL)
	require.NoError(t, err)
	assert.Empty(t, u.Host)
	assert.Equal(t, "/C:/Users/repo", u.Path)
	assert.Equal(t, filepath.FromSlash("C:/Users/repo"), DirFromFileCloneURL(cloneURL))
}
