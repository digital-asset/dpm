package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func (suite *MainSuite) TestDpmUpdateCommandWorksInSingleOrMultiPackageProject() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	packageDir := testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
 - std-lib
`)

	t.Run("in single package project", func(t *testing.T) {
		cmd, r, w := createTestRootCmd(t, "update")
		assert.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())

		output, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Contains(t, string(output), "Updating package "+packageDir)
	})

	t.Run("in multi package project", func(t *testing.T) {
		package2Dir := testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
 - std-lib
`)

		_ = testutil.ActivateMultiPackageYamlForTest(t, fmt.Sprintf(`
packages:
 - %s
 - %s
`, packageDir, package2Dir))

		cmd, r, w := createTestRootCmd(t, "update")
		require.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())

		output, err := io.ReadAll(r)
		assert.NoError(t, err)
		assert.Contains(t, string(output), "Updating package "+packageDir)
		assert.Contains(t, string(output), "Updating package "+package2Dir)
	})
}

func (suite *MainSuite) TestDpmUpdateCommandForFloatyComponents() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_ = testutil.MkConfig(t)
	_, reg := testutil.StartRegistry(t)
	componentRepo := "newly/added"
	componentOciUri := fmt.Sprintf("oci://%s/%s", testutil.GetRemote(reg).Registry, componentRepo)

	args := testutil.PushComponentUri(reg, componentRepo+":1.1.1", testutil.TestdataPath(t, "meepy-component", testutil.OS), "latest")
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
components:
  - %s:latest
`, componentOciUri))

	var uriBeforeUpdate string

	t.Run("dpm install should append missing sha256 to oci uri", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "install")
		require.NoError(t, cmd.Execute())

		pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)

		uriBeforeUpdate = *pkg.ComponentsList[0].StringBased
		assert.Contains(t, uriBeforeUpdate, componentOciUri+":latest@sha256:")
	})

	t.Run("update command bumps floaty components", func(t *testing.T) {
		// push new version under "latest" tag
		args := testutil.PushComponentUri(reg, componentRepo+":2.2.2", testutil.TestdataPath(t, "meepy-component", testutil.OS), "latest")
		require.NoError(t, createStdTestRootCmd(t, args...).Execute())

		cmd := createStdTestRootCmd(t, "update")
		require.NoError(t, cmd.Execute())

		pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		require.Len(t, pkg.Components, 1)

		uriAfterUpdate := *pkg.ComponentsList[0].StringBased
		assert.Contains(t, uriAfterUpdate, componentOciUri+":latest@sha256:")

		assert.NotEqual(t, uriBeforeUpdate, uriAfterUpdate)
	})

	t.Run("running dpm update more than once in a row", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "update")
		require.NoError(t, cmd.Execute())
	})
}

func (suite *MainSuite) TestDpmUpdateCommandForFloatyDars() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	projectDir := testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
  - std-lib
`)

	darRefLatest, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:latest", reg))
	require.NoError(t, err)

	darRef1, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:1.1.1", reg))
	require.NoError(t, err)

	darRef2, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:2.2.2", reg))
	require.NoError(t, err)

	var valueBeforeUpdate string

	t.Run("add new dar in single-package project", func(t *testing.T) {
		pushDar(t, "oci://"+darRef1.String(), "latest")

		cmd := createStdTestRootCmd(t, "add", "dar", "--data-dependencies", "oci://"+darRefLatest.String(), "--insecure")
		require.NoError(t, cmd.Execute())

		damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)

		valueBeforeUpdate, err = damlPkg.DataDependencies[1].Value()
		require.NoError(t, err)
		assert.Contains(t, valueBeforeUpdate, "latest@sha256")
	})

	t.Run("update command should update floaty dars", func(t *testing.T) {
		pushDar(t, "oci://"+darRef2.String(), "latest")

		cmd := createStdTestRootCmd(t, "update", "--force-insecure")
		require.NoError(t, cmd.Execute())

		damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)

		assert.Len(t, damlPkg.DataDependencies, 2)
		valueAfterUpdate, err := damlPkg.DataDependencies[1].Value()
		require.NoError(t, err)

		assert.Contains(t, valueAfterUpdate, "latest@sha256")

		// make sure it's a different sha256 now
		assert.NotEqual(t, valueAfterUpdate, valueBeforeUpdate)
	})

}
