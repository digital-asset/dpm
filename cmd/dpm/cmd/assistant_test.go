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

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/cmd/dpm/cmd/versions"
	"daml.com/x/assistant/pkg/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
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

const someSdkVersion = "0.0.1-whatever"
const someOtherSdkVersion = "0.0.1-not-whatever"

func TestSuite(t *testing.T) {
	suite.Run(t, &MainSuite{})
}

func (suite *MainSuite) TestResolveMultiPackageRoot() {
	t := suite.T()

	installSdk(t, []string{someSdkVersion})
	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "another-daml-package"))
	testResolution(t, 1)
}

func (suite *MainSuite) TestResolveMultiPackageSubdir() {
	t := suite.T()

	installSdk(t, []string{someSdkVersion})

	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

	// this will make daml.yaml in the CWD
	require.NoError(t, os.Chdir(testutil.TestdataPath(t, "multi-package-with-subdir", "package")))
	testResolution(t, 1)
}

func (suite *MainSuite) TestResolveMultiPackageSdkVersion() {
	t := suite.T()

	installSdk(t, []string{someSdkVersion})

	t.Run("1a: when in multi-package dir", func(t *testing.T) {
		t.Chdir(testutil.TestdataPath(t, "multi-package-sdk-version"))

		testResolution(t, 3)
		assertActiveSdkVersion(t, someSdkVersion)
	})

	t.Run("1b: when in sub package dir", func(t *testing.T) {
		t.Chdir(testutil.TestdataPath(t, "multi-package-sdk-version", "main"))

		// test at level of single package
		assertActiveSdkVersion(t, someSdkVersion)
		testResolution(t, 3)
	})
}

func (suite *MainSuite) TestResolveMultiPackageSdkVersionWithOverrides() {
	t := suite.T()

	//t.Run("3a: when in multi-package dir", func(t *testing.T) {
	//	installSdk(t, []string{someSdkVersion})
	//	t.Chdir(testutil.TestdataPath(t, "multi-package-sdk-version-overrides"))
	//
	//	t.Run("help command", func(t *testing.T) {
	//		output := runHelpCommand(t)
	//		assert.Contains(t, output, "meep")
	//		assert.Contains(t, output, "javux")
	//		assert.NotContains(t, output, "multipak")
	//	})
	//
	//	t.Run("resolve command", func(t *testing.T) {
	//		deepResolution := runResolveCommand(t)
	//		assert.Len(t, deepResolution.Packages, 3)
	//		assert.ElementsMatch(t,
	//			lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
	//			[]string{"meep", "javabro"})
	//	})
	//
	//	t.Run("assert active sdk version", func(t *testing.T) {
	//		assertActiveSdkVersion(t, someSdkVersion)
	//	})
	//})

	t.Run("3b: when in sub package dir", func(t *testing.T) {
		installSdk(t, []string{someSdkVersion})
		t.Chdir(testutil.TestdataPath(t, "multi-package-sdk-version-overrides", "main"))

		t.Run("help command", func(t *testing.T) {
			output := runHelpCommand(t)
			assert.Contains(t, output, "meep")
			assert.Contains(t, output, "javux")
			assert.NotContains(t, output, "multipak")
		})

		t.Run("resolve command", func(t *testing.T) {
			deepResolution := runResolveCommand(t)
			assert.Len(t, deepResolution.Packages, 3)
			assert.ElementsMatch(t,
				lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
				[]string{"meep", "javabro"})
		})

		t.Run("assert active sdk version", func(t *testing.T) {
			assertActiveSdkVersion(t, someSdkVersion)
		})
	})
}

func (suite *MainSuite) TestResolveErrorsInResolutionFile() {
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

func (suite *MainSuite) TestResolveWithDpmSdkVersionEnvVar() {
	t := suite.T()
	ctx := testutil.Context(t)

	installSdk(t, []string{someSdkVersion})

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
		sdkVersion := someSdkVersion
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

func testResolution(t *testing.T, expectedPackages int) {
	deepResolution := runResolveCommand(t)
	assert.Len(t, deepResolution.Packages, expectedPackages)
	assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 1)
	assert.ElementsMatch(t,
		lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
		[]string{"meep"})
	assert.Len(t, lo.Values(deepResolution.Packages)[0].Imports, 2)
	assert.Equal(t, resolution.Kind, deepResolution.Kind)
	assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)

	t.Run("correct package paths", func(t *testing.T) {
		for pkgPath := range deepResolution.Packages {
			assert.True(t, filepath.IsAbs(pkgPath))
			_, err := os.ReadFile(filepath.Join(pkgPath, "daml.yaml"))
			require.NoError(t, err)
		}
	})

	t.Run("default sdk", func(t *testing.T) {
		assert.Len(t, deepResolution.DefaultSDK, 1)
		assert.Len(t, deepResolution.DefaultSDK[someSdkVersion].Components, 1)
		assert.Len(t, deepResolution.DefaultSDK[someSdkVersion].Imports, 2)
		assert.True(t, true)
	})

}

func runHelpCommand(t *testing.T) string {
	cmd, r, w := createTestRootCmd(t, "--help")
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(output)
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
		assert.Regexp(t, regexp.MustCompile(assistantconfig.DpmSdkVersionEnvVar+"=\"?"+someSdkVersion+"\"?"), output)
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
	assert.Contains(t, string(output), "version: unknown\nbuild: unknown\nbuildDate: unknown")
}

func (suite *MainSuite) TestSdkUnInstallCommand() {
	t := suite.T()

	sdkVersion := someSdkVersion
	installSdk(t, []string{sdkVersion})

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
		Name, Version, InstallArg string
	}{
		{
			"via semver", someSdkVersion, someSdkVersion,
		},
		{
			"via some other semver", someOtherSdkVersion, someOtherSdkVersion,
		},
		{
			"via latest tag", someOtherSdkVersion, "latest",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			installFloatySdk(t, c.Version, c.InstallArg)
		})
	}
}

func installFloatySdk(t *testing.T, version string, floatyTag string) {
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	// Push assembly for each version
	testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, version, testutil.TestdataPath(t, "remote-components.yaml"))

	// push assembly, assistant, and component
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	testutil.PushComponent(t, ctx, reg, sdkmanifest.AssistantName, "4.5.6", testutil.TestdataPath(t, "assistant-binary", testutil.OS))

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	// Install each version
	cmd, r, w := createTestRootCmd(t, "install", floatyTag)

	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(output), "Successfully installed SDK "+version)

	// verify installation for this version
	assertSdkVersion(t, version)

	// verify
	testMeepyComponent(t)
	t.Run("link assistant", verifyLink)
}

func installSdk(t *testing.T, versions []string) {
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	// Push assembly for each version
	for _, v := range versions {
		testutil.PushAssembly(t, ctx, sdkmanifest.OpenSource, reg, v, testutil.TestdataPath(t, "remote-components.yaml"))
	}

	// push assembly, assistant, and component
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	testutil.PushComponent(t, ctx, reg, sdkmanifest.AssistantName, "4.5.6", testutil.TestdataPath(t, "assistant-binary", testutil.OS))

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	// Install each version
	for _, version := range versions {
		version := version // capture

		t.Run("install_"+version, func(t *testing.T) {
			cmd, r, w := createTestRootCmd(t, "install", version)

			require.NoError(t, cmd.Execute())
			assert.NoError(t, w.Close())

			output, err := io.ReadAll(r)
			require.NoError(t, err)

			assert.Contains(t, string(output), "Successfully installed SDK "+version)

			// verify installation for this version
			assertSdkVersion(t, version)
		})
	}

	// verify
	testMeepyComponent(t)
	t.Run("link assistant", verifyLink)
}

func (suite *MainSuite) TestAutoInstallDefaultDisabled() {
	t := suite.T()
	ctx := testutil.Context(t)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	err = os.Chdir(testutil.TestdataPath(t, "daml-package", testutil.OS))
	if err != nil {
		return
	}

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err = RootCmd(ctx, da)
	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)
}

func (suite *MainSuite) TestHelpCommandUsesShallowResolution() {
	t := suite.T()
	installSdk(t, []string{someSdkVersion})

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
			tmpDir := t.TempDir()

			t.Setenv(assistantconfig.ResolutionFilePathEnvVar, filepath.Join(tmpDir, "deep.yaml"))
			cmd := createStdTestRootCmd(t, tc.Args...)
			require.NoError(t, cmd.Execute())

			_, err := os.ReadFile(filepath.Join(tmpDir, "deep.yaml"))
			require.ErrorIs(t, err, os.ErrNotExist)

		})
	}

}

func (suite *MainSuite) TestLegacyDamlProjectEnvVar() {
	t := suite.T()
	ctx := testutil.Context(t)

	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "daml-package", testutil.OS))

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err := RootCmd(ctx, da)
	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)
}

func (suite *MainSuite) TestLegacyDamlPackageEnvVar() {
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
	installSdk(t, []string{someSdkVersion})

	t.Run("single package", func(t *testing.T) {
		tmpDir := t.TempDir()

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
		tmpDir := t.TempDir()
		t.Setenv(assistantconfig.ResolutionFilePathEnvVar, filepath.Join(tmpDir, "deep.yaml"))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
		require.NoError(t, os.Chdir(testutil.TestdataPath(t, filepath.Join("multi-package-another"))))

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

func (suite *MainSuite) TestMultiPackageComponentOverrides() {
	t := suite.T()

	t.Run("13a: when in multi-package dir", func(t *testing.T) {
		installSdk(t, []string{someSdkVersion, someOtherSdkVersion})
		t.Chdir(testutil.TestdataPath(t, "multi-package-all-in-one", testutil.OS))

		t.Run("help command", func(t *testing.T) {
			output := runHelpCommand(t)
			assert.Contains(t, output, "meep")
			assert.Contains(t, output, "multipak")
			assert.NotContains(t, output, "javux")
		})

		t.Run("resolve command", func(t *testing.T) {
			deepResolution := runResolveCommand(t)
			assert.Len(t, deepResolution.Packages, 1)
			assert.ElementsMatch(t,
				lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
				[]string{"multipak", "meep", "javabro"})
		})

		t.Run("assert active sdk version", func(t *testing.T) {
			assertActiveSdkVersion(t, someSdkVersion)
		})

	})

	t.Run("13b: when in sub package dir", func(t *testing.T) {
		installSdk(t, []string{someSdkVersion, someOtherSdkVersion})
		t.Chdir(testutil.TestdataPath(t, "multi-package-all-in-one", testutil.OS, "daml-package"))

		t.Run("help command", func(t *testing.T) {
			output := runHelpCommand(t)
			assert.Contains(t, output, "meep")
			assert.Contains(t, output, "multipak")
			assert.Contains(t, output, "javux")
		})

		t.Run("resolve command", func(t *testing.T) {
			deepResolution := runResolveCommand(t)
			assert.Len(t, deepResolution.Packages, 1)
			assert.ElementsMatch(t,
				lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
				[]string{"multipak", "meep", "javabro"})
		})

		t.Run("assert active sdk version", func(t *testing.T) {
			assertActiveSdkVersion(t, someOtherSdkVersion)
		})
	})
}

func (suite *MainSuite) TestMultiPackageSdkAndComponentOverrides() {
	t := suite.T()

	installSdk(t, []string{someSdkVersion})

	deepResolution := runResolveCommand(t)
	assert.ElementsMatch(t,
		lo.Keys(lo.Values(deepResolution.DefaultSDK)[0].ComponentsV2),
		[]string{"meep"})

	t.Run("when in multi-package dir", func(t *testing.T) {
		t.Chdir(testutil.TestdataPath(t, "multi-package-all-in-one-with-null-package-sdk", testutil.OS))

		t.Run("help command", func(t *testing.T) {
			output := runHelpCommand(t)
			assert.Contains(t, output, "meep")
			assert.Contains(t, output, "multipak")
			assert.NotContains(t, output, "javux")
		})

		t.Run("resolve command", func(t *testing.T) {
			deepResolution := runResolveCommand(t)
			assert.Len(t, deepResolution.Packages, 1)
			assert.ElementsMatch(t,
				lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
				[]string{"multipak", "meep", "javabro"})
		})
	})

	t.Run("when in sub package dir", func(t *testing.T) {
		t.Chdir(testutil.TestdataPath(t, "multi-package-all-in-one-with-null-package-sdk", testutil.OS, "daml-package"))

		t.Run("help command", func(t *testing.T) {
			output := runHelpCommand(t)
			assert.Contains(t, output, "meep")
			assert.Contains(t, output, "multipak")
			assert.Contains(t, output, "javux")
		})

		t.Run("resolve command", func(t *testing.T) {
			deepResolution := runResolveCommand(t)
			assert.Len(t, deepResolution.Packages, 1)
			assert.ElementsMatch(t,
				lo.Keys(lo.Values(deepResolution.Packages)[0].ComponentsV2),
				[]string{"multipak", "meep", "javabro"})
		})
	})
}

func (suite *MainSuite) TestMultiPackageInstall() {
	t := suite.T()

	sdkVersion := someSdkVersion
	installSdk(t, []string{sdkVersion})

	t.Run("multi pkg no override", func(t *testing.T) {

		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
		require.NoError(t, os.Chdir(testutil.TestdataPath(t, filepath.Join("multi-package-another"))))
		t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDir)

		cmd, r, w := createTestRootCmd(t, "install", "package")
		require.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())

		output, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Contains(t, string(output), "Successfully installed SDK "+sdkVersion)
		assert.Contains(t, string(output), "No overrides to install")
	})
}

func (suite *MainSuite) TestMultiPackageSkip() {
	t := suite.T()

	tmpDir := t.TempDir()

	cwd, err := os.Getwd()

	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
	require.NoError(t, os.Chdir(testutil.TestdataPath(t, "multi-package-sdk-skipping", testutil.OS)))
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDir)

	cmd, r, w := createTestRootCmd(t, "install", "package", "--skip-sdk")

	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())
	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(output), "Skipping installation of multi-package")

	ctx := testutil.Context(t)
	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	_, err = RootCmd(ctx, da)

	require.ErrorIs(t, err, assistantconfig.ErrTargetSdkNotInstalled)

}

func (suite *MainSuite) TestSdkVersionCommand() {
	t := suite.T()
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	sdkVersions := []string{someSdkVersion, "2.0.0-alpha", "1.0.0", "1.0.1", "3.0.0", "1.1.0"}
	sorted := []string{
		"  0.0.1-whatever    ",
		"  1.0.0             ",
		"  1.0.1             ",
		"  1.1.0    (latest) ",
		"  2.0.0-alpha       ",
		"  3.0.0             ",
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

	t.Run("active sdk version err outside project when no sdk installed", func(t *testing.T) {
		assertNoActiveSdkVersion(t)
	})

	t.Run("active sdk version err in single package with null sdk-version", func(t *testing.T) {
		t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "null-sdk-version"))
		assertNoActiveSdkVersion(t)
	})

	t.Run("active sdk version outside project", func(t *testing.T) {
		installSdk(t, []string{someSdkVersion})
		assertActiveSdkVersion(t, someSdkVersion)
	})

	t.Run("active sdk version from env var not installed", func(t *testing.T) {
		t.Setenv(assistantconfig.DpmSdkVersionEnvVar, "1.2.3-nonexistent")
		assertActiveSdkVersion(t, "1.2.3-nonexistent")
	})

	t.Run("active sdk version from daml.yaml not installed", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Setenv(assistantconfig.DamlPackageEnvVar, tmpDir)
		err = os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(`sdk-version: 1.2.3-not-installed`), 0666)
		require.NoError(t, err)
		assertActiveSdkVersion(t, "1.2.3-not-installed")
	})

	t.Run("active sdk version of multi-package", func(t *testing.T) {
		tmpDir := t.TempDir()

		err = os.WriteFile(filepath.Join(tmpDir, "multi-package.yaml"), []byte(`sdk-version: 1.2.3-multi-package`), 0666)
		require.NoError(t, err)

		t.Chdir(tmpDir)
		assertActiveSdkVersion(t, "1.2.3-multi-package")

		t.Run("active sdk version of package takes precedence over multi-package sdk version", func(t *testing.T) {
			subPackageDir := filepath.Join(tmpDir, "sub-package")
			require.NoError(t, utils.EnsureDirs(subPackageDir))
			require.NoError(t, os.WriteFile(
				filepath.Join(subPackageDir, "daml.yaml"), []byte(`sdk-version: 4.5.6-not-installed`),
				0666))

			t.Chdir(subPackageDir)
			assertActiveSdkVersion(t, "4.5.6-not-installed")
		})

		t.Run("active sdk version of package with null version inherits multi-package version", func(t *testing.T) {
			subPackageDir := filepath.Join(tmpDir, "sub-package")
			require.NoError(t, os.RemoveAll(filepath.Join(tmpDir, "sub-package")))

			require.NoError(t, utils.EnsureDirs(subPackageDir))
			require.NoError(t, os.WriteFile(
				filepath.Join(subPackageDir, "daml.yaml"), []byte(`sdk-version: null`),
				0666))

			t.Chdir(subPackageDir)
			assertActiveSdkVersion(t, "1.2.3-multi-package")
		})
	})

	t.Run("active sdk version of multi-package with null sdk defaults to global", func(t *testing.T) {
		tmpDir := t.TempDir()

		err = os.WriteFile(filepath.Join(tmpDir, "multi-package.yaml"), []byte(`sdk-version: null`), 0666)
		require.NoError(t, err)

		t.Chdir(tmpDir)
		assertNoActiveSdkVersion(t)
		installSdk(t, []string{someSdkVersion})
		assertActiveSdkVersion(t, someSdkVersion)
	})
}

func assertNoActiveSdkVersion(t *testing.T) {
	cmd := createStdTestRootCmd(t, string(builtincommand.Version), "--active")
	require.ErrorIs(t, cmd.Execute(), versions.ErrNoActiveSdk)
}

func assertActiveSdkVersion(t *testing.T, version string) {
	cmd, r, w := createTestRootCmd(t, string(builtincommand.Version), "--active")
	err := cmd.Execute()
	assert.NoError(t, w.Close())
	require.NoError(t, err)

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, version+"\n", string(output))
}

func (suite *MainSuite) TestNullableSdkVersionInDamlYaml() {
	t := suite.T()

	t.Setenv(assistantconfig.DamlProjectEnvVar, testutil.TestdataPath(t, "null-sdk-version"))

	output := runHelpCommand(t)
	assert.Contains(t, output, "hello-from-null-sdk-daml-yaml")

	t.Run("resolution", func(t *testing.T) {
		deepResolution := runResolveCommand(t)
		assert.Len(t, deepResolution.Packages, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Imports, -0)
		assert.Equal(t, resolution.Kind, deepResolution.Kind)
		assert.Equal(t, resolution.ApiVersion, deepResolution.APIVersion)

		t.Run("correct package paths", func(t *testing.T) {
			for pkgPath := range deepResolution.Packages {
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

	output := runHelpCommand(t)
	assert.Contains(t, output, "hello-from-null-sdk-daml-yaml")

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

			for pkgPath := range deepResolution.Packages {
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

	installSdk(t, []string{someSdkVersion})

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
		err := os.Setenv("HOME", home)
		if err != nil {
			return
		}
	})

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd := createStdTestRootCmd(t)
	require.NoError(t, cmd.Execute())
}

func (suite *MainSuite) TestComponentInit() {
	t := suite.T()
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	cmd, _, w := createTestRootCmd(t, "component", "init")
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	assert.FileExists(t, "daml.yaml")
	assert.FileExists(t, "component.yaml")

}

func (suite *MainSuite) TestComponentInitFail() {
	t := suite.T()
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(``), 0666)
	_ = os.WriteFile(filepath.Join(tmpDir, "component.yaml"), []byte(``), 0666)

	t.Chdir(tmpDir)

	cmd, _, _ := createTestRootCmd(t, "component", "init")

	require.Error(t, cmd.Execute())

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
		_ = r.Close()
		_ = w.Close()
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
