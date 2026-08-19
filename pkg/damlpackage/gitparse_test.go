package damlpackage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitDependency(t *testing.T) {
	t.Parallel()

	raw := "git:github.com/org/repo.git#main?path=loyalty.dar"
	dep, err := ParseGitDependency(raw)
	require.NoError(t, err)
	assert.Equal(t, "main", dep.GitRef)
	assert.Equal(t, "loyalty.dar", dep.DarPath)
	assert.Equal(t, "https", dep.CloneURL.Scheme)
	assert.Equal(t, "github.com", dep.CloneURL.Host)
	assert.Equal(t, "/org/repo.git", dep.CloneURL.Path)
	assert.Equal(t, "git", dep.Scheme())
}

func TestParseGitDependency_pinnedRef(t *testing.T) {
	t.Parallel()

	raw := "git:https://github.com/org/repo.git#" + strings.Repeat("a", 40) + "?path=loyalty.dar"
	dep, err := ParseGitDependency(raw)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 40), dep.GitRef)
	assert.False(t, GitRefIsMutable(dep.GitRef))
}

func TestParseGitDependency_releaseInRepoPath(t *testing.T) {
	t.Parallel()

	raw := "git:https://github.com/org/release=foo.git#main?path=loyalty.dar"
	dep, err := ParseGitDependency(raw)
	require.NoError(t, err)
	assert.Equal(t, "main", dep.GitRef)
	assert.Equal(t, "loyalty.dar", dep.DarPath)
	assert.False(t, dep.GitRelease)
}

func TestParseGitReleaseDependency_emptyAsset(t *testing.T) {
	t.Parallel()

	const tag = "v1.0.0"
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "missing asset param",
			raw:  "git:https://github.com/org/repo.git?release=" + tag,
		},
		{
			name: "explicit empty asset param",
			raw:  "git:https://github.com/org/repo.git?release=" + tag + "&asset=",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dep, err := ParseGitDependency(tc.raw)
			require.NoError(t, err)
			assert.True(t, dep.GitRelease)
			assert.Equal(t, tag, dep.GitRef)
			assert.Empty(t, dep.DarPath)
		})
	}
}

func TestParseGitReleaseDependency_invalidAsset(t *testing.T) {
	t.Parallel()

	raw := "git:https://github.com/org/repo.git?release=v1.0.0&asset=readme.txt"
	_, err := ParseGitDependency(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must end with .dar")
}

func TestParseGitDependency_errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
	}{
		{"missing path", "git:https://github.com/org/repo.git#main"},
		{"missing ref", "git:https://github.com/org/repo.git?path=loyalty.dar"},
		{"not git prefix", "https://github.com/org/repo.git#main?path=a.dar"},
		{"parent path", "git:https://github.com/org/repo.git#main?path=../outside.dar"},
		{"non-dar path", "git:https://github.com/org/repo.git#main?path=README.md"},
		{"unsupported scheme", "git:ssh://github.com/org/repo.git#main?path=loyalty.dar"},
		{"missing repo path", "git:github.com#main?path=loyalty.dar"},
		{
			"release with ref and path",
			"git:github.com/org/repo.git?release=v1.0.0&asset=bar.dar#main?path=dist/foo.dar",
		},
		{
			"asset without release",
			"git:github.com/org/repo.git?asset=bar.dar#main?path=foo.dar",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGitDependency(tc.raw)
			require.Error(t, err)
		})
	}
}

func TestCoerceGitDependencyInput_rejectsConflictingInlineFields(t *testing.T) {
	t.Parallel()

	raw := "git:github.com/org/repo.git?release=v1.0.0&asset=bar.dar#main?path=dist/foo.dar"
	_, err := CoerceGitDependencyInput(raw, GitInputOptions{RequireGitPrefix: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release cannot be combined with ref or path")
}

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
	assert.Equal(t, "main", dep.GitRef)
	assert.Equal(t, "pkg/foo.dar", dep.DarPath)
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
	assert.Equal(t, "main", dep.GitRef)
	assert.Equal(t, "pkg/foo.dar", dep.DarPath)
}

func TestFormatGitYamlLine(t *testing.T) {
	dep, err := ParseGitDependency("git:github.com/org/repo.git#main?path=foo.dar")
	require.NoError(t, err)
	pinned := dep.WithGitRef("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	assert.Equal(t,
		"git:github.com/org/repo#deadbeefdeadbeefdeadbeefdeadbeefdeadbeef?path=foo.dar",
		FormatGitYamlLine(pinned),
	)
}

func TestFormatGitYamlLine_writesCanonicalPath(t *testing.T) {
	dep, err := ParseGitDependency("git:github.com/org/repo.git#main?path=pkg%2Ffoo.dar")
	require.NoError(t, err)
	assert.Equal(t, "pkg/foo.dar", dep.DarPath)
	assert.Equal(t,
		"git:github.com/org/repo#main?path=pkg/foo.dar",
		FormatGitYamlLine(dep),
	)
}

func TestFormatGitYamlLineIsFixedPointOfCoerce(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"git:github.com/org/repo.git#main?path=pkg%2Ffoo.dar",
		"git:github.com/org/repo#main?path=pkg/foo.dar",
		"git:https://github.com/org/repo.git#main?path=foo.dar",
		"git:gitlab.com/group/subgroup/repo.git#v1.2.3?path=out/foo.dar",
		"git:git.example.com/team/repo#main?path=foo.dar",
		"git:github.com/org/repo.git?release=v1.0.0",
		"git:github.com/org/repo?release=v1.0.0&asset=foo.dar",
		"git:github.com/org/repo#main?path=foo%2Bbar.dar",
		"git:github.com/org/repo#main?path=build%2B1/foo.dar",
		"git:github.com/org/repo#main?path=100%25.dar",
		"git:github.com/org/repo?release=v1.0.0&asset=foo%2Bbar.dar",
		"https://gitlab.com/org/repo/-/blob/main/dist/foo.dar",
		"https://github.com/org/repo/raw/refs/tags/v1.0.0/dist/foo.dar",
	}

	for _, raw := range inputs {
		t.Run(raw, func(t *testing.T) {
			canonical, err := CoerceGitDependencyInput(raw, GitInputOptions{})
			require.NoError(t, err)

			dep, err := ParseGitDependency(canonical)
			require.NoError(t, err)
			assert.Equal(t, canonical, FormatGitYamlLine(dep),
				"formatting a parsed dependency must reproduce the canonical line")

			again, err := CoerceGitDependencyInput(canonical, GitInputOptions{RequireGitPrefix: true})
			require.NoError(t, err)
			assert.Equal(t, canonical, again, "normalization must be idempotent")
		})
	}
}

func TestGitDarPathSurvivesCanonicalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantDarPath string
	}{
		{
			name:        "plus is a literal, not an encoded space",
			raw:         "git:github.com/org/repo#main?path=foo%2Bbar.dar",
			wantDarPath: "foo+bar.dar",
		},
		{
			name:        "percent is a literal, not an escape sequence",
			raw:         "git:github.com/org/repo#main?path=100%25.dar",
			wantDarPath: "100%.dar",
		},
		{
			name:        "encoded slashes still decode to directories",
			raw:         "git:github.com/org/repo#main?path=dist%2Ffoo.dar",
			wantDarPath: "dist/foo.dar",
		},
		{
			name:        "blob url path is re-escaped when it becomes a query",
			raw:         "https://github.com/org/repo/blob/main/dist/foo%2Bbar.dar",
			wantDarPath: "dist/foo+bar.dar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical, err := CoerceGitDependencyInput(tt.raw, GitInputOptions{})
			require.NoError(t, err)

			dep, err := ParseGitDependency(canonical)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDarPath, dep.DarPath)

			recoerced, err := CoerceGitDependencyInput(canonical, GitInputOptions{RequireGitPrefix: true})
			require.NoError(t, err)
			redep, err := ParseGitDependency(recoerced)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDarPath, redep.DarPath,
				"re-normalizing must not change the resolved dar path")
		})
	}

	t.Run("structured path scalar is escaped", func(t *testing.T) {
		line, err := FormatGitStructuredLine(&GitStructuredFields{
			URL:  "github.com/org/repo",
			Ref:  "main",
			Path: "dist/foo+bar.dar",
		})
		require.NoError(t, err)

		dep, err := ParseGitDependency(line)
		require.NoError(t, err)
		assert.Equal(t, "dist/foo+bar.dar", dep.DarPath)
	})
}

func TestParseGitStructuredDependency(t *testing.T) {
	contents := []byte(`sdk-version: 3.4.5
dependencies:
  - git:
      url: https://github.com/org/repo.git
      ref: main
      path: pkg/foo.dar
`)
	p, err := ReadFromContents(contents, "")
	require.NoError(t, err)
	require.Len(t, p.ParsedDarDependencies.Dependencies, 1)
	dep := loFirstDependency(p)
	assert.Equal(t, "main", dep.GitRef)
	assert.Equal(t, "pkg/foo.dar", dep.DarPath)
}

func TestParseGitStructuredDependency_conflictingFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
		err  string
	}{
		{
			name: "release with ref and path",
			yaml: `sdk-version: 3.4.5
dependencies:
  - git:
      url: https://github.com/org/repo.git
      ref: main
      path: dist/foo.dar
      release: v1.0.0
      asset: bar.dar
`,
			err: "release cannot be combined with ref or path",
		},
		{
			name: "asset without release",
			yaml: `sdk-version: 3.4.5
dependencies:
  - git:
      url: https://github.com/org/repo.git
      ref: main
      path: pkg/foo.dar
      asset: bar.dar
`,
			err: "asset requires release",
		},
		{
			name: "path without ref",
			yaml: `sdk-version: 3.4.5
dependencies:
  - git:
      url: https://github.com/org/repo.git
      path: pkg/foo.dar
`,
			err: "path requires ref",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadFromContents([]byte(tc.yaml), "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
		})
	}
}

func TestGitLockKeyForDep_normalizesGitSuffix(t *testing.T) {
	t.Parallel()

	withSuffix, err := ParseGitDependency("git:github.com/org/repo.git#main?path=foo.dar")
	require.NoError(t, err)
	withoutSuffix, err := ParseGitDependency("git:github.com/org/repo#main?path=foo.dar")
	require.NoError(t, err)

	keyA, err := GitLockKeyForDep(withSuffix)
	require.NoError(t, err)
	keyB, err := GitLockKeyForDep(withoutSuffix)
	require.NoError(t, err)
	assert.Equal(t, keyA, keyB)
}

func loFirstDependency(p *DamlPackage) *ParsedDarDependency {
	for _, d := range p.ParsedDarDependencies.Dependencies {
		return d
	}
	return nil
}
