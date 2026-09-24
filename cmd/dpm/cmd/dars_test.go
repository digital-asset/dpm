// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/darmanifest"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	config := testutil.MkConfig(t)

	projectDir := t.TempDir()
	t.Chdir(projectDir)
	require.NoError(t, utils.CopyFile(
		testutil.TestdataPath(t, "oci-dar-deps", "daml.yaml"), // fixture daml.yaml
		filepath.Join(projectDir, "daml.yaml"),
	))

	// push dars to test registry
	testutil.StartRegistry(t)

	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)
	fooDarRef, err := registry.ParseReference(fmt.Sprintf("%s/more/official/dars/foo:1.2.3", reg))
	require.NoError(t, err)
	barDarRef, err := registry.ParseReference(fmt.Sprintf("%s/some/dars/n/stuff/bar:4.5.6", reg))
	require.NoError(t, err)

	fooDarRefWithDigest := pushDar(t, "oci://"+fooDarRef.String())
	barDarRefWithDigest := pushDar(t, "oci://"+barDarRef.String())

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
			filepath.Join(config.CachePathForDar(&fooDarRefWithDigest), "test.dar"),
		)
		assert.Contains(t,
			res.GetResolvedDataDependencies(),
			filepath.Join(config.CachePathForDar(&barDarRefWithDigest), "test.dar"),
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
	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	config := testutil.MkConfig(t)

	// push dars
	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	fooDarRef, err := registry.ParseReference(fmt.Sprintf("%s/more/official/dars/foo:1.2.3", reg))
	require.NoError(t, err)
	barDarRef, err := registry.ParseReference(fmt.Sprintf("%s/some/dars/n/stuff/bar:4.5.6", reg))
	require.NoError(t, err)

	fooDarRefWithDigest := pushDar(t, "oci://"+fooDarRef.String())
	barDarRefWithDigest := pushDar(t, "oci://"+barDarRef.String())

	// install dars
	projectDir := testutil.ActivateDamlYamlForTest(t, `
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

	require.NotEmpty(t, projectDir)

	t.Run("dar manifest includes main-package-id", func(t *testing.T) {
		darManifestPath := filepath.Join(config.CachePathForDar(&fooDarRefWithDigest), assistantconfig.DarManifestName)
		m, err := darmanifest.ReadDarManifest(darManifestPath)
		require.NoError(t, err)
		assert.Equal(t, "0984ff5e3082add400bfcc6e3244bf9822ca5a617cfd92429e3fbce58058dbfa", m.Spec.Dars[0].MainPackageId)
	})

	// verify installed dars
	t.Run("dars downloaded to the dpm cache", func(t *testing.T) {
		assert.FileExists(t, filepath.Join(config.CachePathForDar(&fooDarRefWithDigest), "test.dar"))
		assert.FileExists(t, filepath.Join(config.CachePathForDar(&barDarRefWithDigest), "test.dar"))
	})

	t.Run("oci digest gets added to daml.yaml when missing", func(t *testing.T) {
		damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)

		assert.Len(t, damlPkg.Dependencies, 1, "should not include more entries than it previously did")
		assert.Len(t, damlPkg.DataDependencies, 1, "should not include more entries than it previously did")
		assert.Contains(t, *damlPkg.Dependencies[0].ValueOnly, "@sha256:")
		assert.Contains(t, *damlPkg.DataDependencies[0].ValueOnly, "@sha256:")
	})
}

func pushDar(t *testing.T, uri string, extraTags ...string) (refWithDigest registry.Reference) {
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

	ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	require.NoError(t, err)

	client, err := assistantremote.New(ref.Registry, "", true)
	require.NoError(t, err)
	resolvedDigest, _, err := ocilister.FetchManifest(t.Context(), client, ref)
	require.NoError(t, err)

	return appendShaToRef(t, ref, resolvedDigest.String())
}

func appendShaToRef(t *testing.T, ref registry.Reference, digest string) registry.Reference {
	require.Contains(t, digest, "sha256:")
	result, err := registry.ParseReference(ref.String() + "@" + digest)
	require.NoError(t, err)
	return result
}

func (suite *MainSuite) TestResolutionOfGitDarDependencies() {
	t := suite.T()
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	config := testutil.MkConfig(t)
	cloneURL := testutil.InitGitRepo(t, "packages/foo.dar", []byte("fake git dar contents"))
	gitDep := "git:" + cloneURL + "#main?path=packages/foo.dar"

	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
`, gitDep))

	var res *resolution.Package

	t.Run("dpm install package", func(t *testing.T) {
		require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())
	})

	t.Run("install pins commit in daml.yaml", func(t *testing.T) {
		pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		require.Len(t, pkg.Dependencies, 1)
		value, err := pkg.Dependencies[0].Value()
		require.NoError(t, err)
		dep := lo.Values(pkg.ParsedDarDependencies.Dependencies)[0]
		assert.False(t, gitparse.GitRefIsMutable(dep.Git.Ref), "expected commit SHA pin in yaml ref")
		assert.NotEqual(t, "main", dep.Git.Ref)
		assert.Contains(t, value, dep.Git.Ref)
	})

	t.Run("should execute dpm resolve without errors", func(t *testing.T) {
		output := runResolveCommand(t)
		res = lo.Values(output.Packages)[0]
	})

	t.Run("resolution output should contain cached git dar path", func(t *testing.T) {
		require.NotEmpty(t, res.GetResolvedDependencies())
		cachedPath := res.GetResolvedDependencies()[0]
		assert.True(t, filepath.IsAbs(cachedPath))
		assert.Contains(t, cachedPath, filepath.Join("cache", "git"))
		assert.Contains(t, cachedPath, "packages/foo.dar")
		assert.FileExists(t, cachedPath)

		pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		dep := lo.Values(pkg.ParsedDarDependencies.Dependencies)[0]
		expected, err := config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, dep.Git.Ref)
		require.NoError(t, err)
		assert.Equal(t, expected, cachedPath)
	})

	t.Run("dpm update --check passes when pinned", func(t *testing.T) {
		require.NoError(t, createStdTestRootCmd(t, "update", "--check").Execute())
	})
}

func (suite *MainSuite) TestInstallGitDarWithCommitPinInYaml() {
	t := suite.T()
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	config := testutil.MkConfig(t)
	cloneURL := testutil.InitGitRepo(t, "packages/foo.dar", []byte("first dar version"))

	repoPath := strings.TrimPrefix(cloneURL, "file://")
	repo, err := git.PlainOpen(repoPath)
	require.NoError(t, err)
	head, err := repo.Head()
	require.NoError(t, err)
	firstCommit := head.Hash().String()

	w, err := repo.Worktree()
	require.NoError(t, err)
	darAbs := filepath.Join(repoPath, "packages", "foo.dar")
	require.NoError(t, os.WriteFile(darAbs, []byte("second dar version"), 0o644))
	_, err = w.Add("packages/foo.dar")
	require.NoError(t, err)
	secondCommit, err := w.Commit("update dar", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test"},
	})
	require.NoError(t, err)

	secondDep := fmt.Sprintf("git:%s#%s?path=packages/foo.dar", cloneURL, secondCommit.String())
	testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
`, secondDep))

	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	secondCached, err := gitparse.ParseGitDependency(secondDep)
	require.NoError(t, err)
	secondCachedPath, err := config.CachePathForGitDependency(secondCached.Git.CloneURL, secondCached.Git.DarPath, secondCommit.String())
	require.NoError(t, err)
	assert.FileExists(t, secondCachedPath)
	secondContent, err := os.ReadFile(secondCachedPath)
	require.NoError(t, err)
	assert.Equal(t, "second dar version", string(secondContent))

	firstDep := fmt.Sprintf("git:%s#%s?path=packages/foo.dar", cloneURL, firstCommit)
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - %s
`, firstDep))
	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	firstCached, err := gitparse.ParseGitDependency(firstDep)
	require.NoError(t, err)
	firstCachedPath, err := config.CachePathForGitDependency(firstCached.Git.CloneURL, firstCached.Git.DarPath, firstCommit)
	require.NoError(t, err)
	firstContent, err := os.ReadFile(firstCachedPath)
	require.NoError(t, err)
	assert.Equal(t, "first dar version", string(firstContent))

	newYaml := fmt.Sprintf(`
dependencies:
  - %s
`, secondDep)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "daml.yaml"), []byte(newYaml), 0o644))
	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	secondContent, err = os.ReadFile(secondCachedPath)
	require.NoError(t, err)
	assert.Equal(t, "second dar version", string(secondContent))
	assert.NotEqual(t, string(firstContent), string(secondContent))
}

func (suite *MainSuite) TestResolutionOfGitDarDataDependencies() {
	t := suite.T()
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	config := testutil.MkConfig(t)
	cloneURL := testutil.InitGitRepo(t, "packages/foo.dar", []byte("fake git data dar contents"))
	const packageID = "data-main-package"
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
data-dependencies:
  - value: git:%s#main?path=packages/foo.dar
    main-package-id: %s
`, cloneURL, packageID))

	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)
	dep := lo.Values(pkg.ParsedDarDependencies.DataDependencies)[0]
	assert.False(t, gitparse.GitRefIsMutable(dep.Git.Ref))
	require.NotNil(t, pkg.DataDependencies[0].GetMainPackageId())
	assert.Equal(t, packageID, *pkg.DataDependencies[0].GetMainPackageId())

	output := runResolveCommand(t)
	res := lo.Values(output.Packages)[0]
	require.Empty(t, res.GetResolvedDependencies())
	require.Len(t, res.GetResolvedDataDependencies(), 1)
	cachedPath := res.GetResolvedDataDependencies()[0]
	assert.FileExists(t, cachedPath)
	expected, err := config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, dep.Git.Ref)
	require.NoError(t, err)
	assert.Equal(t, expected, cachedPath)

	require.NoError(t, createStdTestRootCmd(t, "update", "--check").Execute())
}

func (suite *MainSuite) TestResolutionOfGitDarDependenciesWithAlias() {
	t := suite.T()
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	config := testutil.MkConfig(t)
	cloneURL := testutil.InitGitRepo(t, "packages/foo.dar", []byte("foo dar contents"))
	repoPath := strings.TrimPrefix(cloneURL, "file://")
	barAbs := filepath.Join(repoPath, "packages", "bar.dar")
	require.NoError(t, os.WriteFile(barAbs, []byte("bar dar contents"), 0o644))
	repo, err := git.PlainOpen(repoPath)
	require.NoError(t, err)
	w, err := repo.Worktree()
	require.NoError(t, err)
	_, err = w.Add("packages/bar.dar")
	require.NoError(t, err)
	commit, err := w.Commit("add bar dar", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test"},
	})
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName("main"),
		commit,
	)))

	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - "@shared-repo#main?path=packages/foo.dar"
  - "@shared-repo#main?path=packages/bar.dar"

artifact-locations:
  "@shared-repo":
    url: git:%s
`, cloneURL))

	require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())

	pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)
	require.Len(t, pkg.Dependencies, 2)

	for _, raw := range pkg.Dependencies {
		value, err := raw.Value()
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(value, "git:"), "expected expanded git pin, got %q", value)
		assert.NotContains(t, value, "#main?")
		dep := pkg.ParsedDarDependencies.Dependencies[value]
		require.NotNil(t, dep)
		assert.False(t, gitparse.GitRefIsMutable(dep.Git.Ref))
		assert.Contains(t, []string{"packages/foo.dar", "packages/bar.dar"}, dep.Git.DarPath)

		cached, err := config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, dep.Git.Ref)
		require.NoError(t, err)
		assert.FileExists(t, cached)
	}

	output := runResolveCommand(t)
	res := lo.Values(output.Packages)[0]
	require.Len(t, res.GetResolvedDependencies(), 2)
}
