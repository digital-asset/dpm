// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/darmanifest"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func (suite *MainSuite) TestResolutionOfBuiltInDarDependencies() {
	t := suite.T()

	testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
data-dependencies:
  - foo-script
`)

	res := lo.Values(runResolveCommand(t).Packages)[0]
	assert.Contains(t, res.GetResolvedDependencies(), "daml-script")
	assert.Contains(t, res.GetResolvedDataDependencies(), "foo-script")
}

func (suite *MainSuite) TestResolutionOfOciDarDependencies() {
	var res *resolution.Package

	t := suite.T()

	config := testutil.MkConfig(t)
	t.Chdir(testutil.TestdataPath(t, "oci-dar-deps")) // fixture daml.yaml

	// push dars to test registry
	testutil.StartRegistry(t)

	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)
	fooDarRef, err := registry.ParseReference(fmt.Sprintf("%s/more/official/dars/foo:1.2.3", reg))
	require.NoError(t, err)
	barDarRef, err := registry.ParseReference(fmt.Sprintf("%s/some/dars/n/stuff/bar:4.5.6", reg))
	require.NoError(t, err)

	pushDar(t, "oci://"+fooDarRef.String())
	pushDar(t, "oci://"+barDarRef.String())

	t.Run("dpm install package", func(t *testing.T) {
		require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())
	})

	t.Run("should execute dpm resolve without errors", func(t *testing.T) {
		output := runResolveCommand(t)
		res = lo.Values(output.Packages)[0]
	})

	t.Run("resolution output should contain dars sourced via OCI", func(t *testing.T) {
		assert.Contains(t,
			res.GetResolvedDependencies(),
			filepath.Join(config.CachePathForDar(&fooDarRef), "test.dar"),
		)
		assert.Contains(t,
			res.GetResolvedDataDependencies(),
			filepath.Join(config.CachePathForDar(&barDarRef), "test.dar"),
		)
	})
}

func (suite *MainSuite) TestResolutionOfFilePathBasedDarDependencies() {
	t := suite.T()

	t.Run("resolution of relative file-path dars", func(t *testing.T) {
		packageDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - ./relative.dar
data-dependencies:
  - relative.dar
`))
		os.WriteFile(
			filepath.Join(packageDir, "relative.dar"),
			[]byte("another fake test dar"),
			06444)

		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.GetResolvedDependencies()[0], "relative.dar")
		checkDar(t, res.GetResolvedDependencies()[0])

		assert.Contains(t, res.GetResolvedDataDependencies()[0], "relative.dar")
		checkDar(t, res.GetResolvedDataDependencies()[0])
	})

	t.Run("resolution of absolute file-path dars", func(t *testing.T) {
		absoluteDarPath, _ := filepath.Abs(testutil.TestdataPath(t, "test-dar", "test.dar"))
		testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
data-dependencies:
  - %s
`, absoluteDarPath, absoluteDarPath))
		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.GetResolvedDependencies()[0], "test.dar")
		checkDar(t, res.GetResolvedDependencies()[0])

		assert.Contains(t, res.GetResolvedDataDependencies()[0], "test.dar")
		checkDar(t, res.GetResolvedDataDependencies()[0])
	})
}

func checkDar(t *testing.T, darFile string) {
	assert.True(t, filepath.IsAbs(darFile), "expecting absolute dar paths in the output")
	_, err := os.ReadFile(darFile)
	require.NoError(t, err)
}

func (suite *MainSuite) TestDarInstallWithArtifactLocationAlias() {
	t := suite.T()

	config := testutil.MkConfig(t)

	// push dars
	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	fooDarRef, err := registry.ParseReference(fmt.Sprintf("%s/more/official/dars/foo:1.2.3", reg))
	require.NoError(t, err)
	barDarRef, err := registry.ParseReference(fmt.Sprintf("%s/some/dars/n/stuff/bar:4.5.6", reg))
	require.NoError(t, err)

	pushDar(t, "oci://"+fooDarRef.String())
	pushDar(t, "oci://"+barDarRef.String())

	// install dars
	testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - "@digital-asset/foo:1.2.3"

data-dependencies:
  - "@my-location/bar:4.5.6"

artifact-locations:
  "@digital-asset":
    url: oci://$DPM_REGISTRY/more/official/dars
    insecure: true
  "@my-location":
    url: oci://$DPM_REGISTRY/some/dars/n/stuff
    insecure: true
`)
	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	t.Run("dar manifest includes main-package-id", func(t *testing.T) {
		darManifestPath := filepath.Join(config.CachePathForDar(&fooDarRef), assistantconfig.DarManifestName)
		m, err := darmanifest.ReadDarManifest(darManifestPath)
		require.NoError(t, err)
		assert.Equal(t, "0984ff5e3082add400bfcc6e3244bf9822ca5a617cfd92429e3fbce58058dbfa", m.Spec.Dars[0].MainPackageId)
	})

	// verify installed dars
	t.Run("dars downloaded to the dpm cache", func(t *testing.T) {
		assert.FileExists(t, filepath.Join(config.CachePathForDar(&fooDarRef), "test.dar"))
		assert.FileExists(t, filepath.Join(config.CachePathForDar(&barDarRef), "test.dar"))
	})
}

func pushDar(t *testing.T, uri string, extraTags ...string) {
	args := []string{
		"publish", "dar", uri,
		"-f", testutil.TestdataPath(t, "test-dar", "test.dar"),
		"--license", testutil.TestdataPath(t, "test-dar", "LICENSE"),
	}

	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}

	for _, tag := range extraTags {
		args = append(args, "--extra-tags", tag)
	}

	cmd := createStdTestRootCmd(t, args...)
	require.NoError(t, cmd.Execute())
}
