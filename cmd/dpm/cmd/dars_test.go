// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"testing"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestDars() {
	t := suite.T()

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	testutil.StartRegistry(t)
	destinationRegistry := os.Getenv(assistantconfig.OciRegistryEnvVar)

	t.Setenv("TEST_DPM_REGISTRY", destinationRegistry)
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")
	pushDar(t, fmt.Sprintf("oci://%s/some/dars/foo:1.2.3", destinationRegistry))
	pushDar(t, fmt.Sprintf("oci://%s/some/dars/n/stuff/foo:4.5.6", destinationRegistry))
	pushDar(t, fmt.Sprintf("oci://%s/some/more/official/dars/foo:7.8.9", destinationRegistry))

	t.Chdir(testutil.TestdataPath(t, "daml-dependencies"))
	res := lo.Values(runResolveCommand(t).Packages)[0]

	assert.Len(t, res.Errors, 3)
	for _, err := range res.Errors {
		assert.Equal(t, err.Code, resolutionerrors.DarNotInstalled)
	}
}

func pushDar(t *testing.T, uri string, extraTags ...string) {
	cmd := createStdTestRootCmd(t)
	args := []string{
		"publish", "dar", uri,
		"-f", testutil.TestdataPath(t, "test-dar"),
	}

	for _, t := range extraTags {
		args = append(args, "--extra-tags", t)
	}

	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())

}
