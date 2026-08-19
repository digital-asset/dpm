package damlpackage

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
