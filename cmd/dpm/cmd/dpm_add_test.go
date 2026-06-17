package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func (suite *MainSuite) TestDpmAddComponentCommand() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_, reg := testutil.StartRegistry(t)
	newComponentRepo := "newly/added:4.5.6"
	newComponent := fmt.Sprintf("oci://%s/%s", testutil.GetRemote(reg).Registry, newComponentRepo)

	args := testutil.PushComponentUri(reg, newComponentRepo, testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	t.Run("add new component in single-package project", func(t *testing.T) {
		projectDir := testutil.ActivateDamlYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent, "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent+"@sha256:")
	})

	t.Run("add new component in multi-package project", func(t *testing.T) {
		projectDir := testutil.ActivateMultiPackageYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent, "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "multi-package.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent+"@sha256:")
	})
}

func (suite *MainSuite) TestDpmAddDarCommand() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	darRef, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:4.5.6", reg))
	require.NoError(t, err)
	pushDar(t, "oci://"+darRef.String())

	t.Run("add new dar in single-package project", func(t *testing.T) {
		projectDir := testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
  - std-lib
`)

		cmd := createStdTestRootCmd(t, "add", "dar", "--data-dependencies", "oci://"+darRef.String(), "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- oci://"+darRef.String()+"@sha256:")
	})

}

func (suite *MainSuite) TestDpmAddDarCommandNegativeCases() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	t.Run("add new dar to both data-dependencies and dependencies fails in single-package project", func(t *testing.T) {
		_ = testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
  - std-lib
`)

		cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", "--data-dependencies", "oci://doesnt/matter/for/this/test:1.2.3", "--insecure")
		require.Error(t, cmd.Execute())
	})
}
