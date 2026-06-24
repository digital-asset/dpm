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

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
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

	t.Run("install package with local-filepath components", func(t *testing.T) {
		localComponentPath := testutil.TestdataPath(t, "another-generic-component")

		testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
components:
    - name: my-local-component
      path: %s`, localComponentPath))

		cmd := createStdTestRootCmd(t, installCommand...)
		require.NoError(t, cmd.Execute())

		deepResolution := runResolveCommand(t)
		assert.Equal(t, localComponentPath, lo.Values(deepResolution.Packages)[0].ComponentsV2["my-local-component"]["path"])
	})
}

func (suite *MainSuite) TestShaPinningForUriComponentsInSinglePackageProject() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_ = testutil.MkConfig(t)
	_, reg := testutil.StartRegistry(t)
	newComponentRepo := "newly/added:4.5.6"
	newComponent := fmt.Sprintf("oci://%s/%s", testutil.GetRemote(reg).Registry, newComponentRepo)

	args := testutil.PushComponentUri(reg, newComponentRepo, testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
components:
  - %s
`, newComponent))

	t.Run("dpm install appends missing sha256 to oci uri", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "install")
		require.NoError(t, cmd.Execute())

		ref, err := registry.ParseReference(strings.TrimPrefix(newComponent, "oci://"))
		require.NoError(t, err)
		client, err := assistantremote.New(ref.Registry, "", true)
		require.NoError(t, err)
		resolvedDigest, _, err := ocilister.FetchManifest(t.Context(), client, ref)
		require.NoError(t, err)

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), newComponent+"@"+resolvedDigest.String())

		assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
	})

	t.Run("dpm resolve works for sha256 pinned oci components", func(t *testing.T) {
		resolution := runResolveCommand(t)
		comps := lo.Values(lo.Values(resolution.Packages)[0].ComponentsV2)
		assert.Len(t, comps, 1)
		assert.Equal(t, "4.5.6", comps[0]["version"])

		assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
	})

	t.Run("dpm installing more than once", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "install")
		require.NoError(t, cmd.Execute())

		assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
	})
}

func (suite *MainSuite) TestShaPinningForUriComponentsInMultiPackageProject() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_ = testutil.MkConfig(t)
	_, reg := testutil.StartRegistry(t)
	newComponentRepo := "newly/added:4.5.6"
	newComponent := fmt.Sprintf("oci://%s/%s", testutil.GetRemote(reg).Registry, newComponentRepo)

	args := testutil.PushComponentUri(reg, newComponentRepo, testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	projectDir := testutil.ActivateMultiPackageYamlForTest(t, fmt.Sprintf(`
components:
  - %s
`, newComponent))

	t.Run("dpm install appends missing sha256 to oci uri", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "install", "package")
		require.NoError(t, cmd.Execute())

		ref, err := registry.ParseReference(strings.TrimPrefix(newComponent, "oci://"))
		require.NoError(t, err)
		client, err := assistantremote.New(ref.Registry, "", true)
		require.NoError(t, err)
		resolvedDigest, _, err := ocilister.FetchManifest(t.Context(), client, ref)
		require.NoError(t, err)

		newContent, err := os.ReadFile(filepath.Join(projectDir, "multi-package.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), newComponent+"@"+resolvedDigest.String())

		assertContainsComment(t, filepath.Join(projectDir, "multi-package.yaml"), "# 4.5.6")
	})

	t.Run("dpm installing more than once", func(t *testing.T) {
		cmd := createStdTestRootCmd(t, "install")
		require.NoError(t, cmd.Execute())

		assertContainsComment(t, filepath.Join(projectDir, "multi-package.yaml"), "# 4.5.6")
	})
}

func (suite *MainSuite) TestLegacyCacheResolutionForSdkComponentsUnaffectedByShaCaching() {
	t := suite.T()

	installSdk(t, []string{someSdkVersion})

	deepResolution := runResolveCommand(t)
	comp := deepResolution.DefaultSDK[someSdkVersion].ComponentsV2["meep"]
	assert.Equal(t, "1.2.3", comp["version"])
	assert.Equal(t,
		filepath.Join(os.Getenv(assistantconfig.DpmHomeEnvVar), "cache", "components", "meep", "1.2.3"),
		comp["path"])
}

func (suite *MainSuite) TestLegacyCacheResolutionForShorthandComponentsUnaffectedByShaCaching() {
	t := suite.T()
	c := testutil.MkConfig(t)

	ctx := testutil.Context(t)
	_, reg := testutil.StartRegistry(t)

	// Want to ensure that version is still using handleOCI - push up using internal DA pushComponent
	testutil.PushComponent(t, ctx, reg, "meep", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))

	t.Chdir(testutil.TestdataPath(t, "resolve-test", testutil.OS))
	cmd := createStdTestRootCmd(t, "install", "package")
	require.NoError(t, cmd.Execute())

	deepResolution := runResolveCommand(t)
	comp := lo.Values(deepResolution.Packages)[0].ComponentsV2["meep"]
	assert.Equal(t, comp["path"], filepath.Join(c.CachePath, "components", "meep", "1.2.3"))
	assert.Equal(t, "1.2.3", comp["version"])
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
