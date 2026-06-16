package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestDpmAddComponentCommand() {
	t := suite.T()

	newComponent := "oci://example.com/newly-added:4.5.6"

	t.Run("add new component in single-package project", func(t *testing.T) {
		projectDir := testutil.ActivateDamlYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent)
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent)
	})

	t.Run("add new component in multi-package project", func(t *testing.T) {
		projectDir := testutil.ActivateMultiPackageYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent)
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "multi-package.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent)
	})
}
