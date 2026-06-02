// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"testing"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestPullingAndResolutionOfDarDependencies() {
	t := suite.T()

	// enable feature flag
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	// setup
	tmpDpmHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDpmHome)

	t.Run("dar resolution contains data-dependencies and dependencies", func(t *testing.T) {
		t.Chdir(testutil.TestdataPath(t, "daml-dependencies"))
		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.Errors[0].Cause, "oci://")

		assert.Len(t, res.ResolvedDataDependencies, 4)
		assert.Contains(t)

		assert.Len(t, res.Errors, 3)
		for _, err := range res.Errors {
			assert.Equal(t, err.Code, resolutionerrors.DarNotInstalled)
		}
	})
}
