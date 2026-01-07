// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assemblyplan

import (
	"fmt"
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

func TestAssemblyPlan(t *testing.T) {
	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	_, commands := loadDamlPackage(t, testutil.TestdataPath(t, "installed-sdks"), testutil.TestdataPath(t, "daml-package", testutil.OS), "9.9.9")
	require.ElementsMatchf(t, lo.Keys(commands), []string{"meep", "javabro"}, "")

	// overridden component
	meepCommnads := commands["meep"]
	require.Len(t, meepCommnads, 2)
	assert.Equal(t, meepCommnads[0].ComponentName, "meep")
	assert.Equal(t, meepCommnads[0].Command.GetName(), "meep")
	assert.Equal(t, meepCommnads[1].ComponentName, "meep")
	assert.Equal(t, meepCommnads[1].Command.GetName(), "show-env-vars")
}

func TestAssemblyPlanCommandConflict(t *testing.T) {
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.EditionEnvVar, "open-source")
	cwd, err := os.Getwd()
	require.NoError(t, err)

	plan := load(t,
		filepath.Join(cwd, "command-conflict-testcase", "installed-sdks"),
		filepath.Join(cwd, "command-conflict-testcase", "daml-package"),
		"1.2.123",
	)

	_, err = plan.Assemble(ctx)
	assert.Error(t, err)
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
	return loadDamlPackage(t, testutil.TestdataPath(t, "installed-sdks"), testutil.TestdataPath(t, "daml-package", testutil.OS), "9.9.9")
}

func load(t *testing.T, installedSdksPath, damlPackagePath, sdkVersion string) *AssemblyPlan {
	ctx := testutil.Context(t)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	// reset original cwd
	defer func() { require.NoError(t, os.Chdir(cwd)) }()

	// this will make daml.yaml in the CWD
	require.NoError(t, os.Chdir(damlPackagePath))
	newCwd, _ := os.Getwd()
	fmt.Println(newCwd)

	config := &assistantconfig.Config{
		Edition:                   assistantconfig.NewLazyEdition(sdkmanifest.OpenSource),
		InstalledSdkManifestsPath: installedSdksPath,
	}

	a, err := assembler.Fake(nil)
	require.NoError(t, err)

	plan, err := New(ctx, config, a)
	require.NoError(t, err)
	assert.Equal(t, plan.Base.Spec.Version.Value().String(), sdkVersion)

	require.NotNil(t, plan.DamlPackage)

	return plan
}

func loadDamlPackage(t *testing.T, installedSdksPath, damlPackagePath, sdkVersion string) (*AssemblyPlan, map[string][]*assembler.ValidatedCommand) {
	ctx := testutil.Context(t)

	plan := load(t, installedSdksPath, damlPackagePath, sdkVersion)
	result, err := plan.Assemble(ctx)
	require.NoError(t, err)
	return plan, result.ValidatedCommands
}
