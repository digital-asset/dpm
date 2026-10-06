package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// FileCloneURL returns a file:// clone URL for dir.
// The host is empty, so a Windows drive letter stays in the path
// (file:///C:/repo rather than file://C:/repo, which parsers treat as host "C").
func FileCloneURL(dir string) string {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// DirFromFileCloneURL returns the local directory for a URL from FileCloneURL.
func DirFromFileCloneURL(cloneURL string) string {
	p := strings.TrimPrefix(cloneURL, "file://")
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// InitGitRepo creates a local git repository with one commit containing darRelPath.
// Returns a file:// clone URL (requires DPM_TEST_ALLOW_FILE_GIT=true).
func InitGitRepo(t *testing.T, darRelPath string, darContents []byte) string {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	darAbs := filepath.Join(dir, darRelPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(darAbs), 0o755))
	require.NoError(t, os.WriteFile(darAbs, darContents, 0o644))

	_, err = w.Add(darRelPath)
	require.NoError(t, err)

	commit, err := w.Commit("add dar", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test"},
	})
	require.NoError(t, err)

	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName("main"),
		commit,
	)))

	return FileCloneURL(dir)
}
