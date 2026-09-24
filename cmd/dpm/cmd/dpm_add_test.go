package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func (suite *MainSuite) TestDpmAddComponentCommand() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_, reg := testutil.StartRegistry(t)
	newComponentRepo := "newly/added:4.5.6"
	newComponent := fmt.Sprintf("oci://%s/%s", testutil.GetRemote(reg).Registry, newComponentRepo)

	args := testutil.PushComponentUri(reg, newComponentRepo, testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	t.Run("add new component in single-package project", func(t *testing.T) {
		projectDir := testutil.ActivateDamlYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent, "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent+"@sha256:")
	})

	t.Run("add new component in multi-package project", func(t *testing.T) {
		projectDir := testutil.ActivateMultiPackageYamlForTest(t, `
components:
  - damlc:1.2.3
  - oci://example.com/some/component:1.2.3
`)

		cmd := createStdTestRootCmd(t, "add", "component", newComponent, "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "multi-package.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- "+newComponent+"@sha256:")
	})
}

func (suite *MainSuite) TestDpmAddDarCommand() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	darRef, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:4.5.6", reg))
	require.NoError(t, err)
	pushDar(t, "oci://"+darRef.String())

	t.Run("add new dar in single-package project", func(t *testing.T) {
		projectDir := testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
  - std-lib
`)

		cmd := createStdTestRootCmd(t, "add", "dar", "--data-dependencies", "oci://"+darRef.String(), "--insecure")
		require.NoError(t, cmd.Execute())

		newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "- oci://"+darRef.String()+"@sha256:")
	})

}

// tests 'dpm add dar' for a dar that already exists in daml.yaml
func (suite *MainSuite) TestAddingExistingDar() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	testutil.StartRegistry(t)
	reg := os.Getenv(assistantconfig.OciRegistryEnvVar)

	// set up a daml.yaml containing a ":latest@sha256:" dar already
	darRefLatest, err := registry.ParseReference(fmt.Sprintf("%s/newly/added:latest", reg))
	require.NoError(t, err)

	oldSha256 := "sha256:12d74505ebae3959746e8a2f5ab68b942a5580634dda7ea1a586874e07b52eb9"
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
data-dependencies:
  - std-lib
  - oci://%s@%s`, darRefLatest.String(), oldSha256))

	// push a new "latest" tag
	pushDar(t, fmt.Sprintf("oci://%s/newly/added:4.5.6", reg), "latest")

	// Running 'dpm add dar oci://<uri>:latest' should now essentially bump the dar to the new latest
	cmd := createStdTestRootCmd(t, "add", "dar", "--data-dependencies", "oci://"+darRefLatest.String(), "--insecure")
	require.NoError(t, cmd.Execute())

	damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)

	assert.Len(t, damlPkg.DataDependencies, 2, "should not include more entries than it previously did")
	assert.Equal(t, "std-lib", *damlPkg.DataDependencies[0].ValueOnly)
	assert.Contains(t, *damlPkg.DataDependencies[1].ValueOnly, "oci://"+darRefLatest.String()+"@sha256:")
	assert.NotContains(t, *damlPkg.DataDependencies[1].ValueOnly, oldSha256, "daml.yaml expected to not contain the old sha256 anymore")

	assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
}

func (suite *MainSuite) TestDpmAddGitDarCommand() {
	t := suite.T()
	t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")

	cloneURL := testutil.InitGitRepo(t, "foo.dar", []byte("add command dar"))
	gitURI := "git:" + cloneURL + "#main?path=foo.dar"

	projectDir := testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
`)

	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", gitURI)
	require.NoError(t, cmd.Execute())

	newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(newContent), "git:"+cloneURL)
	assert.NotContains(t, string(newContent), "#main?")
	assert.Regexp(t, `#[0-9a-f]{40}\?path=foo\.dar`, string(newContent))
}

func (suite *MainSuite) TestDpmAddGitReleaseDarCommand() {
	t := suite.T()

	const tag = "v1.0.0"
	const assetA = "foo-1.0.0.dar"
	const assetB = "bar-1.0.0.dar"

	host := testutil.GitHubReleaseServer(t, true,
		testutil.GitHubReleaseAsset{Name: assetA, Body: []byte("release dar")},
		testutil.GitHubReleaseAsset{Name: assetB, Body: []byte("release dar")},
	)

	projectDir := testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
`)

	releaseURI := "git:http://" + host + "/org/repo?release=" + tag
	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", releaseURI)
	require.NoError(t, cmd.Execute())

	newContent, err := os.ReadFile(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(newContent), assetA)
	assert.Contains(t, string(newContent), assetB)
}

func (suite *MainSuite) TestDpmAddGitReleaseDarExpandsExistingLine() {
	t := suite.T()

	const tag = "v1.0.0"
	const assetA = "foo-1.0.0.dar"
	const assetB = "bar-1.0.0.dar"

	host := testutil.GitHubReleaseServer(t, true,
		testutil.GitHubReleaseAsset{Name: assetA, Body: []byte("release dar")},
		testutil.GitHubReleaseAsset{Name: assetB, Body: []byte("release dar")},
	)

	releaseLine := "git:http://" + host + "/org/repo?release=" + tag
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
dependencies:
  - daml-script
  - %s
`, releaseLine))

	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", releaseLine)
	require.NoError(t, cmd.Execute())

	damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)
	assert.Len(t, damlPkg.Dependencies, 3)

	yamlContent := string(mustReadFile(t, filepath.Join(projectDir, "daml.yaml")))
	assert.NotContains(t, yamlContent, releaseLine)

	assetCount := 0
	for _, dep := range damlPkg.Dependencies {
		line, err := dep.Value()
		require.NoError(t, err)
		if strings.Contains(line, assetA) || strings.Contains(line, assetB) {
			assetCount++
		}
	}
	assert.Equal(t, 2, assetCount)
}

func (suite *MainSuite) TestDpmAddGitReleaseDarRejectsNonGitHubHostWithoutEditingYaml() {
	t := suite.T()

	const original = `
dependencies:
  - daml-script
`
	projectDir := testutil.ActivateDamlYamlForTest(t, original)
	yamlPath := filepath.Join(projectDir, "daml.yaml")
	before := mustReadFile(t, yamlPath)

	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", "git:gitlab.com/org/repo?release=v1.0.0")
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for github.com")
	assert.Contains(t, err.Error(), "?path=")

	assert.Equal(t, string(before), string(mustReadFile(t, yamlPath)),
		"daml.yaml must be untouched when the release host is unsupported")
}

func (suite *MainSuite) TestDpmAddGitReleaseDarRollsBackYamlWhenExpansionFails() {
	t := suite.T()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv("DPM_TEST_GITHUB_API_BASE", srv.URL)
	host := srv.Listener.Addr().String()
	t.Setenv("DPM_TEST_GITHUB_RELEASE_HOST", host)

	projectDir := testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
`)
	yamlPath := filepath.Join(projectDir, "daml.yaml")
	before := mustReadFile(t, yamlPath)

	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", "git:http://"+host+"/org/repo?release=missing-tag")
	require.Error(t, cmd.Execute())

	after := mustReadFile(t, yamlPath)
	assert.Equal(t, string(before), string(after),
		"daml.yaml must be restored when release expansion fails")
	assert.NotContains(t, string(after), "missing-tag")
}

func (suite *MainSuite) TestDpmAddGitReleaseDarKeepsExpandedLinesWhenDownloadFails() {
	t := suite.T()

	const tag = "v1.0.0"
	const asset = "foo-1.0.0.dar"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"assets": []map[string]string{{"name": asset}},
			})
		case strings.Contains(r.URL.Path, "/releases/download/"):
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("DPM_TEST_GITHUB_API_BASE", srv.URL)
	host := srv.Listener.Addr().String()
	t.Setenv("DPM_TEST_GITHUB_RELEASE_HOST", host)

	projectDir := testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
`)
	yamlPath := filepath.Join(projectDir, "daml.yaml")

	releaseURI := "git:http://" + host + "/org/repo?release=" + tag
	cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", releaseURI)
	require.Error(t, cmd.Execute())

	after := string(mustReadFile(t, yamlPath))
	assert.Contains(t, after, asset,
		"the expanded per-asset line must survive so install can retry the download")
	assert.NotContains(t, after, releaseURI,
		"the unexpanded base line must not be left behind")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return b
}

func (suite *MainSuite) TestDpmAddDarCommandNegativeCases() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	t.Run("git dar can be added to data-dependencies", func(t *testing.T) {
		t.Setenv("DPM_TEST_ALLOW_FILE_GIT", "true")
		cloneURL := testutil.InitGitRepo(t, "foo.dar", []byte("x"))
		gitURI := "git:" + cloneURL + "#main?path=foo.dar"

		projectDir := testutil.ActivateDamlYamlForTest(t, `
dependencies:
  - daml-script
`)

		cmd := createStdTestRootCmd(t, "add", "dar", "--data-dependencies", gitURI)
		require.NoError(t, cmd.Execute())

		pkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
		require.NoError(t, err)
		require.Len(t, pkg.ParsedDarDependencies.DataDependencies, 1)
	})

	t.Run("add new dar to both data-dependencies and dependencies fails in single-package project", func(t *testing.T) {
		_ = testutil.ActivateDamlYamlForTest(t, `
data-dependencies:
  - std-lib
`)

		cmd := createStdTestRootCmd(t, "add", "dar", "--dependencies", "--data-dependencies", "oci://doesnt/matter/for/this/test:1.2.3", "--insecure")
		require.Error(t, cmd.Execute())
	})
}

func (suite *MainSuite) TestAddingExistingComponentWithFloatyTagBumpsToNewerVersion() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_, reg := testutil.StartRegistry(t)

	newComponentWithoutRegistry := "newly/added"
	newComponent := fmt.Sprintf("%s/%s", testutil.GetRemote(reg).Registry, newComponentWithoutRegistry)

	// set up a daml.yaml containing a ":latest@sha256:" dar already
	compRefLatest := newComponent + ":latest"
	oldSha256 := "sha256:12d74505ebae3959746e8a2f5ab68b942a5580634dda7ea1a586874e07b52eb9"
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
components:
  - pre-existing:1.2.3
  - oci://%s@%s`, compRefLatest, oldSha256))

	// push a new "latest" tag
	args := testutil.PushComponentUri(reg, newComponentWithoutRegistry+":4.5.6", testutil.TestdataPath(t, "meepy-component", testutil.OS), "latest")
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	// Running 'dpm add oci://<uri>:latest' should now essentially bump to the new latest
	cmd := createStdTestRootCmd(t, "add", "component", "oci://"+compRefLatest, "--insecure")
	require.NoError(t, cmd.Execute())

	damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)

	assert.Len(t, damlPkg.ComponentsList, 2, "should not include more entries than it previously did")
	assert.Equal(t, "pre-existing:1.2.3", *damlPkg.ComponentsList[0].StringBased)
	assert.Contains(t, *damlPkg.ComponentsList[1].StringBased, "oci://"+compRefLatest+"@sha256:")
	assert.NotContains(t, *damlPkg.ComponentsList[1].StringBased, oldSha256, "daml.yaml expected to not contain the old sha256 anymore")

	assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
}

func assertContainsComment(t *testing.T, yamlFilePath, comment string) {
	bytes, err := os.ReadFile(yamlFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), comment)
}

// tests 'dpm add component' for a component that already exists in daml.yaml
func (suite *MainSuite) TestAddingExistingComponentWithDifferentTagReplacesEntry() {
	t := suite.T()

	t.Setenv(assistantconfig.DpmShaPinningEnabled, "true")

	_, reg := testutil.StartRegistry(t)

	newComponentWithoutRegistry := "newly/added"
	newComponent := fmt.Sprintf("%s/%s", testutil.GetRemote(reg).Registry, newComponentWithoutRegistry)

	// set up a daml.yaml containing a ":latest@sha256:" dar already
	compRefLatest := newComponent + ":latest"
	oldSha256 := "sha256:12d74505ebae3959746e8a2f5ab68b942a5580634dda7ea1a586874e07b52eb9"
	projectDir := testutil.ActivateDamlYamlForTest(t, fmt.Sprintf(`
components:
  - pre-existing:1.2.3
  - oci://%s@%s`, compRefLatest, oldSha256))

	// push a different tag
	args := testutil.PushComponentUri(reg, newComponentWithoutRegistry+":4.5.6", testutil.TestdataPath(t, "meepy-component", testutil.OS), "latest")
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	// Running 'dpm add oci://<uri>:<different tag>' should now essentially overwrite the tag to the new tag
	cmd := createStdTestRootCmd(t, "add", "component", "oci://"+newComponent+":4.5.6", "--insecure")
	require.NoError(t, cmd.Execute())

	damlPkg, err := damlpackage.Read(filepath.Join(projectDir, "daml.yaml"))
	require.NoError(t, err)

	assert.Len(t, damlPkg.ComponentsList, 2, "should not include more entries than it previously did")
	assert.Equal(t, "pre-existing:1.2.3", *damlPkg.ComponentsList[0].StringBased)
	assert.Contains(t, *damlPkg.ComponentsList[1].StringBased, "oci://"+newComponent+":4.5.6@sha256:")
	assert.NotContains(t, *damlPkg.ComponentsList[1].StringBased, oldSha256, "daml.yaml expected to not contain the old sha256 anymore")

	assertContainsComment(t, filepath.Join(projectDir, "daml.yaml"), "# 4.5.6")
}
