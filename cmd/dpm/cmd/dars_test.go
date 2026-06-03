// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocipusher/darpusher"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func (suite *MainSuite) TestResolutionOfBuiltInDarDependencies() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
data-dependencies:
  - foo-script
`)

	res := lo.Values(runResolveCommand(t).Packages)[0]
	assert.Contains(t, res.ResolvedDependencies, "daml-script")
	assert.Contains(t, res.ResolvedDataDependencies, "foo-script")
}

func (suite *MainSuite) TestResolutionOfFilePathBasedDarDependencies() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	t.Run("resolution of relative file-path dars", func(t *testing.T) {
		packageDir := ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - ./relative.dar
data-dependencies:
  - ./relative.dar
`))
		os.WriteFile(
			filepath.Join(packageDir, "relative.dar"),
			[]byte("another fake test dar"),
			06444)

		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.ResolvedDependencies[0], "relative.dar")
		checkDar(t, res.ResolvedDependencies[0])

		assert.Contains(t, res.ResolvedDataDependencies[0], "relative.dar")
		checkDar(t, res.ResolvedDataDependencies[0])
	})

	t.Run("resolution of absolute file-path dars", func(t *testing.T) {
		absoluteDarPath, _ := filepath.Abs(testutil.TestdataPath(t, "test-dar", "test.dar"))
		ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
data-dependencies:
  - %s
`, absoluteDarPath, absoluteDarPath))
		res := lo.Values(runResolveCommand(t).Packages)[0]

		assert.Contains(t, res.ResolvedDependencies[0], "test.dar")
		checkDar(t, res.ResolvedDependencies[0])

		assert.Contains(t, res.ResolvedDataDependencies[0], "test.dar")
		checkDar(t, res.ResolvedDataDependencies[0])
	})
}

func ActivateDamlYamlForTest(t *testing.T, s string) (packageDir string) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(s), 0666))
	return tmpDir
}

func checkDar(t *testing.T, darFile string) {
	assert.True(t, filepath.IsAbs(darFile), "expecting absolute dar paths in the output")
	_, err := os.ReadFile(darFile)
	require.NoError(t, err)
}

func (suite *MainSuite) TestDarInstallWithArtifactLocationAlias() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmDarsEnabledEnvVar, "true")

	tmpDpmHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDpmHome)

	// push dars
	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)
	pushDar(t, fmt.Sprintf("%s/more/official/dars/foo:1.2.3", reg))
	pushDar(t, fmt.Sprintf("%s/some/dars/n/stuff/bar:4.5.6", reg))

	// install dars
	ActivateDamlYamlForTest(t, `
dependencies:
  - "foo:1.2.3"

data-dependencies:
  - "@my-location/bar:4.5.6"

artifact-locations:
  "@digital-asset":
    default: true
    url: oci://$DPM_REGISTRY/more/official/dars
    insecure: true
  "@my-location":
    url: oci://$DPM_REGISTRY/some/dars/n/stuff
    insecure: true
`)
	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	// verify installed dars
	dars := listAllDarsInCache(t, filepath.Join(tmpDpmHome, "cache", "dars"))
	assertContainsDar(t, dars, "foo/1.2.3/test.dar")
	assertContainsDar(t, dars, "bar/4.5.6/test.dar")
}

func pushDar(t *testing.T, uri string, extraTags ...string) {
	ref, err := registry.ParseReference(uri)
	require.NoError(t, err)

	pushOp, err := darpusher.DarNew(t.Context(), darpusher.DarOpts{
		Artifact: &ociconsts.DarArtifact{
			DarRepo: ref.Repository,
		},
		RawTag:              ref.Reference,
		Dir:                 testutil.TestdataPath(t, "test-dar"),
		RequiredAnnotations: ociconsts.DescriptorAnnotations{},
	})

	require.NoError(t, err)
	client, err := assistantremote.New(ref.Registry, "", true)
	require.NoError(t, err)

	_, err = pushOp.DarDo(t.Context(), client)
	require.NoError(t, err)
}

func listAllDarsInCache(t *testing.T, darsCacheDir string) []string {
	var matches []string

	err := filepath.WalkDir(darsCacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".dar") {
			matches = append(matches, filepath.ToSlash(path))
		}
		return nil
	})

	require.NoError(t, err)
	return matches
}

func assertContainsDar(t *testing.T, cacheDars []string, darFilePath string) {
	_, ok := lo.Find(cacheDars, func(f string) bool {
		return strings.HasSuffix(f, darFilePath)
	})
	assert.True(t, ok, "expected to find dar %q in cache", darFilePath)
}
