package cmd

import (
	"context"

	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/ociindex"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestInstallPackage() {
	t := suite.T()

	t.Run("install package via alias command", func(t *testing.T) {
		testInstallPackage(t, []string{"install"})
	})

	t.Run("install package via full dpm install package command", func(t *testing.T) {
		testInstallPackage(t, []string{"install", "package"})
	})
}

func testInstallPackage(t *testing.T, installCommand []string) {
	// set dpm home to temp dir
	dpmHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, dpmHome)

	// cleanup
	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

	t.Run("multi pkg no override", func(t *testing.T) {

		// install some sdk version
		sdkVersion := someSdkVersion
		installSdk(t, []string{sdkVersion})

		// use multi-package-another testdata which
		// has a package that depends on the same sdk version
		// but doesn't override it, so should not cause sdk to be reinstalled
		require.NoError(t, os.Chdir(testutil.TestdataPath(t, filepath.Join(
			"multi-package-another"))))

		cmd, r, w := createTestRootCmd(t, installCommand...)

		// assertions
		require.NoError(t, cmd.Execute())
		assert.NoError(t, w.Close())
		output, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Contains(t, string(output), "Successfully installed SDK "+sdkVersion)
		assert.Contains(t, string(output), "No opt-in components to install")
	})

	t.Run("install package multiple registries", func(t *testing.T) {
		regURL, altURL := setupRegistriesAndPublishedComponents(t)

		// run install package

		tmpProjectDir := t.TempDir()
		require.NoError(t, utils.CopyFile(
			filepath.Join(testutil.TestdataPath(t, "multi-registry", "daml.yaml")),
			filepath.Join(tmpProjectDir, "daml.yaml"),
		))
		t.Chdir(tmpProjectDir)

		cmd := createStdTestRootCmd(t, installCommand...)
		require.NoError(t, cmd.Execute())

		// run some command for meep component
		require.NoError(t, createStdTestRootCmd(t, "meep").Execute())

		// run resolve command
		deepResolution := runResolveCommand(t)
		assert.Len(t, deepResolution.Packages, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 3)

		checkComponent := checkComponent(t, deepResolution, dpmHome)
		require.NoError(t, err)

		// Test that the cache and dpm resolve use the full URI for `oci://` based components
		checkComponent(regURL+"/"+"foo/bar/meep", "1.2.3")
		checkComponent(altURL+"/"+"bar/foo/rando", "1.2.4")
	})

	t.Run("install package with pinning", func(t *testing.T) {
		ctx := testutil.Context(t)
		client, reg := testutil.StartRegistry(t)

		regURL := strings.TrimPrefix(reg.URL, "http://")

		t.Setenv("TEST_DPM_REGISTRY", "oci://"+regURL)

		cwd, err := os.Getwd()
		require.NoError(t, err)

		t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
		args := testutil.PushComponentUri(reg, fmt.Sprintf("%s/%s:%s", "foo/bar", "meep", "1.2.3"), testutil.TestdataPath(t, "meepy-component", testutil.OS))
		require.NoError(t, createStdTestRootCmd(t, args...).Execute())
		args = testutil.PushComponentUri(reg, fmt.Sprintf("%s/%s:%s", "bar/foo", "rando", "1.2.4"), testutil.TestdataPath(t, "components", "rando"))
		require.NoError(t, createStdTestRootCmd(t, args...).Execute())

		// Want to ensure that version is still using handleOCI - push up using internal DA pushComponent
		testutil.PushComponent(t, ctx, reg, "javabro", "6.7.8", testutil.TestdataPath(t, "javabro-component"))

		require.NoError(t, os.Chdir(testutil.TestdataPath(t, "multi-pinning", testutil.OS)))

		// retrieve sha to use in damls
		repo, err := client.Repo("foo/bar/meep")
		require.NoError(t, err)
		meepDescriptor, err := repo.Resolve(ctx, "1.2.3")
		require.NoError(t, err)
		meepSHA := meepDescriptor.Digest.String()
		t.Setenv("TEST_MEEP_SHA", meepSHA)

		repoRando, err := client.Repo("bar/foo/rando")
		require.NoError(t, err)
		randoDescriptor, err := repoRando.Resolve(ctx, "1.2.4")
		require.NoError(t, err)
		randoSHA := randoDescriptor.Digest.String()
		t.Setenv("TEST_RANDO_SHA", randoSHA)

		cmd := createStdTestRootCmd(t, installCommand...)
		require.NoError(t, cmd.Execute())

		require.NoError(t, createStdTestRootCmd(t, "meep").Execute())

		deepResolution := runResolveCommand(t)
		assert.Len(t, deepResolution.Packages, 1)
		assert.Len(t, lo.Values(deepResolution.Packages)[0].Components, 2)

		checkComponent := func(name, version string) {
			// Test that the cache and dpm resolve use the full URI for `oci://` based components
			comp := lo.Values(deepResolution.Packages)[0].ComponentsV2[name]
			assert.Equal(t, comp["path"], filepath.Join(dpmHome, "cache", "components", utils.UrlToFilePath(name), comp["version"]))
			assert.Equal(t, version, comp["version"])
		}

		// Test that the cache and dpm resolve use the full URsI for `oci://` based components
		checkComponent(regURL+"/"+"foo/bar/meep", "1.2.3")

		t.Run("test that moving tag to new sha doesn't break pinning", func(t *testing.T) {
			args := testutil.PushComponentUri(reg, fmt.Sprintf("%s/%s:%s", "foo/bar", "meep", "1.2.3"), testutil.TestdataPath(t, "components", "rando"))
			require.NoError(t, createStdTestRootCmd(t, args...).Execute())
			cmd := createStdTestRootCmd(t, installCommand...)
			require.NoError(t, cmd.Execute())
			// assert meep component not overwritten
			require.NoError(t, createStdTestRootCmd(t, "meep").Execute())
			checkComponent(regURL+"/"+"foo/bar/meep", "1.2.3")
		})
	})
}

func (suite *MainSuite) TestLegacyCacheResolution() {
	t := suite.T()

	cwd, err := os.Getwd()

	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

	installSdk(t, []string{someSdkVersion})

	c, err := assistantconfig.Get()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(testutil.TestdataPath(t, "resolve-test", testutil.OS)))

	deepResolution := runResolveCommand(t)
	comp := lo.Values(deepResolution.Packages)[0].ComponentsV2["meep"]
	assert.Len(t, deepResolution.Packages, 1)
	assert.Equal(t, "1.2.3", comp["version"])

	t.Run("test cache still writing to version", func(t *testing.T) {
		ctx := testutil.Context(t)
		client, reg := testutil.StartRegistry(t)

		// Want to ensure that version is still using handleOCI - push up using internal DA pushComponent
		testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
		require.NoError(t, os.Chdir(testutil.TestdataPath(t, "resolve-test", testutil.OS)))

		cmd := createStdTestRootCmd(t, "install", "package")
		require.NoError(t, cmd.Execute())

		repoMeep, err := client.Repo("components/meep")
		require.NoError(t, err)
		meepDescriptor, err := repoMeep.Resolve(ctx, "1.2.3")
		require.NoError(t, err)

		rc, err := repoMeep.Fetch(ctx, meepDescriptor)
		require.NoError(t, err)
		defer rc.Close()

		index, _, err := ociindex.FetchIndex(ctx, client, "components/meep", "1.2.3")
		require.NoError(t, err)

		comp := lo.Values(deepResolution.Packages)[0].ComponentsV2["meep"]
		assert.Equal(t, comp["path"], filepath.Join(c.CachePath, "components", utils.UrlToFilePath("meep"), strings.ReplaceAll(index.Manifests[0].Digest.String(), ":", "_")))
		assert.Equal(t, "1.2.3", comp["version"])
	})
}

func checkComponent(t *testing.T, deepResolution *resolution.Resolution, dpmHome string) func(name string, version string) {
	checkComponent := func(name, version string) {
		// Test that the cache and dpm resolve use the full URI for `oci://` based components
		comp := lo.Values(deepResolution.Packages)[0].ComponentsV2[name]
		assert.Equal(t, comp["path"], filepath.Join(dpmHome, "cache", "components", utils.UrlToFilePath(name), version))
		assert.Equal(t, version, comp["version"])
	}
	return checkComponent
}

func setupRegistriesAndPublishedComponents(t *testing.T) (mainRegistryURL string, altRegistryURL string) {
	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)
	_, altReg := testutil.StartRegistry(t)

	regURL := strings.TrimPrefix(reg.URL, "http://")
	altURL := strings.TrimPrefix(altReg.URL, "http://")

	t.Setenv("TEST_DPM_REGISTRY", "oci://"+regURL)
	t.Setenv("TEST_ALT_DPM_REGISTRY", "oci://"+altURL)

	cwd, err := os.Getwd()
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
	publishMeepRandoJavabroComponents(t, reg, altReg, ctx)
	return regURL, altURL
}

func publishMeepRandoJavabroComponents(t *testing.T, reg *httptest.Server, altReg *httptest.Server, ctx context.Context) {
	args := testutil.PushComponentUri(reg, fmt.Sprintf("%s/%s:%s", "foo/bar", "meep", "1.2.3"), testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())
	args = testutil.PushComponentUri(altReg, fmt.Sprintf("%s/%s:%s", "bar/foo", "rando", "1.2.4"), testutil.TestdataPath(t, "components", "rando"))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	// Want to ensure that version is still using handleOCI - push up using internal DA pushComponent
	testutil.PushComponent(t, ctx, altReg, "javabro", "6.7.8", testutil.TestdataPath(t, "javabro-component"))
}
