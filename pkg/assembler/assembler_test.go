// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assembler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble(t *testing.T) {
	ctx := testutil.Context(t)
	t.Setenv(assistantconfig.EditionEnvVar, "enterprise")

	a, err := Fake(nil)
	require.NoError(t, err)

	result, err := a.ReadAndAssemble(ctx, testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))
	require.NoError(t, err)

	p := testutil.TestdataPath(t, "meepy-component", testutil.OS, "meep")
	if testutil.OS == "windows" {
		p = testutil.TestdataPath(t, "meepy-component", testutil.OS, "meep.bat")
	}

	// local dir component
	expected, err := filepath.Abs(p)
	require.NoError(t, err)
	assert.Equal(t, expected, getCommandByName(result.ValidatedCommands, "meep").AbsolutePath)

	expected, err = filepath.Abs(testutil.TestdataPath(t, "javabro-component", "jars", "fake.jar"))
	require.NoError(t, err)
	assert.Equal(t, expected, getCommandByName(result.ValidatedCommands, "javux").AbsolutePath)

	t.Run("exports and imports", func(t *testing.T) {
		assert.Len(t, result.ShallowResolution.Imports, 2)
		assert.Len(t, result.ShallowResolution.Imports["MEEP_EXTERNAL_DAR"], 3)
		assert.Len(t, result.ShallowResolution.Imports["SHEEP_EXTERNAL_DAR"], 2)
	})
}

func TestBinaryPathValidation(t *testing.T) {
	ctx := testutil.Context(t)
	t.Setenv(assistantconfig.EditionEnvVar, "enterprise")

	a, err := Fake(nil)
	require.NoError(t, err)

	_, err = a.ReadAndAssemble(ctx, testutil.TestdataPath(t, "missing-bin.yaml"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorContains(t, err, "oops")
}

func TestAssembleRemote(t *testing.T) {
	ctx := testutil.Context(t)
	t.Setenv(assistantconfig.EditionEnvVar, "enterprise")

	// currently, allowing pull of remote overridden components requires setting auto-install to true
	t.Setenv(assistantconfig.AutoInstallEnvVar, "true")

	registry := httptest.NewTLSServer(registry.New())
	defer registry.Close()

	repoName := "meep" // repo name must match component's name in the assembly manifest
	tag := "1.2.3"
	testutil.PushComponent(t, ctx, registry, repoName, tag, testutil.TestdataPath(t, "meepy-component", testutil.OS))
	testutil.PushComponent(t, ctx, registry, sdkmanifest.AssistantName, "4.5.6", testutil.TestdataPath(t, "assistant-binary", testutil.OS))

	a, err := Fake(registry)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		result, err := a.ReadAndAssemble(ctx, testutil.TestdataPath(t, "remote-components.yaml"))
		require.NoError(t, err)
		assert.Len(t, result.ValidatedCommands, 1)
	}
}

func getCommandByName(cmds map[string][]*ValidatedCommand, commandName string) *ValidatedCommand {
	flattened := lo.Flatten(lo.Values(cmds))
	return lo.FirstOr(lo.Filter(flattened, func(cmd *ValidatedCommand, _ int) bool {
		return cmd.GetName() == commandName
	}), nil)
}
