package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/packagelock"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get(t *testing.T, lock *packagelock.PackageLock, s string) *packagelock.Dar {
	d, ok := lo.Find(lock.Dars, func(d *packagelock.Dar) bool {
		return d.URI.String() == s
	})
	require.Truef(t, ok, "expected %q dar is missing in lockfile", s)
	return d
}

func (suite *MainSuite) TestLockfileUpdate() {
	t := suite.T()
	ctx := t.Context()

	t.Setenv(assistantconfig.DpmLockfileEnabledEnvVar, "true")

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	_, reg := testutil.StartRegistry(t)
	multiPackageDir := testutil.TestdataPath(t, "simple-multi-package")
	t.Setenv(assistantconfig.DamlMultiPackageEnvVar, multiPackageDir)

	cleanup := func() {
		_ = os.Remove(filepath.Join(multiPackageDir, "a", assistantconfig.DamlLocalFilename))
		_ = os.Remove(filepath.Join(multiPackageDir, "b", assistantconfig.DamlLocalFilename))
	}
	cleanup()
	t.Cleanup(cleanup)

	// TODO: using a PushComponent() for lack of a PushDar() for now
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "some-dar"), "latest")
	testutil.PushComponent(t, ctx, reg, "sheep", "4.5.6", testutil.TestdataPath(t, "some-dar"), "latest")

	cmd := createStdTestRootCmd(t, "update")
	require.NoError(t, cmd.Execute())

	aLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "a", assistantconfig.DpmLockFileName))
	require.NoError(t, err)
	assert.Len(t, aLock.Dars, 2)
	d := get(t, aLock, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)))
	assert.NotEmpty(t, d.Digest)

	d = get(t, aLock, "builtin://daml-script")
	assert.Empty(t, d.Digest)

	bLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "b", assistantconfig.DpmLockFileName))
	require.NoError(t, err)
	assert.Len(t, bLock.Dars, 2)

	d = get(t, bLock, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)))
	assert.NotEmpty(t, d.Digest)

	d = get(t, bLock, fmt.Sprintf("oci://%s/components/sheep:4.5.6", os.Getenv(assistantconfig.OciRegistryEnvVar)))
	assert.NotEmpty(t, d.Digest)

	t.Run("bump versions", func(t *testing.T) {
		testutil.PushComponent(t, ctx, reg, "meep", "2.0.0", testutil.TestdataPath(t, "some-dar"), "latest")
		testutil.PushComponent(t, ctx, reg, "sheep", "5.0.0", testutil.TestdataPath(t, "some-dar"), "latest")

		cmd := createStdTestRootCmd(t, "update")
		require.NoError(t, cmd.Execute())

		aLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "a", assistantconfig.DpmLockFileName))
		require.NoError(t, err)
		bLock, err = packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "b", assistantconfig.DpmLockFileName))
		require.NoError(t, err)

		assert.Len(t, aLock.Dars, 2)
		assert.Len(t, bLock.Dars, 2)

		t.Run("pinned stay pinned", func(t *testing.T) {
			d = get(t, aLock, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)))
			assert.NotEmpty(t, d.Digest)

			d = get(t, bLock, fmt.Sprintf("oci://%s/components/sheep:4.5.6", os.Getenv(assistantconfig.OciRegistryEnvVar)))
			assert.NotEmpty(t, d.Digest)
		})

		t.Run("floaty get bumped", func(t *testing.T) {
			d = get(t, bLock, fmt.Sprintf("oci://%s/components/meep:2.0.0", os.Getenv(assistantconfig.OciRegistryEnvVar)))
			assert.NotEmpty(t, d.Digest)
		})
	})

	t.Run("dars in resolution", func(t *testing.T) {
		cmd, r, w := createTestRootCmd(t, "resolve")
		require.NoError(t, cmd.Execute())
		require.NoError(t, w.Close())

		output, err := io.ReadAll(r)
		require.NoError(t, err)

		deepResolution := resolution.Resolution{}
		require.NoError(t, yaml.Unmarshal(output, &deepResolution))

		aRes := deepResolution.Packages[filepath.Join(multiPackageDir, "a")].Imports[resolution.DarImportsFields]
		assert.Len(t, aRes, 2)
		assert.Len(t,
			deepResolution.Packages[filepath.Join(multiPackageDir, "b")].Imports[resolution.DarImportsFields],
			2,
		)

		assert.Contains(t, aRes, "daml-script")

		dar, err := os.ReadFile(aRes[1])
		require.NoError(t, err)
		assert.Contains(t, string(dar), "haha not a real dar")
	})
}
