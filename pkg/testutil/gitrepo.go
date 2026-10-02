package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

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

	return "file://" + filepath.ToSlash(dir)
}
