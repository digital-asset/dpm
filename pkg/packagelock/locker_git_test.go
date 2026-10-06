package packagelock

import (
	"context"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shaMain   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaStable = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fooLine   = "git:github.com/org/repo#main?path=packages/foo.dar"
	barLine   = "git:github.com/org/repo#main?path=packages/bar.dar"
)

func TestGitDependencyForUpdate_reusesSinglePin(t *testing.T) {
	main := gitDep(t, fooLine)
	reuse := gitPinReuseFrom(lockWith(t, fooLine, shaMain), []*Dar{gitDar(main)})

	got, err := gitDependencyForUpdate(main, reuse)
	require.NoError(t, err)
	assert.Equal(t, shaMain, got.Git.Ref)
	assert.Equal(t, "main", main.Git.Ref)
}

func TestGitDependencyForUpdate_reusesPinForDuplicateRef(t *testing.T) {
	main := gitDep(t, fooLine)
	withGitSuffix := gitDep(t, "git:github.com/org/repo.git#main?path=packages/foo.dar")
	expected := []*Dar{gitDar(main), gitDar(withGitSuffix)}
	reuse := gitPinReuseFrom(lockWith(t, fooLine, shaMain), expected)

	for _, dep := range []*damlpackage.ParsedDarDependency{main, withGitSuffix} {
		got, err := gitDependencyForUpdate(dep, reuse)
		require.NoError(t, err)
		assert.Equal(t, shaMain, got.Git.Ref)
	}
}

func TestGitDependencyForUpdate_reusesPinsForDifferentPaths(t *testing.T) {
	foo := gitDep(t, fooLine)
	bar := gitDep(t, barLine)
	expected := []*Dar{gitDar(foo), gitDar(bar)}
	lock := &PackageLock{Dars: []*Dar{
		lockedDar(t, fooLine, shaMain),
		lockedDar(t, barLine, shaStable),
	}}
	reuse := gitPinReuseFrom(lock, expected)

	gotFoo, err := gitDependencyForUpdate(foo, reuse)
	require.NoError(t, err)
	gotBar, err := gitDependencyForUpdate(bar, reuse)
	require.NoError(t, err)
	assert.Equal(t, shaMain, gotFoo.Git.Ref)
	assert.Equal(t, shaStable, gotBar.Git.Ref)
}

func TestGitDependencyForUpdate_rejectsTwoLockedSHAs(t *testing.T) {
	main := gitDep(t, fooLine)
	lock := &PackageLock{Dars: []*Dar{
		lockedDar(t, fooLine, shaMain),
		lockedDar(t, "git:github.com/org/repo#stable?path=packages/foo.dar", shaStable),
	}}
	reuse := gitPinReuseFrom(lock, []*Dar{gitDar(main)})

	_, err := gitDependencyForUpdate(main, reuse)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "packages/foo.dar")
	assert.Contains(t, err.Error(), "github.com/org/repo")
	assert.Contains(t, err.Error(), "daml.yaml")
	assert.Equal(t, "main", main.Git.Ref)
}

func TestGitDependencyForUpdate_rejectsTwoRefsSharingOnePin(t *testing.T) {
	main := gitDep(t, fooLine)
	stable := gitDep(t, "git:github.com/org/repo#stable?path=packages/foo.dar")
	expected := []*Dar{gitDar(main), gitDar(stable)}
	reuse := gitPinReuseFrom(lockWith(t, fooLine, shaMain), expected)

	_, err := gitDependencyForUpdate(main, reuse)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fooLine)
	assert.Contains(t, err.Error(), "packages/foo.dar")
	assert.Contains(t, err.Error(), "github.com/org/repo")
}

func TestGitDependencyForUpdate_rejectsMutableRefBesidePinnedRef(t *testing.T) {
	main := gitDep(t, fooLine)
	pinned := main.WithGitRef(shaStable)
	expected := []*Dar{gitDar(main), gitDar(pinned)}
	reuse := gitPinReuseFrom(lockWith(t, fooLine, shaStable), expected)

	_, err := gitDependencyForUpdate(main, reuse)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "packages/foo.dar")

	got, err := gitDependencyForUpdate(pinned, reuse)
	require.NoError(t, err)
	assert.Equal(t, shaStable, got.Git.Ref)
}

func TestGitDependencyForUpdate_resolvesDistinctRefsWhenLockHasNoPin(t *testing.T) {
	main := gitDep(t, fooLine)
	stable := gitDep(t, "git:github.com/org/repo#stable?path=packages/foo.dar")
	expected := []*Dar{gitDar(main), gitDar(stable)}
	reuse := gitPinReuseFrom(nil, expected)

	gotMain, err := gitDependencyForUpdate(main, reuse)
	require.NoError(t, err)
	gotStable, err := gitDependencyForUpdate(stable, reuse)
	require.NoError(t, err)
	assert.Equal(t, "main", gotMain.Git.Ref)
	assert.Equal(t, "stable", gotStable.Git.Ref)
}

func TestGitDependencyForUpdate_leavesPinnedYAMLRef(t *testing.T) {
	pinned := gitDep(t, fooLine).WithGitRef(shaMain)
	other := gitDep(t, fooLine).WithGitRef(shaStable)
	expected := []*Dar{gitDar(pinned), gitDar(other)}
	lock := &PackageLock{Dars: []*Dar{
		lockedDar(t, fooLine, shaMain),
		lockedDar(t, fooLine, shaStable),
	}}
	reuse := gitPinReuseFrom(lock, expected)

	got, err := gitDependencyForUpdate(pinned, reuse)
	require.NoError(t, err)
	assert.Same(t, pinned, got)
}

func TestGitDependencyForUpdate_keepsDuplicateLockedSHA(t *testing.T) {
	main := gitDep(t, fooLine)
	lock := &PackageLock{Dars: []*Dar{
		lockedDar(t, fooLine, shaMain),
		lockedDar(t, fooLine, shaMain),
	}}
	reuse := gitPinReuseFrom(lock, []*Dar{gitDar(main)})

	got, err := gitDependencyForUpdate(main, reuse)
	require.NoError(t, err)
	assert.Equal(t, shaMain, got.Git.Ref)
}

func TestGitDependencyForUpdate_nilDependency(t *testing.T) {
	got, err := gitDependencyForUpdate(nil, newGitPinReuse())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGitPinsFromExistingLock_missingFile(t *testing.T) {
	reuse := gitPinsFromExistingLock(filepath.Join(t.TempDir(), "missing.yaml"))
	assert.Empty(t, reuse.pins)
	assert.Empty(t, reuse.ambiguous)
}

func TestResolveGitDar_rejectsAmbiguousPinBeforeFetch(t *testing.T) {
	main := gitDep(t, fooLine)
	stable := gitDep(t, "git:github.com/org/repo#stable?path=packages/foo.dar")
	expected := []*Dar{gitDar(main), gitDar(stable)}
	reuse := gitPinReuseFrom(lockWith(t, fooLine, shaMain), expected)

	err := (&Locker{}).resolveGitDar(context.Background(), gitDar(main), reuse)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pin each ref to a commit SHA in daml.yaml")
	assert.Equal(t, "main", main.Git.Ref)
}

func gitPinReuseFrom(lock *PackageLock, expected []*Dar) gitPinReuse {
	reuse := gitPinsFromLock(lock)
	reuse.disallowSharedMutablePins(expected)
	return reuse
}

func gitDep(t *testing.T, raw string) *damlpackage.ParsedDarDependency {
	t.Helper()
	parsed, err := gitparse.ParseGitDependency(raw)
	require.NoError(t, err)
	return damlpackage.FromGit(parsed)
}

func gitDar(dep *damlpackage.ParsedDarDependency) *Dar {
	return &Dar{URI: dep.FullUrl, Dependency: dep}
}

func lockWith(t *testing.T, raw, sha string) *PackageLock {
	t.Helper()
	return &PackageLock{Dars: []*Dar{lockedDar(t, raw, sha)}}
}

func lockedDar(t *testing.T, raw, sha string) *Dar {
	t.Helper()
	return &Dar{URI: gitDep(t, raw).WithGitRef(sha).FullUrl}
}
