package githubrelease

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDarAssets_andDownload(t *testing.T) {
	const tag = "ContingentClaims.Core/2.0.0"
	const asset = "contingent-claims-core-2.0.0.dar"
	darBody := []byte("fake dar bytes")

	host := testutil.GitHubReleaseServer(t, true,
		testutil.GitHubReleaseAsset{Name: asset, Body: darBody},
		testutil.GitHubReleaseAsset{Name: asset + ".asc"},
	)

	cloneURL, err := url.Parse("http://" + host + "/digital-asset/daml-finance.git")
	require.NoError(t, err)

	ctx := context.Background()
	assets, err := ListDarAssets(ctx, cloneURL, tag)
	require.NoError(t, err)
	assert.Equal(t, []string{asset}, assets)

	dir := t.TempDir()
	path, err := DownloadAsset(ctx, cloneURL, tag, asset, dir)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, darBody, got)
}

func TestNotFoundErrorsOmitResponseBody(t *testing.T) {
	const htmlBody = `<!DOCTYPE html><html><head><title>Page not found</title></head>` +
		`<body><div class="container">much more markup</div></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	host := srv.Listener.Addr().String()
	t.Setenv("DPM_TEST_GITHUB_API_BASE", srv.URL)
	t.Setenv("DPM_TEST_GITHUB_RELEASE_HOST", host)

	cloneURL, err := url.Parse("http://" + host + "/digital-asset/daml-finance.git")
	require.NoError(t, err)

	ctx := context.Background()

	_, err = ListDarAssets(ctx, cloneURL, "no-such-release")
	require.Error(t, err)
	assert.Equal(t, `release "no-such-release" not found for digital-asset/daml-finance`, err.Error())

	_, err = DownloadAsset(ctx, cloneURL, "no-such-release", "missing.dar", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `asset "missing.dar" not found in digital-asset/daml-finance release "no-such-release"`)
	assert.NotContains(t, err.Error(), "<")
	assert.NotContains(t, err.Error(), "DOCTYPE")
}

func TestParseGitHubRepo(t *testing.T) {
	t.Parallel()

	valid := []string{
		"github.com/digital-asset/daml-finance.git",
		"github.com/digital-asset/daml-finance",
	}
	for _, raw := range valid {
		t.Run("valid "+raw, func(t *testing.T) {
			t.Parallel()
			u, err := url.Parse(raw)
			require.NoError(t, err)
			owner, repo, err := ParseGitHubRepo(u)
			require.NoError(t, err)
			assert.Equal(t, "digital-asset", owner)
			assert.Equal(t, "daml-finance", repo)
		})
	}

	invalid := []string{
		"github.com/digital-asset/daml-finance/extra.git",
		"github.com/digital-asset",
		"github.com/",
	}
	for _, raw := range invalid {
		t.Run("invalid "+raw, func(t *testing.T) {
			t.Parallel()
			u, err := url.Parse(raw)
			require.NoError(t, err)
			_, _, err = ParseGitHubRepo(u)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "couldn't parse github owner/repo")
		})
	}
}

func TestReleaseDownloadURL_escapesTagAndAsset(t *testing.T) {
	cloneURL, err := url.Parse("github.com/digital-asset/daml-finance.git")
	require.NoError(t, err)

	got := releaseDownloadURL(cloneURL, "ContingentClaims.Core/2.0.0", "contingent-claims-core-2.0.0.dar")
	assert.Equal(t,
		"github.com/digital-asset/daml-finance/releases/download/ContingentClaims.Core%2F2.0.0/contingent-claims-core-2.0.0.dar",
		got,
	)
}
