package gitpuller

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustGitDep(t *testing.T, raw string) *damlpackage.ParsedDarDependency {
	t.Helper()
	d, err := gitparse.ParseGitDependency(raw)
	require.NoError(t, err)
	return damlpackage.FromGit(d)
}

func TestGitDarPuller_releaseWithoutAssetFails(t *testing.T) {
	raw := "git:github.com/org/repo.git?release=v1.0.0"
	dep := mustGitDep(t, raw)

	tmpHome := t.TempDir()
	config, err := assistantconfig.GetWithCustomDamlHome(tmpHome)
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	_, err = New(config).PullDar(t.Context(), dep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no asset")
}

func TestGitDarPuller_rejectsSymlinkOutsideRepo(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))
	outsideFile := filepath.Join(base, "secret.dar")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))

	symlinkPath := filepath.Join(repoDir, "loyalty.dar")
	require.NoError(t, os.Symlink(outsideFile, symlinkPath))

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	w, err := repo.Worktree()
	require.NoError(t, err)
	_, err = w.Add("loyalty.dar")
	require.NoError(t, err)
	commit, err := w.Commit("add symlinked dar", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test"},
	})
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName("main"),
		commit,
	)))

	cloneURL := "file://" + filepath.ToSlash(repoDir)
	raw := "git:" + cloneURL + "#main?path=loyalty.dar"
	dep := mustGitDep(t, raw)

	tmpHome := t.TempDir()
	config, err := assistantconfig.GetWithCustomDamlHome(tmpHome)
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	_, err = New(config).PullDar(t.Context(), dep)
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "symlink points outside the repository") ||
			strings.Contains(err.Error(), "failed to resolve symlink"),
		"expected symlink rejection, got: %v", err,
	)
}

func TestGitDarPuller_fileRepo(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	cloneURL := testutil.InitGitRepo(t, "loyalty.dar", []byte("fake dar contents"))
	raw := "git:" + cloneURL + "#main?path=loyalty.dar"

	dep := mustGitDep(t, raw)

	tmpHome := t.TempDir()
	config, err := assistantconfig.GetWithCustomDamlHome(tmpHome)
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	pulled, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)
	assert.NotEmpty(t, pulled.ResolvedRef)
	assert.FileExists(t, pulled.DarFilePath)
	assert.Contains(t, pulled.Digest, "sha256:")
	expected, err := config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, pulled.ResolvedRef)
	require.NoError(t, err)
	assert.Equal(t, expected, pulled.DarFilePath)

	pulledAgain, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)
	assert.Equal(t, pulled.ResolvedRef, pulledAgain.ResolvedRef)
	assert.Equal(t, pulled.DarFilePath, pulledAgain.DarFilePath)
}

func TestPullGitDar_rejectsEmptySourceBeforeCachingOrPinning(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	cloneURL := testutil.InitGitRepo(t, "loyalty.dar", nil)
	dep := mustGitDep(t, "git:"+cloneURL+"#main?path=loyalty.dar")

	config, err := assistantconfig.GetWithCustomDamlHome(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	sourceRepo, err := git.PlainOpen(strings.TrimPrefix(cloneURL, "file://"))
	require.NoError(t, err)
	mainRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("main"), true)
	require.NoError(t, err)
	pinned := dep.WithGitRef(mainRef.Hash().String())
	cachedPath, err := config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, pinned.Git.Ref)
	require.NoError(t, err)

	result, err := PullGitDar(t.Context(), config, dep)
	require.Error(t, err)
	assert.Nil(t, result, "an invalid source DAR must not produce a pin")
	assert.Contains(t, err.Error(), `dar file "loyalty.dar"`)
	assert.Contains(t, err.Error(), "is empty")
	assert.Contains(t, err.Error(), "non-empty pre-built .dar")
	assert.NoFileExists(t, cachedPath)
	assert.False(t, DarIsCached(config, pinned))
}

func TestFetchMissingReleaseAssets(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"
	darBody := []byte("release dar")

	host := testutil.GitHubReleaseServer(t, true,
		testutil.GitHubReleaseAsset{Name: asset, Body: darBody},
	)

	cloneURL, err := url.Parse("http://" + host + "/org/repo")
	require.NoError(t, err)
	dep := &damlpackage.ParsedDarDependency{
		Git: gitparse.GitSource{
			CloneURL: cloneURL,
			Ref:      tag,
			DarPath:  asset,
			Release:  true,
		},
	}

	config, err := assistantconfig.GetWithCustomDamlHome(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	n, err := FetchMissingReleaseAssets(t.Context(), config, []*damlpackage.ParsedDarDependency{dep})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, DarIsCached(config, dep))

	n, err = FetchMissingReleaseAssets(t.Context(), config, []*damlpackage.ParsedDarDependency{dep})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestDarIsCached(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	cloneURL := testutil.InitGitRepo(t, "loyalty.dar", []byte("fake dar"))
	dep := mustGitDep(t, "git:"+cloneURL+"#main?path=loyalty.dar")

	config, err := assistantconfig.GetWithCustomDamlHome(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	assert.False(t, DarIsCached(config, dep))

	pulled, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)

	pinned := dep.WithGitRef(pulled.ResolvedRef)
	assert.True(t, DarIsCached(config, pinned))
	assert.False(t, DarIsCached(config, dep.WithGitRef("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")))
}

func TestGitDarPuller_refetchesEmptyPinnedRepoCache(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	const darContents = "fake dar contents"
	cloneURL := testutil.InitGitRepo(t, "loyalty.dar", []byte(darContents))
	dep := mustGitDep(t, "git:"+cloneURL+"#main?path=loyalty.dar")

	config, err := assistantconfig.GetWithCustomDamlHome(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	pulled, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)
	pinned := dep.WithGitRef(pulled.ResolvedRef)

	require.NoError(t, os.WriteFile(pulled.DarFilePath, nil, 0o644))
	require.False(t, DarIsCached(config, pinned), "an empty DAR must not count as cached")

	recovered, err := New(config).PullDar(t.Context(), pinned)
	require.NoError(t, err)
	recoveredContents, err := os.ReadFile(recovered.DarFilePath)
	require.NoError(t, err)
	assert.Equal(t, darContents, string(recoveredContents),
		"the puller should replace an invalid empty cache entry from the repository")
}

func TestGitDarPuller_refetchesEmptyReleaseCache(t *testing.T) {
	const (
		tag         = "v1.0.0"
		asset       = "pkg-1.0.0.dar"
		darContents = "release dar contents"
	)

	host := testutil.GitHubReleaseServer(t, true,
		testutil.GitHubReleaseAsset{Name: asset, Body: []byte(darContents)},
	)

	cloneURL, err := url.Parse("http://" + host + "/org/repo")
	require.NoError(t, err)
	dep := &damlpackage.ParsedDarDependency{
		Git: gitparse.GitSource{
			CloneURL: cloneURL,
			Ref:      tag,
			DarPath:  asset,
			Release:  true,
		},
	}

	config, err := assistantconfig.GetWithCustomDamlHome(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())
	cachedPath, err := config.CachePathForGitRelease(cloneURL, tag, asset)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachedPath), 0o755))
	require.NoError(t, os.WriteFile(cachedPath, nil, 0o644))
	require.False(t, DarIsCached(config, dep), "an empty release asset must not count as cached")

	recovered, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)
	recoveredContents, err := os.ReadFile(recovered.DarFilePath)
	require.NoError(t, err)
	assert.Equal(t, darContents, string(recoveredContents),
		"the puller should redownload an invalid empty release cache entry")
}

func TestGitDarPuller_skipsFetchWhenPinnedCacheExists(t *testing.T) {
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	cloneURL := testutil.InitGitRepo(t, "loyalty.dar", []byte("fake dar contents"))
	raw := "git:" + cloneURL + "#main?path=loyalty.dar"

	dep := mustGitDep(t, raw)

	tmpHome := t.TempDir()
	config, err := assistantconfig.GetWithCustomDamlHome(tmpHome)
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	pulled, err := New(config).PullDar(t.Context(), dep)
	require.NoError(t, err)

	workPath, err := config.GitWorkPathForRepo(dep.Git.CloneURL)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(workPath))

	pinned := dep.WithGitRef(pulled.ResolvedRef)
	pinnedAgain, err := New(config).PullDar(t.Context(), pinned)
	require.NoError(t, err)
	assert.Equal(t, pulled.ResolvedRef, pinnedAgain.ResolvedRef)
	assert.Equal(t, pulled.DarFilePath, pinnedAgain.DarFilePath)
	assert.NoFileExists(t, workPath)
}

func TestGitDarPuller_reusesExistingClone(t *testing.T) {
	if os.Getenv("DPM_TEST_GIT_NETWORK") != "true" {
		t.Skip("set DPM_TEST_GIT_NETWORK=true to run network git puller tests")
	}

	raw := `git:github.com/gonzamontiel/test-daml-hello.git#master?path=dist/test-daml-hello-0.0.1.dar`
	dep := mustGitDep(t, raw)

	tmpHome := t.TempDir()
	config, err := assistantconfig.GetWithCustomDamlHome(tmpHome)
	require.NoError(t, err)
	require.NoError(t, config.EnsureDirs())

	puller := New(config)
	pulled, err := puller.PullDar(t.Context(), dep)
	require.NoError(t, err)

	pulledAgain, err := puller.PullDar(t.Context(), dep)
	require.NoError(t, err)
	assert.Equal(t, pulled.ResolvedRef, pulledAgain.ResolvedRef)
}
