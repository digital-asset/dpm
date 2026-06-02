// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/resolution"
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
		var res *resolution.Resolution

		ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
data-dependencies:
  - foo-script
`)

		t.Run("dpm resolve command exits successfully", func(t *testing.T) {
			res = runResolveCommand(t)
		})

		t.Run("builtin dars get included in resolution", func(t *testing.T) {
			assert.Contains(t, lo.Values(res.Packages)[0].ResolvedDependencies, "daml-script")
			assert.Contains(t, lo.Values(res.Packages)[0].ResolvedDataDependencies, "foo-script")
		})
	})
}

func ActivateDamlYamlForTest(t *testing.T, s string) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(s), 0666))
}
