package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestDpmAddComponentCommand() {
	t := suite.T()

	newComponent := "oci://example.com/newly-added:4.5.6"

	projectDir := ActivateDamlYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

	cmd := createStdTestRootCmd(t, "add", "component", newComponent)
	require.NoError(t, cmd.Execute())

	t.Run("modifies the daml.yaml file", func(t *testing.T) {
		damlYaml, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(damlYaml), "- "+newComponent)
	})
}
