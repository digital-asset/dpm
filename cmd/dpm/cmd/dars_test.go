// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestResolutionOfDarDependencies() {
	t := suite.T()

	// enable feature flag
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	// setup
	tmpDpmHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDpmHome)

	t.Run("dpm resolve data-dependencies and dependencies fields", func(t *testing.T) {
		var res *resolution.Package
		testDar, _ := filepath.Abs(testutil.TestdataPath(t, "test-dar", "test.dar"))

		packageDir := ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s  # absolute filepath dar
  - daml-script

data-dependencies:
  - ./test2.dar
  - foo-script
`, testDar))

		test2Dar := filepath.Join(packageDir, "test2.dar")
		os.WriteFile(test2Dar, []byte("another fake test dar"), 06444)

		t.Run("dpm resolve command exits successfully", func(t *testing.T) {
			output := runResolveCommand(t)
			res = lo.Values(output.Packages)[0]
		})

		t.Run("builtin dars get included in resolution", func(t *testing.T) {
			assert.Contains(t, res.ResolvedDependencies, "daml-script")
			assert.Contains(t, res.ResolvedDataDependencies, "foo-script")
		})

		t.Run("relative filepath-based dars get included in resolution", func(t *testing.T) {
			assert.Contains(t, res.ResolvedDependencies, testDar)
		})
		t.Run("absolute filepath-based dars get included in resolution", func(t *testing.T) {
			assert.Contains(t, res.ResolvedDataDependencies, test2Dar)
		})
	})
}

func ActivateDamlYamlForTest(t *testing.T, s string) (packageDir string) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(s), 0666))
	d, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	return d
}
