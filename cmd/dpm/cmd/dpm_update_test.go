package cmd

import (
	"fmt"
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
