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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestLockfileUpdate() {
	t := suite.T()
	ctx := t.Context()

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	_, reg := testutil.StartRegistry(t)
	multiPackageDir := testutil.TestdataPath(t, "simple-multi-package")
	t.Setenv(assistantconfig.DamlMultiPackageEnvVar, multiPackageDir)

	// TODO: using a PushComponent() for lack of a PushDar() for now
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "some-dar"), "latest")
	testutil.PushComponent(t, ctx, reg, "sheep", "4.5.6", testutil.TestdataPath(t, "some-dar"), "latest")

	cmd := createStdTestRootCmd(t, "update")
	require.NoError(t, cmd.Execute())

	aLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "a", assistantconfig.DpmLockFileName))
	require.NoError(t, err)
	assert.Len(t, aLock.Dars, 1)
	assert.Equal(t, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)), aLock.Dars[0].URI)
	assert.NotEmpty(t, aLock.Dars[0].Digest)

	bLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "b", assistantconfig.DpmLockFileName))
	require.NoError(t, err)
	assert.Len(t, bLock.Dars, 2)
	assert.Equal(t, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)), bLock.Dars[0].URI)
	assert.NotEmpty(t, bLock.Dars[0].Digest)
	assert.Equal(t, fmt.Sprintf("oci://%s/components/sheep:4.5.6", os.Getenv(assistantconfig.OciRegistryEnvVar)), bLock.Dars[1].URI)
	assert.NotEmpty(t, bLock.Dars[1].Digest)

	t.Run("bump versions", func(t *testing.T) {
		testutil.PushComponent(t, ctx, reg, "meep", "2.0.0", testutil.TestdataPath(t, "some-dar"), "latest")
		testutil.PushComponent(t, ctx, reg, "sheep", "5.0.0", testutil.TestdataPath(t, "some-dar"), "latest")

		cmd := createStdTestRootCmd(t, "update")
		require.NoError(t, cmd.Execute())

		aLock, err := packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "a", assistantconfig.DpmLockFileName))
		require.NoError(t, err)
		bLock, err = packagelock.ReadPackageLock(filepath.Join(multiPackageDir, "b", assistantconfig.DpmLockFileName))
		require.NoError(t, err)

		assert.Len(t, aLock.Dars, 1)
		assert.Len(t, bLock.Dars, 2)

		t.Run("pinned stay pinned", func(t *testing.T) {
			assert.Equal(t, fmt.Sprintf("oci://%s/components/meep:1.2.3", os.Getenv(assistantconfig.OciRegistryEnvVar)), aLock.Dars[0].URI)
			assert.NotEmpty(t, aLock.Dars[0].Digest)

			assert.Equal(t, fmt.Sprintf("oci://%s/components/sheep:4.5.6", os.Getenv(assistantconfig.OciRegistryEnvVar)), bLock.Dars[1].URI)
			assert.NotEmpty(t, bLock.Dars[1].Digest)
		})

		t.Run("floaty get bumped", func(t *testing.T) {
			assert.Equal(t, fmt.Sprintf("oci://%s/components/meep:2.0.0", os.Getenv(assistantconfig.OciRegistryEnvVar)), bLock.Dars[0].URI)
			assert.NotEmpty(t, bLock.Dars[0].Digest)
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

		assert.Len(t,
			deepResolution.Packages[filepath.Join(multiPackageDir, "a")].Imports[resolution.DarImportsFields],
			1,
		)
		assert.Len(t,
			deepResolution.Packages[filepath.Join(multiPackageDir, "b")].Imports[resolution.DarImportsFields],
			2,
		)

		dar, err := os.ReadFile(deepResolution.Packages[filepath.Join(multiPackageDir, "a")].Imports[resolution.DarImportsFields][0])
		require.NoError(t, err)
		assert.Contains(t, string(dar), "haha not a real dar")
	})
}
