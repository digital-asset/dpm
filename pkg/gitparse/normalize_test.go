package gitparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoerceGitDependencyInput_githubBlobURLs(t *testing.T) {
	t.Parallel()

	const want = "git:github.com/gonzamontiel/test-daml-hello#master?path=dist/test-daml-hello-0.0.1.dar"

	cases := []struct {
		name string
		raw  string
		opts GitInputOptions
	}{
		{
			name: "https blob without git prefix",
			raw:  "https://github.com/gonzamontiel/test-daml-hello/blob/master/dist/test-daml-hello-0.0.1.dar",
			opts: GitInputOptions{RequireGitPrefix: false},
		},
		{
			name: "https blob with git prefix",
			raw:  "git:https://github.com/gonzamontiel/test-daml-hello/blob/master/dist/test-daml-hello-0.0.1.dar",
			opts: GitInputOptions{RequireGitPrefix: true},
		},
		{
			name: "host-first blob without git prefix",
			raw:  "github.com/gonzamontiel/test-daml-hello/blob/master/dist/test-daml-hello-0.0.1.dar",
			opts: GitInputOptions{RequireGitPrefix: false},
		},
		{
			name: "host-first blob with git prefix",
			raw:  "git:github.com/gonzamontiel/test-daml-hello/blob/master/dist/test-daml-hello-0.0.1.dar",
			opts: GitInputOptions{RequireGitPrefix: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CoerceGitDependencyInput(tc.raw, tc.opts)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestNormalizeDarDependencyURI_githubBlobWithoutGitPrefix(t *testing.T) {
	t.Parallel()

	raw := "https://github.com/gonzamontiel/test-daml-hello/blob/master/dist/test-daml-hello-0.0.1.dar"
	normalized, isGit, err := NormalizeDarDependencyURI(raw)
	require.NoError(t, err)
	assert.True(t, isGit)
	assert.Equal(t, "git:github.com/gonzamontiel/test-daml-hello#master?path=dist/test-daml-hello-0.0.1.dar", normalized)
}

func TestCoerceGitDependencyInput_githubRawURLs(t *testing.T) {
	t.Parallel()

	const wantMain = "git:github.com/canton-network/splice#main?path=daml/dars/splice-amulet-0.1.19.dar"
	const wantTag = "git:github.com/canton-network/splice#0.6.10?path=daml/dars/splice-amulet-0.1.19.dar"

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "raw refs/heads",
			raw:  "https://github.com/canton-network/splice/raw/refs/heads/main/daml/dars/splice-amulet-0.1.19.dar",
			want: wantMain,
		},
		{
			name: "raw refs/tags",
			raw:  "https://github.com/canton-network/splice/raw/refs/tags/0.6.10/daml/dars/splice-amulet-0.1.19.dar",
			want: wantTag,
		},
		{
			name: "raw single-segment ref",
			raw:  "https://github.com/canton-network/splice/raw/main/daml/dars/splice-amulet-0.1.19.dar",
			want: wantMain,
		},
		{
			name: "host-first raw refs/heads",
			raw:  "github.com/canton-network/splice/raw/refs/heads/main/daml/dars/splice-amulet-0.1.19.dar",
			want: wantMain,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CoerceGitDependencyInput(tc.raw, GitInputOptions{RequireGitPrefix: false})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
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

func TestParseGitDependency_gitLabReleaseParses(t *testing.T) {
	t.Parallel()

	dep, err := ParseGitDependency("git:gitlab.com/org/repo?release=v1.0.0&asset=foo.dar")
	require.NoError(t, err)
	assert.True(t, dep.Git.Release)
}
