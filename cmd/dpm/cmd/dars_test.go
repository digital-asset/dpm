// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestResolutionOfBuiltInDarDependencies() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
data-dependencies:
  - foo-script
`)

	res := lo.Values(runResolveCommand(t).Packages)[0]
	assert.Contains(t, res.ResolvedDependencies, "daml-script")
	assert.Contains(t, res.ResolvedDataDependencies, "foo-script")
}

func (suite *MainSuite) TestResolutionOfFilePathBasedDarDependencies() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	t.Run("resolution of relative file-path dars", func(t *testing.T) {
		packageDir := ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - ./relative.dar
data-dependencies:
  - ./relative.dar
`))
		os.WriteFile(
			filepath.Join(packageDir, "relative.dar"),
			[]byte("another fake test dar"),
			06444)

		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.ResolvedDependencies[0], "relative.dar")
		checkDar(t, res.ResolvedDependencies[0])

		assert.Contains(t, res.ResolvedDataDependencies[0], "relative.dar")
		checkDar(t, res.ResolvedDataDependencies[0])
	})

	t.Run("resolution of absolute file-path dars", func(t *testing.T) {
		absoluteDar, _ := filepath.Abs(testutil.TestdataPath(t, "test-dar", "test.dar"))
		ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
data-dependencies:
  - %s
`, absoluteDar, absoluteDar))
		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.ResolvedDependencies[0], "test.dar")
		checkDar(t, res.ResolvedDependencies[0])

		assert.Contains(t, res.ResolvedDataDependencies[0], "test.dar")
		checkDar(t, res.ResolvedDataDependencies[0])
	})
}

func ActivateDamlYamlForTest(t *testing.T, s string) (packageDir string) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(s), 0666))
	return tmpDir
}

func checkDar(t *testing.T, darFile string) {
	assert.True(t, filepath.IsAbs(darFile), "expecting absolute dar paths in the output")
	_, err := os.ReadFile(darFile)
	require.NoError(t, err)
}
