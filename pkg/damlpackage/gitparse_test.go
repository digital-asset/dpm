package damlpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitDependency_inlineConflictingFieldsInYaml(t *testing.T) {
	raw := "git:github.com/org/repo.git?release=v1.0.0&asset=bar.dar#main?path=dist/foo.dar"
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - ` + raw + `
`)
	_, err := ReadFromContents(contents, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release cannot be combined with ref or path")
}

func TestGitDependency_inDataDependencies(t *testing.T) {
	raw := "git:https://github.com/example-org/example-repo.git#main?path=pkg/foo.dar"
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - daml-script
data-dependencies:
  - ` + raw + `
`)
	p, err := ReadFromContents(contents, "")
	require.NoError(t, err)
	dep, ok := p.ParsedDarDependencies.DataDependencies[raw]
	require.True(t, ok)
	assert.Equal(t, "main", dep.Git.Ref)
	assert.Equal(t, "pkg/foo.dar", dep.Git.DarPath)
}

func TestGitDependency_inDependencies(t *testing.T) {
	raw := "git:https://github.com/example-org/example-repo.git#main?path=pkg/foo.dar"
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - ` + raw + `
`)
	p, err := ReadFromContents(contents, "")
	require.NoError(t, err)
	dep, ok := p.ParsedDarDependencies.Dependencies[raw]
	require.True(t, ok)
	assert.Equal(t, "main", dep.Git.Ref)
	assert.Equal(t, "pkg/foo.dar", dep.Git.DarPath)
}

func TestParseGitStructuredDependency_rejected(t *testing.T) {
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - git:
      url: https://github.com/org/repo.git
      ref: main
      path: pkg/foo.dar
`)
	_, err := ReadFromContents(contents, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, RawDependenciesSchemaErr)
}

func TestGitDependencyHostsAgreeAcrossLayers(t *testing.T) {
	t.Parallel()

	lines := []string{
		"git:gitlab.com/calvogenerico/daml-temp-coso#3000cf452734676e9c87d0a92d1bad38a3f16ec3?path=foo.dar",
		"git:github.com/org/repo#main?path=dist/foo.dar",
		"git:bitbucket.org/org/repo#main?path=foo.dar",
		"git:git.internal.example.com/team/repo#v1.2.3?path=out/foo.dar",
		"git:https://gitlab.example.com/group/subgroup/repo.git#main?path=foo.dar",
	}

	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			contents := []byte("sdk-version: 3.4.5\ndependencies:\n  - " + line + "\n")
			p, err := ReadFromContents(contents, "")
			require.NoError(t, err, "package parse should accept any https host")
			require.Len(t, p.Dependencies, 1)

			_, _, coerceErr := CanonicalizeRawGitDependencies([]*RawDependency{rawDependencyFromValue(line)})
			require.NoError(t, coerceErr, "canonicalization must accept whatever parsing accepts")
		})
	}
}

func TestParseGitDependency_gitLabReleaseIsRejectedWithGuidance(t *testing.T) {
	t.Parallel()

	_, err := ExpandReleaseGitDependenciesRaw(t.Context(), []*RawDependency{
		rawDependencyFromValue("git:gitlab.com/org/repo?release=v1.0.0"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for github.com")
	assert.Contains(t, err.Error(), "?path=")
}
