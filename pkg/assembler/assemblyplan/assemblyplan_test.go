// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assemblyplan

import (
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemblyPlanForSdkBundle(t *testing.T) {
	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	_, commands := loadDamlPackage(t, testutil.TestdataPath(t, "installed-sdks"), testutil.TestdataPath(t, "daml-package-sdk-only"), "9.9.9")
	require.ElementsMatch(t, lo.Keys(commands), []string{"meep"})

	// overridden component
	meepCommnads := commands["meep"]
	require.Len(t, meepCommnads, 1)
	assert.Equal(t, meepCommnads[0].ComponentName, "meep")
	assert.Equal(t, meepCommnads[0].Command.GetName(), "useless")
}

func TestAssemblyPlanForOptInComponents(t *testing.T) {
	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	_, commands := loadDamlPackage(t, testutil.TestdataPath(t, "installed-sdks"), testutil.TestdataPath(t, "daml-package", testutil.OS), "")
	require.ElementsMatchf(t, lo.Keys(commands), []string{"meep", "javabro"}, "")

}

func TestAssemblyPlanCommandConflictSdkBundle(t *testing.T) {
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	cwd, err := os.Getwd()
	require.NoError(t, err)

	plan := load(t,
		filepath.Join(cwd, "command-conflict-testcase", "installed-sdks"),
		filepath.Join(cwd, "command-conflict-testcase", "daml-package-sdk-only"),
		"1.2.123",
	)

	_, err = plan.Assemble(ctx)
	assert.ErrorContains(t, err, "defined in multiple components")
}

func TestAssemblyPlanCommandWithOptInComponents(t *testing.T) {
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	cwd, err := os.Getwd()
	require.NoError(t, err)

	plan := load(t,
		filepath.Join(cwd, "command-conflict-testcase", "installed-sdks"),
		filepath.Join(cwd, "command-conflict-testcase", "daml-package"),
		"",
	)

	_, err = plan.Assemble(ctx)
	assert.ErrorContains(t, err, "defined in multiple components")
}

func TestMultiPackage(t *testing.T) {
	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	plan, commands := loadMultiPackage(t)
	assert.NotNil(t, plan.MultiPackage)
	require.ElementsMatchf(t, lo.Keys(commands), []string{"meep", "javabro", "multipak"}, "")
}

func loadMultiPackage(t *testing.T) (*AssemblyPlan, map[string][]*assembler.ValidatedCommand) {
	p := testutil.TestdataPath(t, "multi-package", testutil.OS)
	t.Setenv(assistantconfig.DamlMultiPackageEnvVar, p)
	return loadDamlPackage(t, testutil.TestdataPath(t, "installed-sdks"), testutil.TestdataPath(t, "daml-package", testutil.OS), "")
}

func load(t *testing.T, installedSdksPath, damlPackagePath, sdkVersion string) *AssemblyPlan {
	ctx := testutil.Context(t)

	t.Chdir(damlPackagePath)

	config := &assistantconfig.Config{
		Edition:                   assistantconfig.NewLazyEdition(sdkmanifest.OpenSource),
		InstalledSdkManifestsPath: installedSdksPath,
	}

	a, err := assembler.Fake(nil)
	require.NoError(t, err)

	plan, err := New(ctx, config, a)
	require.NoError(t, err)

	if sdkVersion == "" {
		assert.Nil(t, plan.Base.Spec.Version)
		require.NotNil(t, plan.DamlPackage)
	} else {
		// component opt-in case
		assert.Equal(t, plan.Base.Spec.Version.Value().String(), sdkVersion)
		require.Nil(t, plan.DamlPackage) // this field is nil when there are no 'components'
	}

	return plan
}

func loadDamlPackage(t *testing.T, installedSdksPath, damlPackagePath, sdkVersion string) (*AssemblyPlan, map[string][]*assembler.ValidatedCommand) {
	ctx := testutil.Context(t)

	plan := load(t, installedSdksPath, damlPackagePath, sdkVersion)
	result, err := plan.Assemble(ctx)
	require.NoError(t, err)
	return plan, result.ValidatedCommands
}
