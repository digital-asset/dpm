package assistantconfig

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachePathForGitDependency_rejectsTraversal(t *testing.T) {
	t.Parallel()

	config := &Config{CachePath: t.TempDir()}
	cloneURL, err := url.Parse("https://github.com/org/repo.git")
	require.NoError(t, err)

	_, err = config.CachePathForGitDependency(cloneURL, "foo/../../escape.dar", strings.Repeat("a", 40))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dar path")

	_, err = config.CachePathForGitDependency(cloneURL, "foo.dar", "../../../evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ref")
}

func TestGitRepoSegments_rejectsTraversal(t *testing.T) {
	t.Parallel()

	cloneURL, err := url.Parse("https://github.com/org/../../outside.git")
	require.NoError(t, err)

	_, _, _, err = gitRepoSegments(cloneURL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo path")
}

func TestCachePathForGitDependency_happyPath(t *testing.T) {
	t.Parallel()

	config := &Config{CachePath: t.TempDir()}
	cloneURL, err := url.Parse("https://github.com/org/repo.git")
	require.NoError(t, err)

	ref := strings.Repeat("a", 40)
	got, err := config.CachePathForGitDependency(cloneURL, "packages/foo.dar", ref)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(config.CachePath, "git", "github.com", "org", "repo", ref, "packages", "foo.dar"), got)
}

func TestCachePathForGitDependency_rewritesHostPort(t *testing.T) {
	t.Parallel()

	config := &Config{CachePath: t.TempDir()}
	cloneURL, err := url.Parse("http://127.0.0.1:62255/org/repo")
	require.NoError(t, err)

	got, err := config.GitWorkPathForRepo(cloneURL)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(config.CachePath, "git", "127.0.0.1_62255", "org", "repo", ".work"), got)
	assert.NotContains(t, got, "127.0.0.1:62255")
}

func TestCachePathForGitDependency_fileURLDriveIsADirectoryName(t *testing.T) {
	t.Parallel()

	config := &Config{CachePath: filepath.Join("cache-root")}
	cloneURL, err := url.Parse("file:///C:/Users/repo")
	require.NoError(t, err)

	got, err := config.GitWorkPathForRepo(cloneURL)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("cache-root", "git", "local", "C_", "Users", "repo", ".work"), got)
}

func TestCachePathForGitDependency_cleansDarPath(t *testing.T) {
	t.Parallel()

	config := &Config{CachePath: t.TempDir()}
	cloneURL, err := url.Parse("https://github.com/org/repo.git")
	require.NoError(t, err)

	ref := strings.Repeat("b", 40)
	got, err := config.CachePathForGitDependency(cloneURL, "packages/./foo.dar", ref)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(config.CachePath, "git", "github.com", "org", "repo", ref, "packages", "foo.dar"), got)
}
