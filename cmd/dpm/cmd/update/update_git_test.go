package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestCheckGitDependency_releaseDep(t *testing.T) {
	config := testutil.MkConfig(t)

	dep, err := damlpackage.ParseGitDependency("git:github.com/org/repo?release=v1.0.0&asset=pkg-1.0.0.dar")
	require.NoError(t, err)
	require.True(t, dep.GitRelease)

	ctx := context.Background()

	err = checkGitDependency(ctx, config, dep)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not installed")

	cached, err := config.CachePathForGitRelease(dep.CloneURL, dep.GitRef, dep.DarPath)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cached), 0o755))
	require.NoError(t, os.WriteFile(cached, []byte("release dar"), 0o644))

	require.NoError(t, checkGitDependency(ctx, config, dep))
}

func TestCheckGitDependency_unexpandedRelease(t *testing.T) {
	config := testutil.MkConfig(t)

	dep, err := damlpackage.ParseGitDependency("git:github.com/org/repo?release=v1.0.0")
	require.NoError(t, err)
	require.True(t, dep.GitRelease)

	err = checkGitDependency(context.Background(), config, dep)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no asset")
}
