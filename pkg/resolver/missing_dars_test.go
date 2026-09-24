package resolver

import (
	"errors"
	"net/url"
	"testing"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatMissingDarsError_releaseGroup(t *testing.T) {
	release := "test-release-0.0.6"
	cloneURL, err := url.Parse("https://github.com/Moonsong-Labs/daml-finance")
	require.NoError(t, err)

	var missing []*damlpackage.ParsedDarDependency
	for _, asset := range []string{"foo.dar", "bar.dar", "baz.dar"} {
		missing = append(missing, &damlpackage.ParsedDarDependency{
			FullUrl: mustParseGitReleaseURL(t, release, asset),
			Git: gitparse.GitSource{
				Ref:      release,
				DarPath:  asset,
				CloneURL: cloneURL,
				Release:  true,
			},
		})
	}

	err = formatMissingDarsError(missing)
	require.Error(t, err)

	var resErr *resolutionerrors.ResolutionError
	require.True(t, errors.As(err, &resErr))
	assert.Equal(t, resolutionerrors.DarNotInstalled, resErr.Code)
	assert.Contains(t, resErr.Cause.Error(), "3 git release assets from git:github.com/Moonsong-Labs/daml-finance?release="+release)
	assert.Contains(t, resErr.Cause.Error(), "dpm install package")
	assert.NotContains(t, resErr.Cause.Error(), "bar.dar\n")
}

func TestFormatMissingDarsError_singleDar(t *testing.T) {
	parsed, err := gitparse.ParseGitDependency("git:github.com/org/repo#main?path=foo.dar")
	require.NoError(t, err)
	dep := damlpackage.FromGit(parsed)

	err = formatMissingDarsError([]*damlpackage.ParsedDarDependency{dep})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git:github.com/org/repo#main?path=foo.dar")
}

func mustParseGitReleaseURL(t *testing.T, release, asset string) *url.URL {
	t.Helper()
	u, err := url.Parse("git://github.com/Moonsong-Labs/daml-finance@" + release + "?asset=" + asset)
	require.NoError(t, err)
	return u
}
