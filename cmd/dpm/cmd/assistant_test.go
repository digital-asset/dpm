// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"daml.com/x/assistant/cmd/dpm/cmd/versions"
	"daml.com/x/assistant/pkg/builtincommand"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry/remote"
)

type MainSuite struct {
	testutil.CommonSetupSuite
}

func TestSuite(t *testing.T) {
	suite.Run(t, &MainSuite{})
}

func (suite *MainSuite) TestResolveCommand() {
	t := suite.T()

	installSdk(t, "0.0.1-whatever")
	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "another-daml-package"))
	testResolution(t)
}

func (suite *MainSuite) TestResolutionWithChangedCwd() {
	t := suite.T()

	installSdk(t, "0.0.1-whatever")

	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

	// this will make daml.yaml in the CWD
	require.NoError(t, os.Chdir(testutil.TestdataPath(t, "multi-package-with-subdir", "package")))
	testResolution(t)
}

func testResolution(t *testing.T) {
	deepResolution := runResolveCommand(t)
	assert.Len(t, deepResolution.Packages, 1)
	assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 1)
	assert.Len(t, lo.Values(deepResolution.Packages)[0].Imports, 2)
	assert.Equal(t, resolution.Kind, deepResolution.Kind)
	assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)

	t.Run("correct package paths", func(t *testing.T) {
		for pkgPath, _ := range deepResolution.Packages {
			assert.True(t, filepath.IsAbs(pkgPath))
			_, err := os.ReadFile(filepath.Join(pkgPath, "daml.yaml"))
			require.NoError(t, err)
		}
	})

	t.Run("default sdk", func(t *testing.T) {
		assert.Len(t, deepResolution.DefaultSDK, 1)
		assert.Len(t, deepResolution.DefaultSDK["0.0.1-whatever"].Components, 1)
		assert.Len(t, deepResolution.DefaultSDK["0.0.1-whatever"].Imports, 2)
		assert.True(t, true)
	})

}

func runResolveCommand(t *testing.T) *resolution.Resolution {
	cmd, r, w := createTestRootCmd(t, "resolve")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)

	deepResolution := resolution.Resolution{}
	require.NoError(t, yaml.Unmarshal(output, &deepResolution))
	return &deepResolution
}

func (suite *MainSuite) TestResolutionWithDpmSdkVersionEnvVar() {
	t := suite.T()
	ctx := testutil.Context(t)

	installSdk(t, "0.0.1-whatever")

	// prepare and install another sdk
	_, reg := testutil.StartRegistry(t)
	anotherSdkVersion := "1.2.3"
	anotherSdkAssembly := createAssembly(t, anotherSdkVersion, "4.5.6")
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, anotherSdkVersion, anotherSdkAssembly)
	cmd := createStdTestRootCmd(t, "install", anotherSdkVersion)
	require.NoError(t, cmd.Execute())

	cmd = createStdTestRootCmd(t, "resolve")
	require.NoError(t, cmd.Execute())

	t.Run("no override", func(t *testing.T) {
		sdkVersion := "1.2.3"
		deepRes := runResolveCommand(t)
		assert.Len(t, deepRes.DefaultSDK, 1)
		assert.Contains(t, deepRes.DefaultSDK, sdkVersion)
		assert.Empty(t, deepRes.DefaultSDK[sdkVersion].Errors)
	})

	t.Run("good override", func(t *testing.T) {
		sdkVersion := "0.0.1-whatever"
		t.Setenv(assistantconfig.DpmSdkVersionEnvVar, sdkVersion)

		deepRes := runResolveCommand(t)
		assert.Len(t, deepRes.DefaultSDK, 1)
		assert.Contains(t, deepRes.DefaultSDK, sdkVersion)
		assert.Len(t, deepRes.DefaultSDK[sdkVersion].Components, 1)
		assert.Empty(t, deepRes.DefaultSDK[sdkVersion].Errors)
	})

	t.Run("bad override", func(t *testing.T) {
		sdkVersion := "1.2.3-non-existent"
		t.Setenv(assistantconfig.DpmSdkVersionEnvVar, sdkVersion)

		deepRes := runResolveCommand(t)
		assert.Len(t, deepRes.DefaultSDK, 1)
		assert.Contains(t, deepRes.DefaultSDK, sdkVersion)
		assert.Empty(t, deepRes.DefaultSDK[sdkVersion].Components)
		assert.NotEmpty(t, deepRes.DefaultSDK[sdkVersion].Errors)
	})
}

func (suite *MainSuite) TestErrorsInResolutionFile() {
	t := suite.T()

	testCases := []struct {
		damlPackagePath   string
		expectedErrorCode string
	}{
		{
			damlPackagePath:   testutil.TestdataPath(t, "invalid-daml-package"),
			expectedErrorCode: resolutionerrors.MalformedDamlYaml,
		},
		{
			damlPackagePath:   testutil.TestdataPath(t, "literally-a-cat-picture"),
			expectedErrorCode: resolutionerrors.MalformedDamlYaml,
		},
		{
			damlPackagePath:   testutil.TestdataPath(t, "this is very likely not a correct path"),
			expectedErrorCode: resolutionerrors.DamlYamlNotFound,
		},
		{
			damlPackagePath:   testutil.TestdataPath(t, "another-daml-package"),
			expectedErrorCode: resolutionerrors.SdkNotInstalled,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.expectedErrorCode, func(t *testing.T) {
			t.Setenv(assistantconfig.DamlProjectEnvVar, testCase.damlPackagePath)

			cmd, r, w := createTestRootCmd(t, "resolve")
			assert.NoError(t, cmd.Execute())
			assert.NoError(t, w.Close())

			output, err := io.ReadAll(r)
			assert.NoError(t, err)

			deepResolution := resolution.Resolution{}
			require.NoError(t, yaml.Unmarshal(output, &deepResolution))

			require.Len(t, deepResolution.Packages, 1)
			pkg := deepResolution.Packages[testCase.damlPackagePath]
			require.NotNil(t, pkg)
			require.NotNil(t, pkg.Errors)
			assert.Equal(t, pkg.Errors[0].Code, testCase.expectedErrorCode)
		})
	}
}

func (suite *MainSuite) TestSdkCommandsFlagParsing() {
	t := suite.T()
	t.Setenv("DPM_ASSEMBLY", testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))
	testMeepyComponent(t)
}

func testMeepyComponent(t *testing.T) {
	cmd, r, w := createTestRootCmd(t, "meep", "--some-flag")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "meep meep! --some-flag")
}

func (suite *MainSuite) TestInjectedEnvVars() {
	t := suite.T()
	if testutil.OS == "windows" {
		t.Skip("this test hates windows")
		return
	}
	t.Setenv("DPM_ASSEMBLY", testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))
	cmd, r, w := createTestRootCmd(t, "show-env-vars", "--some-flag")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)

	t.Run("resolution file path", func(t *testing.T) {
		assert.Regexp(t, regexp.MustCompile(assistantconfig.ResolutionFilePathEnvVar+"=\"?[\\w/._-]+[.]yaml\"?"), output)
	})

	t.Run("sdk version", func(t *testing.T) {
		assert.Regexp(t, regexp.MustCompile(assistantconfig.DpmSdkVersionEnvVar+"=\"?0.0.1-whatever\"?"), output)
	})
}

func (suite *MainSuite) TestJavaCommand() {
	t := suite.T()
	t.Setenv("DPM_ASSEMBLY", testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))
	// put mock `java` in PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", testutil.TestdataPath(t, "fake-java", testutil.OS)+string(os.PathListSeparator)+oldPath)
	cmd, r, w := createTestRootCmd(t, "javux", "--some-flag")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "i am a fake java!")
	assert.Contains(t, string(output), "fake.jar banananas --some-flag")
}

func (suite *MainSuite) TestSdkCommandsLoading() {
	t := suite.T()
	t.Setenv("DPM_ASSEMBLY", testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))
	testcases := []struct {
		Name    string
		CmdArgs []string
	}{
		{"Loads SDK commands with --help flag", []string{"--help"}},
		{"Loads SDK commands with no args", []string{}},
	}

	for _, testcase := range testcases {
		tc := testcase
		t.Run(tc.Name, func(t *testing.T) {
			cmd, r, w := createTestRootCmd(t, tc.CmdArgs...)
			assert.NoError(t, cmd.Execute())
			assert.NoError(t, w.Close())

			output, err := io.ReadAll(r)
			assert.NoError(t, err)
			assert.Contains(t, string(output), "meep")
			assert.Contains(t, string(output), "javux")
			assert.Contains(t, string(output), "needy")

			sdkCommands := lo.Filter(cmd.Commands(), func(subCmd *cobra.Command, _ int) bool {
				return subCmd.GroupID == sdkGroupId
			})
			assert.Len(t, sdkCommands, 4)
		})
	}
}

func (suite *MainSuite) TestComponentDependencyPaths() {
	t := suite.T()
	t.Setenv("DPM_ASSEMBLY", testutil.TestdataPath(t, "local-with-java", testutil.OS, "sdk-manifest.yaml"))

	cmd, r, w := createTestRootCmd(t, "needy")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "meep meep!")
}

func (suite *MainSuite) TestComponentPublish() {
	t := suite.T()
	ctx := testutil.Context(t)
	client, _ := testutil.StartRegistry(t)

	publish := func(t *testing.T, version string) {
		args := []string{"repo", "publish-component", "meepy-repo", version,
			"-p", "generic=" + testutil.TestdataPath(t, "meepy-component", testutil.OS),
			"-t", "latest",
		}
		cmd, _, w := createTestRootCmd(t, appendRegistryArgsFromEnv(args)...)
		assert.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())

		reg, err := remote.NewRegistry(client.Registry)
		require.NoError(t, err)
		_, err = reg.Repository(ctx, ociconsts.ComponentRepoPrefix+"meepy-repo")
		assert.NoError(t, err)
	}

	t.Run("publish", func(t *testing.T) {
		publish(t, "1.2.3")
	})

	t.Run("overwrite latest tag", func(t *testing.T) {
		publish(t, "4.5.6")
	})
}

func (suite *MainSuite) TestComponentPublishDryRun() {
	t := suite.T()
	args := []string{"repo", "publish-component", "--dry-run", "meep", "1.2.3-meep",
		"-p", "generic=" + testutil.TestdataPath(t, "meepy-component", testutil.OS)}
	cmd, r, w := createTestRootCmd(t, args...)
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "")
}

func (suite *MainSuite) TestAssistantVersionCommand() {
	t := suite.T()
	cmd, r, w := createTestRootCmd(t, "--version")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "version: unknown")
}

func (suite *MainSuite) TestSdkUnInstallCommand() {
	t := suite.T()

	sdkVersion := "0.0.1-whatever"
	installSdk(t, sdkVersion)

	cmd := createStdTestRootCmd(t, "--help")
	require.NoError(t, cmd.Execute())
	before := len(cmd.Commands())

	cmd = createStdTestRootCmd(t)
	cmd.SetArgs([]string{string(builtincommand.UnInstall), sdkVersion})
	require.NoError(t, cmd.Execute())

	cmd = createStdTestRootCmd(t, "--help")
	require.NoError(t, cmd.Execute())
	after := len(cmd.Commands())

	assert.Less(t, after, before, "expected fewer commands in help after uninstall")

	cmd, r, w := createTestRootCmd(t, "dpm", string(builtincommand.Versions))
	cmd.SetArgs([]string{string(builtincommand.Versions)})
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.NotContains(t, string(output), sdkVersion)
}

func (suite *MainSuite) TestSdkInstallCommand() {
	t := suite.T()

	cases := []struct {
		Name, InstallArg string
	}{
		{
			"via semver", "0.0.1-whatever",
		},
		{
			"via latest tag", "latest",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			installSdk(t, c.InstallArg)
		})
	}
}

func installSdk(t *testing.T, installArg string) {
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	sdkVersion := "0.0.1-whatever"

	// push assembly, assistant, and component
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, sdkVersion, testutil.TestdataPath(t, "remote-components.yaml"))
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	testutil.PushComponent(t, ctx, reg, sdkmanifest.AssistantName, "4.5.6", testutil.TestdataPath(t, "assistant-binary", testutil.OS))

	tmpDamlHome, deleteFn, err := utils.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = deleteFn()
	})
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd, r, w := createTestRootCmd(t, "install", installArg)
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(output), "Successfully installed SDK "+sdkVersion)

	// verify
	assertSdkVersion(t, sdkVersion)
	testMeepyComponent(t)
	t.Run("link assistant", verifyLink)
}

func (suite *MainSuite) TestAutoInstallDefaultDisabled() {
	t := suite.T()
	ctx := testutil.Context(t)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	os.Chdir(testutil.TestdataPath(t, "daml-package", testutil.OS))

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err = RootCmd(ctx, da)
	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)
}

func (suite *MainSuite) TestHelpCommandUsesShallowResolution() {
	t := suite.T()
	installSdk(t, "0.0.1-whatever")

	testcases := []struct {
		Name string
		Args []string
	}{
		{Name: "no args", Args: []string{}},
		{Name: "with -h", Args: []string{"-h"}},
		{Name: "with --help", Args: []string{"--help"}},
		{Name: "with help", Args: []string{"help"}},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			tmpDir, deleteFn, err := utils.MkdirTemp("", "")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = deleteFn()
			})

			t.Setenv(assistantconfig.ResolutionFilePathEnvVar, filepath.Join(tmpDir, "deep.yaml"))
			cmd := createStdTestRootCmd(t, tc.Args...)
			require.NoError(t, cmd.Execute())

			_, err = os.ReadFile(filepath.Join(tmpDir, "deep.yaml"))
			require.ErrorIs(t, err, os.ErrNotExist)

		})
	}

}

func (suite *MainSuite) TestDamlProjectEnvVar() {
	t := suite.T()
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "daml-package", testutil.OS))

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err := RootCmd(ctx, da)
	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)
}

func (suite *MainSuite) TestDamlPackageEnvVar() {
	t := suite.T()
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.DamlPackageEnvVar, testutil.TestdataPath(t, "daml-package", testutil.OS))

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err := RootCmd(ctx, da)
	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)
}

func (suite *MainSuite) TestDeepResolutionForSdkCommands() {
	t := suite.T()
	t.Run("using DAML_PACKAGE", func(t *testing.T) {
		testDeepResolutionForSdkCommands(t, assistantconfig.DamlPackageEnvVar)
	})
	t.Run("using DAML_PROJECT", func(t *testing.T) {
		testDeepResolutionForSdkCommands(t, assistantconfig.DamlProjectEnvVar)
	})
}

func testDeepResolutionForSdkCommands(t *testing.T, damlPackageEnvVar string) {
	installSdk(t, "0.0.1-whatever")

	t.Run("single package", func(t *testing.T) {
		tmpDir, deleteFn, err := utils.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = deleteFn()
		})

		t.Setenv(damlPackageEnvVar, testutil.TestdataPath(t, "another-daml-package"))
		t.Setenv(assistantconfig.ResolutionFilePathEnvVar, filepath.Join(tmpDir, "deep.yaml"))
		cmd := createStdTestRootCmd(t, "meep")
		require.NoError(t, cmd.Execute())

		bytes, err := os.ReadFile(filepath.Join(tmpDir, "deep.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "another-daml-package")

		deepResolution := &resolution.Resolution{}
		require.NoError(t, yaml.Unmarshal(bytes, deepResolution))
		assert.Equal(t, resolution.Kind, deepResolution.Kind)
		assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)
	})

	t.Run("multi package", func(t *testing.T) {
		tmpDir, deleteFn, err := utils.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = deleteFn()
		})
		t.Setenv(assistantconfig.ResolutionFilePathEnvVar, filepath.Join(tmpDir, "deep.yaml"))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
		require.NoError(t, os.Chdir(testutil.TestdataPath(t, filepath.Join("another-multi-package"))))

		t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "another-daml-package"))

		cmd := createStdTestRootCmd(t, "meep")
		require.NoError(t, cmd.Execute())

		bytes, err := os.ReadFile(filepath.Join(tmpDir, "deep.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "another-daml-package")

		deepResolution := &resolution.Resolution{}
		require.NoError(t, yaml.Unmarshal(bytes, deepResolution))
		assert.Equal(t, resolution.Kind, deepResolution.Kind)
		assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)
	})
}

func (suite *MainSuite) TestListSDKVersion() {
	t := suite.T()
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	sdkVersions := []string{"0.0.1-whatever", "2.0.0-alpha", "1.0.0", "1.0.1", "3.0.0", "1.1.0"}
	sorted := []string{
		"  0.0.1-whatever ",
		"  1.0.0          ",
		"  1.0.1          ",
		"  1.1.0          ",
		"  2.0.0-alpha    ",
		"  3.0.0          ",
	}
	lo.ForEach(sdkVersions, func(v string, _ int) {
		testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, v, testutil.TestdataPath(t, "remote-components.yaml"))
	})

	cmd, r, w := createTestRootCmd(t, string(builtincommand.Versions), "--all")
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, strings.Join(sorted, "\n")+"\n", string(output))

	t.Run("active sdk version err when no sdk installed", func(t *testing.T) {
		assertActiveSdkVersion(t, "")
	})

	t.Run("active sdk version err when package sdk-version null", func(t *testing.T) {
		assertActiveSdkVersion(t, "")
	})

	t.Run("active sdk version", func(t *testing.T) {
		installSdk(t, "0.0.1-whatever")
		assertActiveSdkVersion(t, "0.0.1-whatever")
	})

	t.Run("active sdk version from env var not installed", func(t *testing.T) {
		t.Setenv(assistantconfig.DpmSdkVersionEnvVar, "1.2.3-nonexistent")
		assertActiveSdkVersion(t, "1.2.3-nonexistent")
	})

	t.Run("active sdk version from daml yaml not installed", func(t *testing.T) {
		tmpDir, deleteFn, err := utils.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = deleteFn()
		})

		t.Setenv(assistantconfig.DamlPackageEnvVar, tmpDir)
		err = os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(`sdk-version: 1.2.3-not-installed`), 0666)
		require.NoError(t, err)
		assertActiveSdkVersion(t, "1.2.3-not-installed")
	})
}

func assertActiveSdkVersion(t *testing.T, version string) {
	cmd, r, w := createTestRootCmd(t, string(builtincommand.Version), "--active")
	err := cmd.Execute()
	assert.NoError(t, w.Close())

	if version == "" {
		require.ErrorIs(t, err, versions.ErrNoActiveSdk)
		return
	}
	require.NoError(t, err)

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, version+"\n", string(output))
}

func (suite *MainSuite) TestNullableSdkVersionInDamlYaml() {
	t := suite.T()

	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "null-sdk-version"))

	cmd, r, w := createTestRootCmd(t, "--help")
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(output), "hello-from-null-sdk-daml-yaml")

	t.Run("resolution", func(t *testing.T) {
		deepResolution := runResolveCommand(t)
		assert.Len(t, deepResolution.Packages, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Imports, -0)
		assert.Equal(t, resolution.Kind, deepResolution.Kind)
		assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)

		t.Run("correct package paths", func(t *testing.T) {
			for pkgPath, _ := range deepResolution.Packages {
				assert.True(t, filepath.IsAbs(pkgPath))
				_, err := os.ReadFile(filepath.Join(pkgPath, "daml.yaml"))
				require.NoError(t, err)
			}
		})

		t.Run("default sdk", func(t *testing.T) {
			assert.Len(t, deepResolution.DefaultSDK, 1)
			assert.NotNil(t, deepResolution.DefaultSDK["unknown–sdk-version"].Errors)
			assert.Nil(t, deepResolution.DefaultSDK["unknown–sdk-version"].Components)
			assert.Nil(t, deepResolution.DefaultSDK["unknown–sdk-version"].Imports)
		})
	})

}

func (suite *MainSuite) TestResolutionOfSymlinkPackages() {
	t := suite.T()

	symlink := testutil.TestdataPath(t, "symlinked-package")
	resolvedSymlink := testutil.TestdataPath(t, "null-sdk-version")

	require.NoError(t, os.Symlink(resolvedSymlink, symlink))
	t.Cleanup(func() { assert.NoError(t, os.Remove(symlink)) })

	t.Setenv(assistantconfig.DamlProjectEnvVar, symlink)

	cmd, r, w := createTestRootCmd(t, "--help")
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(output), "hello-from-null-sdk-daml-yaml")

	t.Run("resolution", func(t *testing.T) {
		deepResolution := runResolveCommand(t)
		assert.Len(t, deepResolution.Packages, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Imports, -0)
		assert.Equal(t, resolution.Kind, deepResolution.Kind)
		assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)

		t.Run("correct package paths", func(t *testing.T) {
			assert.NotContains(t, deepResolution.Packages, symlink)
			assert.Contains(t, deepResolution.Packages, resolvedSymlink)

			for pkgPath, _ := range deepResolution.Packages {
				assert.True(t, filepath.IsAbs(pkgPath))
				_, err := os.ReadFile(filepath.Join(pkgPath, "daml.yaml"))
				require.NoError(t, err)
			}
		})
	})
}

func (suite *MainSuite) TestComponentSubdirFilesPerm() {
	t := suite.T()
	// chmod here because w bits don't get preserved by git
	p := testutil.TestdataPath(t, "meepy-component", testutil.OS, "just-a-dir", "xyz")
	f, err := os.Stat(p)
	require.NoError(t, err)
	oldMode := f.Mode()
	t.Cleanup(func() {
		_ = os.Chmod(p, oldMode)
	})

	mode := os.FileMode(0777)
	if testutil.OS == "windows" {
		mode = os.FileMode(0444)
	}

	err = os.Chmod(p, mode)
	require.NoError(t, err)

	installSdk(t, "latest")

	c, err := assistantconfig.Get()
	require.NoError(t, err)
	s, err := os.Stat(filepath.Join(c.CachePath, "components", "meep", "1.2.3", "just-a-dir", "xyz"))
	require.NoError(t, err)

	assert.Equal(t, mode, s.Mode())
}

func (suite *MainSuite) TestNoHomeRequired() {
	t := suite.T()
	home := os.Getenv("HOME")
	require.NoError(t, os.Unsetenv("HOME"))
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	tmpDamlHome, deleteFn, err := utils.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = deleteFn()
	})
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd := createStdTestRootCmd(t)
	require.NoError(t, cmd.Execute())
}

func assertSdkVersion(t *testing.T, sdkVersion string) {
	cmd, r, w := createTestRootCmd(t, "versions")
	assert.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), sdkVersion)
}

func createTestRootCmd(t *testing.T, args ...string) (rootCmd *cobra.Command, r *os.File, w *os.File) {
	ctx := testutil.Context(t)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	da := assistant.DamlAssistant{
		Stderr: w,
		Stdout: w,
		Stdin:  nil,
		ExitFn: func(exitCode int) {
			assert.Equal(t, 0, exitCode)
		},
		OsArgs: append([]string{DpmName}, args...),
	}

	rootCmd, err = RootCmd(ctx, &da)
	require.NoError(t, err)

	return
}

func createStdTestRootCmd(t *testing.T, args ...string) (rootCmd *cobra.Command) {
	ctx := testutil.Context(t)

	da := assistant.DamlAssistant{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
		Stdin:  nil,
		ExitFn: func(exitCode int) {
			assert.Equal(t, 0, exitCode)
		},
		OsArgs: append([]string{DpmName}, args...),
	}

	var err error
	rootCmd, err = RootCmd(ctx, &da)
	require.NoError(t, err)

	return
}

func appendRegistryArgsFromEnv(args []string) []string {
	args = append(args, "--registry", os.Getenv(assistantconfig.OciRegistryEnvVar))
	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}
	return args
}
