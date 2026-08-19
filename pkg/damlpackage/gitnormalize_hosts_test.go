package damlpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			_, parseErr := ParseGitDependency(line)
			require.NoError(t, parseErr, "ParseGitDependency should accept any https host")

			_, _, coerceErr := CanonicalizeGitDependencyLines([]string{line})
			require.NoError(t, coerceErr, "canonicalization must accept whatever parsing accepts")

			normalized, isGit, err := NormalizeDarDependencyURI(line)
			require.NoError(t, err)
			assert.True(t, isGit)

			again, changed, err := CanonicalizeGitDependencyLines([]string{normalized})
			require.NoError(t, err)
			assert.False(t, changed)
			assert.Equal(t, []string{normalized}, again)
		})
	}
}

func TestCoerceGitDependencyInput_nonGitHubHosts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		opts GitInputOptions
		want string
	}{
		{
			name: "gitlab host-first",
			raw:  "git:gitlab.com/org/repo#main?path=foo.dar",
			opts: GitInputOptions{RequireGitPrefix: true},
			want: "git:gitlab.com/org/repo#main?path=foo.dar",
		},
		{
			name: "gitlab https clone url is shortened to host-first",
			raw:  "git:https://gitlab.com/org/repo.git#main?path=foo.dar",
			opts: GitInputOptions{RequireGitPrefix: true},
			want: "git:gitlab.com/org/repo#main?path=foo.dar",
		},
		{
			name: "gitlab blob url with /-/ separator",
			raw:  "https://gitlab.com/org/repo/-/blob/main/dist/foo.dar",
			opts: GitInputOptions{RequireGitPrefix: false},
			want: "git:gitlab.com/org/repo#main?path=dist/foo.dar",
		},
		{
			name: "gitlab raw url with nested groups",
			raw:  "https://gitlab.com/group/subgroup/repo/-/raw/v1.0.0/foo.dar",
			opts: GitInputOptions{RequireGitPrefix: false},
			want: "git:gitlab.com/group/subgroup/repo#v1.0.0?path=foo.dar",
		},
		{
			name: "self-hosted gitea blob url",
			raw:  "https://git.example.com/org/repo/blob/main/foo.dar",
			opts: GitInputOptions{RequireGitPrefix: false},
			want: "git:git.example.com/org/repo#main?path=foo.dar",
		},
		{
			name: "codeberg host-first",
			raw:  "git:codeberg.org/org/repo#main?path=foo.dar",
			opts: GitInputOptions{RequireGitPrefix: true},
			want: "git:codeberg.org/org/repo#main?path=foo.dar",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CoerceGitDependencyInput(tc.raw, tc.opts)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCoerceGitDependencyInput_rejectsNonGitHubRelease(t *testing.T) {
	t.Parallel()

	_, err := CoerceGitDependencyInput(
		"git:gitlab.com/org/repo.git?release=v1.0.0&asset=foo.dar",
		GitInputOptions{RequireGitPrefix: true},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for github.com")
	assert.Contains(t, err.Error(), "?path=")
}

func TestNormalizeDarDependencyURI_leavesNonGitInputsAlone(t *testing.T) {
	t.Parallel()

	cases := []string{
		"oci://registry.example.com/org/image:1.0",
		"registry.example.com/org/image:1.0",
		"registry.example.com:5000/org/image:1.0",
		"ghcr.io/org/image:1.0",
		"gitlab.com/org/repo",
		"daml-script",
		"./local/foo.dar",
	}

	for _, uri := range cases {
		t.Run(uri, func(t *testing.T) {
			normalized, isGit, err := NormalizeDarDependencyURI(uri)
			require.NoError(t, err)
			assert.False(t, isGit, "%q must not be treated as a git dependency", uri)
			assert.Equal(t, uri, normalized)
		})
	}
}

func TestParseGitDependency_gitLabReleaseIsRejectedWithGuidance(t *testing.T) {
	t.Parallel()

	dep, err := ParseGitDependency("git:gitlab.com/org/repo?release=v1.0.0&asset=foo.dar")
	require.NoError(t, err)
	assert.True(t, dep.GitRelease)

	_, err = ExpandReleaseGitDependencies(t.Context(), []string{"git:gitlab.com/org/repo?release=v1.0.0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for github.com")
	assert.Contains(t, err.Error(), "?path=")
}
