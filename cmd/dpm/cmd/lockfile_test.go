package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/packagelock"
	"daml.com/x/assistant/pkg/testutil"
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
	assert.NoError(t, cmd.Execute())

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

	// TODO test bumping dar versions
}
