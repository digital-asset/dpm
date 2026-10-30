package damlpackage

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandReleaseGitDependenciesRaw(t *testing.T) {
	const tag = "v1.0.0"
	const assetA = "foo-1.0.0.dar"
	const assetB = "bar-1.0.0.dar"

	testutil.GitHubReleaseServer(t, false,
		testutil.GitHubReleaseAsset{Name: assetA},
		testutil.GitHubReleaseAsset{Name: assetB},
		testutil.GitHubReleaseAsset{Name: "readme.txt"},
	)

	releaseLine := "git:github.com/org/repo.git?release=" + tag
	otherLine := "oci://example.com/foo:1.0.0"

	expanded, err := ExpandReleaseGitDependenciesRaw(context.Background(), []*RawDependency{
		rawDependencyFromValue(otherLine),
		rawDependencyFromValue(releaseLine),
	})
	require.NoError(t, err)
	require.Len(t, expanded, 3)
	assert.Equal(t, otherLine, mustRawValue(t, expanded[0]))
	assert.Equal(t, gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, assetA), mustRawValue(t, expanded[1]))
	assert.Equal(t, gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, assetB), mustRawValue(t, expanded[2]))
}

func TestExpandGitReleaseDependenciesInYaml(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"

	testutil.GitHubReleaseServer(t, false, testutil.GitHubReleaseAsset{Name: asset})

	yamlPath := t.TempDir() + "/daml.yaml"
	releaseLine := "git:github.com/org/repo.git?release=" + tag
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - daml-script
  - ` + releaseLine + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)

	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", pkg.Dependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 2)
	line, err := got.Dependencies[1].Value()
	require.NoError(t, err)
	assert.Equal(t, gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, asset), line)
}

func TestExpandGitReleaseDependenciesInYaml_alias(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"

	testutil.GitHubReleaseServer(t, false, testutil.GitHubReleaseAsset{Name: asset})

	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`sdk-version: 3.4.5
artifact-locations:
  "@release-repo":
    url: git:github.com/org/repo.git
dependencies:
  - "@release-repo?release=` + tag + `"
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)

	expanded, err := ExpandGitReleaseDependenciesInYaml(
		context.Background(),
		yamlPath,
		"dependencies",
		pkg.Dependencies,
	)
	require.NoError(t, err)
	require.True(t, expanded, "a git release declared through an artifact-location alias should be expanded")

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 1)
	line, err := got.Dependencies[0].Value()
	require.NoError(t, err)
	assert.Equal(t, gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, asset), line)
}

func TestCanonicalizeGitDependenciesInYaml_afterExpandUsesFreshDeps(t *testing.T) {
	const tag = "v1.0.0"
	const assetA = "foo-1.0.0.dar"
	const assetB = "bar-1.0.0.dar"

	testutil.GitHubReleaseServer(t, false,
		testutil.GitHubReleaseAsset{Name: assetA},
		testutil.GitHubReleaseAsset{Name: assetB},
	)

	yamlPath := t.TempDir() + "/daml.yaml"
	releaseLine := "git:https://github.com/org/repo.git?release=" + tag
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - daml-script
  - ` + releaseLine + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	staleDeps := pkg.Dependencies

	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", staleDeps)
	require.NoError(t, err)
	require.True(t, expanded)

	freshPkg, err := Read(yamlPath)
	require.NoError(t, err)
	_, err = CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", freshPkg.Dependencies)
	require.NoError(t, err)

	yamlBytes, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	yamlText := string(yamlBytes)
	assert.Contains(t, yamlText, assetA)
	assert.Contains(t, yamlText, assetB)

	require.NoError(t, os.WriteFile(yamlPath, contents, 0644))
	_, err = ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", staleDeps)
	require.NoError(t, err)
	_, err = CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", staleDeps)
	require.NoError(t, err)
	staleYaml, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.NotContains(t, string(staleYaml), assetA)
	assert.NotContains(t, string(staleYaml), assetB)
	assert.Contains(t, string(staleYaml), "git:github.com/org/repo.git?release="+tag)
}

func TestExpandGitReleaseDependenciesInYaml_preservesMainPackageId(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"
	const pkgID = "0984ff5e3082add400bfcc6e3244bf9822ca5a617cfd92429e3fbce58058dbfa"

	testutil.GitHubReleaseServer(t, false, testutil.GitHubReleaseAsset{Name: asset})

	yamlPath := t.TempDir() + "/daml.yaml"
	releaseLine := "git:github.com/org/repo.git?release=" + tag
	contents := []byte(`sdk-version: 3.4.5
data-dependencies:
  - value: ` + releaseLine + `
    main-package-id: ` + pkgID + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)

	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "data-dependencies", pkg.DataDependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.DataDependencies, 1)
	require.NotNil(t, got.DataDependencies[0].WithPackageId)
	assert.Equal(t, pkgID, got.DataDependencies[0].WithPackageId.MainPackageId)
	line, err := got.DataDependencies[0].Value()
	require.NoError(t, err)
	assert.Equal(t, gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, asset), line)
}

func TestCanonicalizeRawGitDependencies_preservesMainPackageId(t *testing.T) {
	const pkgID = "abc123"
	raw := []*RawDependency{{
		WithPackageId: &withPackageId{
			Value:         "git:https://github.com/org/repo.git#main?path=foo.dar",
			MainPackageId: pkgID,
		},
	}}

	canonicalized, changed, err := CanonicalizeRawGitDependencies(raw)
	require.NoError(t, err)
	require.True(t, changed)
	require.Len(t, canonicalized, 1)
	require.NotNil(t, canonicalized[0].WithPackageId)
	assert.Equal(t, pkgID, canonicalized[0].WithPackageId.MainPackageId)
	assert.Equal(t, "git:github.com/org/repo.git#main?path=foo.dar", canonicalized[0].WithPackageId.Value)
}

func TestExpandGitReleaseDependenciesInYaml_doesNotDuplicateExistingAssets(t *testing.T) {
	const tag = "v1.0.0"
	const assetA = "foo.dar"
	const assetB = "bar.dar"

	testutil.GitHubReleaseServer(t, false,
		testutil.GitHubReleaseAsset{Name: assetA},
		testutil.GitHubReleaseAsset{Name: assetB},
	)

	yamlPath := t.TempDir() + "/daml.yaml"
	baseLine := "git:github.com/org/repo.git?release=" + tag
	assetLine := gitparse.FormatGitReleaseLine("github.com/org/repo.git", tag, assetA)
	contents := []byte(`data-dependencies:
  - ` + assetLine + `
  - ` + baseLine + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "data-dependencies", pkg.DataDependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.DataDependencies, 2)
	values := make([]string, 0, len(got.DataDependencies))
	for _, raw := range got.DataDependencies {
		value, err := raw.Value()
		require.NoError(t, err)
		values = append(values, value)
	}
	assert.Equal(t, 1, strings.Count(strings.Join(values, "\n"), assetA))
	assert.Equal(t, 1, strings.Count(strings.Join(values, "\n"), assetB))
	assert.NotContains(t, strings.Join(values, "\n"), baseLine)
}

func TestExpandGitReleaseDependenciesInYaml_preservesComments(t *testing.T) {
	const tag = "v1.0.0"
	const assetA = "foo.dar"
	const assetB = "bar.dar"

	testutil.GitHubReleaseServer(t, false,
		testutil.GitHubReleaseAsset{Name: assetA},
		testutil.GitHubReleaseAsset{Name: assetB},
	)

	yamlPath := t.TempDir() + "/daml.yaml"
	baseLine := "git:github.com/org/repo.git?release=" + tag
	contents := []byte(`data-dependencies:
  # unrelated head comment
  - local.dar # unrelated line comment
  - ` + baseLine + ` # release line comment
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "data-dependencies", pkg.DataDependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	text := string(got)
	assert.Contains(t, text, "# unrelated head comment")
	assert.Contains(t, text, "# unrelated line comment")
	assert.Equal(t, 1, strings.Count(text, "# release line comment"))
	assert.Contains(t, text, assetA)
	assert.Contains(t, text, assetB)
}

func TestCanonicalizeGitDependenciesInYaml_resolvesImmutableAlias(t *testing.T) {
	const commit = "0123456789012345678901234567890123456789"

	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`artifact-locations:
  "@shared":
    url: git:github.com/org/repo.git
data-dependencies:
  - "@shared#` + commit + `?path=foo.dar" # keep this comment
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	changed, err := CanonicalizeGitDependenciesInYaml(yamlPath, "data-dependencies", pkg.DataDependencies)
	require.NoError(t, err)
	require.True(t, changed)

	got, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	text := string(got)
	assert.NotContains(t, text, `"@shared#`)
	assert.Contains(t, text, "git:github.com/org/repo.git#"+commit+"?path=foo.dar")
	assert.Contains(t, text, "# keep this comment")
}

func TestCanonicalizeGitDependenciesInYaml_rejectsNonGitHubReleaseWithoutEditing(t *testing.T) {
	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - daml-script
  - git:https://gitlab.com/org/repo.git?release=v1.0.0&asset=foo.dar
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)

	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", pkg.Dependencies)
	require.Error(t, err)
	assert.False(t, expanded)
	assert.Contains(t, err.Error(), "only supported for github.com")

	afterExpand, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, string(contents), string(afterExpand))

	_, err = CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", pkg.Dependencies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for github.com")

	afterCanonicalize, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, string(contents), string(afterCanonicalize),
		"canonicalize must not rewrite daml.yaml before rejecting a non-GitHub release")
}

func TestCanonicalizeGitDependenciesInYaml_keepsStructuredGit(t *testing.T) {
	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`sdk-version: 3.6.0
dependencies:
  - git: # keep this comment
      url: https://github.com/org/repo.git
      ref: main
      path: pkg/foo.dar
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	changed, err := CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", pkg.Dependencies)
	require.NoError(t, err)
	require.True(t, changed)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 1)
	require.NotNil(t, got.Dependencies[0].GitStructured)
	assert.Nil(t, got.Dependencies[0].ValueOnly)
	assert.Equal(t, "github.com/org/repo.git", got.Dependencies[0].GitStructured.URL)
	assert.Equal(t, "main", got.Dependencies[0].GitStructured.Ref)
	assert.Equal(t, "pkg/foo.dar", got.Dependencies[0].GitStructured.Path)

	text, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.NotContains(t, string(text), "git:github.com")
	assert.Contains(t, string(text), "# keep this comment")

	again, err := CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", got.Dependencies)
	require.NoError(t, err)
	assert.False(t, again)
}

func TestMarshalDependencyWithValue_pinsStructuredRef(t *testing.T) {
	const pinned = "0123456789012345678901234567890123456789"
	raw := &RawDependency{GitStructured: &gitparse.GitStructuredFields{
		URL:  "github.com/org/repo.git",
		Ref:  "main",
		Path: "pkg/foo.dar",
	}}
	pinnedLine := gitparse.FormatGitYamlLine(gitparse.GitSource{
		Ref:      pinned,
		DarPath:  "pkg/foo.dar",
		CloneURL: mustCloneURL(t, "https://github.com/org/repo.git"),
	})

	out, err := MarshalDependencyWithValue(raw, pinnedLine)
	require.NoError(t, err)

	var got RawDependency
	require.NoError(t, yamlUnmarshalDependency(t, out, &got))
	require.NotNil(t, got.GitStructured)
	assert.Nil(t, got.ValueOnly)
	assert.Equal(t, "github.com/org/repo.git", got.GitStructured.URL)
	assert.Equal(t, pinned, got.GitStructured.Ref)
	assert.Equal(t, "pkg/foo.dar", got.GitStructured.Path)
	assert.Empty(t, got.GitStructured.Release)
	assert.Empty(t, got.GitStructured.Asset)
}

func TestCanonicalizeRawGitDependencies_preservesStructuredMainPackageId(t *testing.T) {
	const pkgID = "abc123"
	raw := []*RawDependency{{
		GitStructured: &gitparse.GitStructuredFields{
			URL:  "https://github.com/org/repo.git",
			Ref:  "main",
			Path: "foo.dar",
		},
		GitMainPackageId: ptr(pkgID),
	}}

	canonicalized, changed, err := CanonicalizeRawGitDependencies(raw)
	require.NoError(t, err)
	require.True(t, changed)
	require.Len(t, canonicalized, 1)
	require.NotNil(t, canonicalized[0].GitMainPackageId)
	assert.Equal(t, pkgID, *canonicalized[0].GitMainPackageId)
	assert.Equal(t, "github.com/org/repo.git", canonicalized[0].GitStructured.URL)

	again, changedAgain, err := CanonicalizeRawGitDependencies(canonicalized)
	require.NoError(t, err)
	assert.False(t, changedAgain)
	require.NotNil(t, again[0].GitMainPackageId)
	assert.Equal(t, pkgID, *again[0].GitMainPackageId)
}

func TestMarshalDependencyWithValue_pinsStructuredRefAndKeepsMainPackageId(t *testing.T) {
	const pinned = "0123456789012345678901234567890123456789"
	const pkgID = "abc123"
	raw := &RawDependency{
		GitStructured: &gitparse.GitStructuredFields{
			URL:  "github.com/org/repo.git",
			Ref:  "main",
			Path: "pkg/foo.dar",
		},
		GitMainPackageId: ptr(pkgID),
	}
	pinnedLine := gitparse.FormatGitYamlLine(gitparse.GitSource{
		Ref:      pinned,
		DarPath:  "pkg/foo.dar",
		CloneURL: mustCloneURL(t, "https://github.com/org/repo.git"),
	})

	out, err := MarshalDependencyWithValue(raw, pinnedLine)
	require.NoError(t, err)
	assert.Contains(t, out, "main-package-id: "+pkgID)
	assert.NotContains(t, out, "ref: main")

	var got RawDependency
	require.NoError(t, yamlUnmarshalDependency(t, out, &got))
	require.NotNil(t, got.GitStructured)
	assert.Equal(t, pinned, got.GitStructured.Ref)
	require.NotNil(t, got.GetMainPackageId())
	assert.Equal(t, pkgID, *got.GetMainPackageId())
}

func TestExpandGitReleaseDependenciesInYaml_keepsStructuredGit(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"

	testutil.GitHubReleaseServer(t, false, testutil.GitHubReleaseAsset{Name: asset})

	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`sdk-version: 3.6.0
dependencies:
  - git:
      url: https://github.com/org/repo.git
      release: ` + tag + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", pkg.Dependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 1)
	require.NotNil(t, got.Dependencies[0].GitStructured)
	assert.Nil(t, got.Dependencies[0].ValueOnly)
	assert.Equal(t, tag, got.Dependencies[0].GitStructured.Release)
	assert.Equal(t, asset, got.Dependencies[0].GitStructured.Asset)
	assert.Empty(t, got.Dependencies[0].GitStructured.Ref)
	assert.Empty(t, got.Dependencies[0].GitStructured.Path)

	text, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.NotContains(t, string(text), "git:github.com")
}

func TestExpandGitReleaseDependenciesInYaml_keepsStructuredMainPackageId(t *testing.T) {
	const tag = "v1.0.0"
	const asset = "pkg-1.0.0.dar"
	const pkgID = "abc123"

	testutil.GitHubReleaseServer(t, false, testutil.GitHubReleaseAsset{Name: asset})

	yamlPath := t.TempDir() + "/daml.yaml"
	contents := []byte(`sdk-version: 3.6.0
dependencies:
  - git:
      url: https://github.com/org/repo.git
      release: ` + tag + `
    main-package-id: ` + pkgID + `
`)
	require.NoError(t, os.WriteFile(yamlPath, contents, 0o644))

	pkg, err := Read(yamlPath)
	require.NoError(t, err)
	require.NotNil(t, pkg.Dependencies[0].GetMainPackageId())

	expanded, err := ExpandGitReleaseDependenciesInYaml(context.Background(), yamlPath, "dependencies", pkg.Dependencies)
	require.NoError(t, err)
	require.True(t, expanded)

	got, err := Read(yamlPath)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 1)
	require.NotNil(t, got.Dependencies[0].GetMainPackageId())
	assert.Equal(t, pkgID, *got.Dependencies[0].GetMainPackageId())
	assert.Equal(t, asset, got.Dependencies[0].GitStructured.Asset)

	text, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.Contains(t, string(text), "main-package-id: "+pkgID)
	assert.NotContains(t, string(text), "git:github.com")
}

func yamlUnmarshalDependency(t *testing.T, raw string, dest *RawDependency) error {
	t.Helper()
	return yaml.Unmarshal([]byte(raw), dest)
}

func mustCloneURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func ptr(s string) *string { return &s }

func mustRawValue(t *testing.T, raw *RawDependency) string {
	t.Helper()
	value, err := raw.Value()
	require.NoError(t, err)
	return value
}
