// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"daml.com/x/assistant/pkg/assembler"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// THe assistant version should always match the active SDK's assistant
func (suite *MainSuite) TestAssistantUpgrade() {
	t := suite.T()
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	sdkVersion := "0.0.1-whatever"
	newerSdkVersion, err := semver.NewVersion("0.0.3")
	require.NoError(t, err)
	newestSdkVersion := "0.0.4-but-contains-old-assistant"
	olderAssistantVersion := "4.5.6"
	newerAssistantVersion := "5.0.0"

	// push assembly, assistant, and component
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, sdkVersion, testutil.TestdataPath(t, "remote-components.yaml"))
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	testutil.PushComponent(t, ctx, reg, sdkmanifest.AssistantName, olderAssistantVersion, testutil.TestdataPath(t, "assistant-binary", testutil.OS))
	newerAssistantPath := copyAssistant(t, testutil.TestdataPath(t, "assistant-binary", testutil.OS), newerAssistantVersion)
	testutil.PushComponent(t, ctx, reg, sdkmanifest.AssistantName, newerAssistantVersion, newerAssistantPath)

	newerAssembly := createAssembly(t, newerSdkVersion.String(), newerAssistantVersion)
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, newerSdkVersion.String(), newerAssembly)

	newestAssembly := createAssembly(t, newestSdkVersion, olderAssistantVersion)
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, newestSdkVersion, newestAssembly)

	t.Run("upgrades if sdk is newer", func(t *testing.T) {
		tmpDamlHome, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

		installAndVerifySdk(t, tmpDamlHome, sdkVersion, sdkVersion, olderAssistantVersion)
		installAndVerifySdk(t, tmpDamlHome, newerSdkVersion.String(), newerSdkVersion.String(), newerAssistantVersion)
	})

	t.Run("skips upgrading if sdk is older", func(t *testing.T) {
		tmpDamlHome, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

		installAndVerifySdk(t, tmpDamlHome, newerSdkVersion.String(), newerSdkVersion.String(), newerAssistantVersion)
		installAndVerifySdk(t, tmpDamlHome, sdkVersion, newerSdkVersion.String(), newerAssistantVersion)
	})

	t.Run("upgrades if sdk is newer regardless of its assistant version", func(t *testing.T) {
		tmpDamlHome, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

		installAndVerifySdk(t, tmpDamlHome, newerSdkVersion.String(), newerSdkVersion.String(), newerAssistantVersion)
		installAndVerifySdk(t, tmpDamlHome, newestSdkVersion, newestSdkVersion, olderAssistantVersion)
	})
}

func installAndVerifySdk(t *testing.T, damlHome, sdkVersionToInstall, expectedSdkVersion, expectedActiveAssistantVersion string) {
	cmd := createStdTestRootCmd(t)
	cmd.SetArgs([]string{"install", sdkVersionToInstall})
	require.NoError(t, cmd.Execute())
	assertSdkVersion(t, expectedSdkVersion)
	testMeepyComponent(t)
	t.Run("link assistant", verifyLink)
	verifyAssistantVersion(t, damlHome, expectedActiveAssistantVersion)
}

func createAssembly(t *testing.T, sdkVersion, assistantVersion string) (assemblyPath string) {
	a, err := sdkmanifest.ReadSdkManifest(testutil.TestdataPath(t, "remote-components.yaml"))
	require.NoError(t, err)
	sdkSemver, err := semver.NewVersion(sdkVersion)
	require.NoError(t, err)
	a.Spec.Version = sdkmanifest.AssemblySemVer(sdkSemver)
	assistantSemver, err := semver.NewVersion(assistantVersion)
	require.NoError(t, err)
	a.Spec.Assistant.Version = sdkmanifest.AssemblySemVer(assistantSemver)
	aBytes, err := yaml.Marshal(a)
	require.NoError(t, err)
	tmp, deleteFn, err := utils.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = deleteFn() })
	assemblyPath = filepath.Join(tmp, "sdk-manifest.yaml")
	err = os.WriteFile(assemblyPath, aBytes, 0666)
	require.NoError(t, err)

	return assemblyPath
}

func verifyAssistantVersion(t *testing.T, damlHome string, expectedVersion string) {
	ctx := testutil.Context(t)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	c := exec.CommandContext(ctx, activeAssistantPath(t, damlHome))
	c.Stdout = w
	c.Stderr = w

	require.NoError(t, c.Run())
	require.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(output), expectedVersion)
}

func copyAssistant(t *testing.T, srcBinDir string, newVersion string) string {
	tmp, deleteFn, err := utils.MkdirTemp("", "")
	t.Cleanup(func() { _ = deleteFn() })
	require.NoError(t, err)

	sourceFile, err := os.Open(filepath.Join(srcBinDir, assembler.AssistantBinName(testutil.OS)))
	require.NoError(t, err)
	defer sourceFile.Close()
	srcBytes, err := io.ReadAll(sourceFile)
	require.NoError(t, err)

	destinationFile, err := os.OpenFile(filepath.Join(tmp, assembler.AssistantBinName(testutil.OS)), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	defer destinationFile.Close()
	_, err = destinationFile.WriteString(strings.ReplaceAll(string(srcBytes), "4.5.6", newVersion))
	require.NoError(t, err)

	require.NoError(t, destinationFile.Sync())
	return tmp
}
