// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	root "daml.com/x/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/licenseutils"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RepoSuite struct {
	testutil.CommonSetupSuite
}

func TestRepoSuite(t *testing.T) {
	suite.Run(t, &RepoSuite{})
}

func (suite *RepoSuite) TestRepoCreateTarball() {
	t := suite.T()
	// the commands under test should work fully without requiring Edition env var to be set
	os.Unsetenv(assistantconfig.EditionEnvVar)
	testutil.StartRegistry(t)
	bundlePath, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	const sdkVersion = "0.0.1-whatever"

	publishComponents(t)

	t.Run("bundle creation", func(t *testing.T) {
		cmd, _, w := createTestRootCmd(t)

		args := []string{"repo", "create-tarball", "-o", bundlePath,
			"-f", testutil.TestdataPath(t, "publish.yaml")}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())
	})

	t.Run("LICENSES file", func(t *testing.T) {
		entries, err := os.ReadDir(bundlePath)
		require.NoError(t, err)
		for _, platformBundle := range entries {
			expected, err := os.ReadFile(testutil.TestdataPath(t, "licenses-for-publish-yaml-tarballs", "unix.golden"))
			if platformBundle.Name() == "windows-amd64" {
				expected, err = os.ReadFile(testutil.TestdataPath(t, "licenses-for-publish-yaml-tarballs", "windows.golden"))
			}
			require.NoError(t, err)

			got, err := os.ReadFile(filepath.Join(bundlePath, platformBundle.Name(), licenseutils.TarballLicensesFilename))
			require.NoError(t, err)
			assert.Equal(t, string(expected), string(got))
		}
	})

	t.Run("bootstrap from bundle", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		// TODO this command should refuse to bootstrap a bundle
		// that doesn't match machine's platform

		platformBundlePath := filepath.Join(bundlePath, runtime.GOOS+"-"+runtime.GOARCH)
		cmd.SetArgs([]string{"bootstrap", platformBundlePath})
		require.NoError(t, cmd.Execute())

		// verify
		assertSdkVersion(t, sdkVersion)
		testMeepyComponent(t)
		t.Run("link assistant", verifyLink)
		t.Run("verify assistant link at bundle root", func(t *testing.T) {
			dir := filepath.Join(platformBundlePath, "bin")
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			verifyLnkAtPath(t, filepath.Join(dir, entries[0].Name()))
		})
		t.Run("verify LICENSE file", func(t *testing.T) {
			licenseFile := filepath.Join(tmpDamlHome, "cache", "components", "dpm", "4.5.6", "LICENSE")
			bytes, err := os.ReadFile(licenseFile)
			require.NoError(t, err)
			assert.Equal(t, bytes, root.License)
		})
	})

	t.Run("test multi-platform sdk", func(t *testing.T) {
		testPlatformSpecificSdk(t, tmpDamlHome)
	})
}

func (suite *RepoSuite) TestRepoPublishAssembly() {
	t := suite.T()

	client, _ := testutil.StartRegistry(t)

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	const sdkVersion = "0.0.1-whatever"

	publishComponents(t)

	t.Run("publish sdk manifest", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)

		args := []string{"repo", "publish-sdk-manifest", "-t", "latest",
			"-f", testutil.TestdataPath(t, "publish.yaml")}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})

	t.Run("verify latest tag", func(t *testing.T) {
		repo, err := sdkmanifest.OpenSource.SdkManifestsRepo()
		require.NoError(t, err)
		tags, _, err := ocilister.ListTags(testutil.Context(t), client, repo)
		require.NoError(t, err)
		assert.Contains(t, tags, "latest")
	})

	t.Run("install published sdk manifest", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		cmd.SetArgs([]string{"install", sdkVersion})
		require.NoError(t, cmd.Execute())

		// verify
		assertSdkVersion(t, sdkVersion)
		testMeepyComponent(t)
		t.Run("link assistant", verifyLink)
	})

	t.Run("test multi-platform sdk", func(t *testing.T) {
		testPlatformSpecificSdk(t, tmpDamlHome)
	})
}

func testPlatformSpecificSdk(t *testing.T, damlHome string) {
	// "no-windows" component should've only been installed on darwin and linux

	config, err := assistantconfig.GetWithCustomDamlHome(damlHome)
	require.NoError(t, err)

	dirEntries, err := os.ReadDir(filepath.Join(config.CachePath, "components"))
	require.NoError(t, err)

	components := lo.Map(dirEntries, func(d os.DirEntry, _ int) string {
		return d.Name()
	})

	assert.Contains(t, components, "meep")

	shouldHaveMeepyNoWindowsComponent := runtime.GOOS != "windows"
	assert.Equal(t, shouldHaveMeepyNoWindowsComponent, lo.Contains(components, "no-windows"))
}

func (suite *RepoSuite) TestResolveLatest() {
	t := suite.T()
	testutil.StartRegistry(t)

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	publishComponents(t, "latest", "main")

	cases := []struct{ tag, file string }{
		{tag: "latest", file: "publish-with-latest.yaml"},
		{tag: "main", file: "publish-with-tags.yaml"},
	}

	for _, c := range cases {
		cmd, r, w := createTestRootCmd(t)

		args := []string{"repo", "resolve-tags", "--from-publish-config", testutil.TestdataPath(t, c.file)}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		assert.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())

		output, err := io.ReadAll(r)
		assert.NoError(t, err)
		assert.NotContains(t, string(output), c.tag)
		assert.Contains(t, string(output), "version: 1.2.3")
	}
}

func (suite *RepoSuite) TestPromoteComponents() {
	t := suite.T()

	testutil.StartRegistry(t)
	sourceRegistry := os.Getenv(assistantconfig.OciRegistryEnvVar)

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	publishComponents(t)

	testutil.StartRegistry(t)
	destinationRegistry := os.Getenv(assistantconfig.OciRegistryEnvVar)

	tmpDamlHome, err = os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd := createStdTestRootCmd(t)
	args := []string{
		"repo", "promote-components",
		"-f", testutil.TestdataPath(t, "publish.yaml"),
		"--source-registry", sourceRegistry,
		"--destination-registry", destinationRegistry,
	}
	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())

	for _, c := range []string{"meep", "dpm", "javabro"} {
		// TODO 'promote-components' command doesn't yet handle extra-tags
		expected := lo.Filter(listTags(t, c, sourceRegistry), func(tag string, _ int) bool {
			return tag != "latest"
		})
		got := listTags(t, c, destinationRegistry)
		assert.Equal(t, expected, got)
	}
}

func listTags(t *testing.T, component, registry string) []string {
	t.Setenv(assistantconfig.OciRegistryEnvVar, registry)

	cmd, r, w := createTestRootCmd(t)
	cmd.SetArgs([]string{
		"repo", "tags", component,
	})
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}

func publishComponents(t *testing.T, extraTags ...string) {
	t.Run("publish component", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		if extraTags == nil || len(extraTags) == 0 {
			extraTags = []string{"latest"}
		}
		args := []string{"repo", "publish-component", "meep", "1.2.3",
			"-p", "windows/amd64=" + testutil.TestdataPath(t, "meepy-component", "windows"),
			"-p", "linux/amd64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
			"-p", "darwin/amd64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
			"-p", "darwin/arm64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
		}
		for _, tag := range extraTags {
			args = append(args, []string{"--extra-tags", tag}...)
		}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})

	t.Run("publish component", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		args := []string{"repo", "publish-component", "no-windows", "1.2.3-no.windows",
			"--extra-tags", "latest",
			"-p", "linux/amd64=" + testutil.TestdataPath(t, "components", "rando"),
			"-p", "darwin/amd64=" + testutil.TestdataPath(t, "components", "rando"),
			"-p", "darwin/arm64=" + testutil.TestdataPath(t, "components", "rando"),
		}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})

	t.Run("publish generic component", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		args := []string{"repo", "publish-component", "javabro", "6.7.8",
			"--extra-tags", "latest",
			"-p", "generic=" + testutil.TestdataPath(t, "javabro-component"),
		}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})

	t.Run("publish assistant", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)
		args := []string{"repo", "publish-dpm", "4.5.6",
			"--extra-tags", "latest",
			"-p", "windows/amd64=" + testutil.TestdataPath(t, "assistant-binary", "windows", "dpm.exe"),
			"-p", "linux/amd64=" + testutil.TestdataPath(t, "assistant-binary", "unix", "dpm"),
			"-p", "darwin/amd64=" + testutil.TestdataPath(t, "assistant-binary", "unix", "dpm"),
			"-p", "darwin/arm64=" + testutil.TestdataPath(t, "assistant-binary", "unix", "dpm"),
		}
		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})
}

func verifyLink(t *testing.T) {
	verifyLnkAtPath(t, activeAssistantPath(t, os.Getenv(assistantconfig.DpmHomeEnvVar)))
}

func activeAssistantPath(t *testing.T, damlHome string) string {
	dir := filepath.Join(damlHome, "bin")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	return filepath.Join(dir, entries[0].Name())
}

func verifyLnkAtPath(t *testing.T, path string) {
	cmd := exec.Command(path)
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})
	cmd.Stdout = w
	cmd.Stderr = w

	assert.NoError(t, cmd.Run())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "fake assistant, Ha!")
}
