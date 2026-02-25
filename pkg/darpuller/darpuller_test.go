package darpuller

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestOciDarPuller(t *testing.T) {
	ctx := testutil.Context(t)

	registry := httptest.NewTLSServer(registry.New())
	defer registry.Close()

	version := "1.2.3"

	// TODO: using a PushComponent() for lack of a PushDar() for now
	testutil.PushComponent(t, ctx, registry, "meep", version, testutil.TestdataPath(t, "some-dar"), "latest")

	u, err := url.Parse(fmt.Sprintf("oci://%s/%s:%s",
		strings.TrimPrefix(registry.URL, "https://"),
		"components/meep",
		"latest",
	))
	if err != nil {
		require.NoError(t, err)
	}

	dar := &damlpackage.ResolvedDependency{
		FullUrl: u,
		Location: &damlpackage.ArtifactLocation{
			Client: &auth.Client{Client: registry.Client()},
		},
	}

	descriptor, v, destPath, err := fake(t).PullDar(ctx, dar)
	require.NoError(t, err)
	assert.NotEmpty(t, destPath)
	assert.NotEmpty(t, descriptor.Digest)
	assert.Equal(t, version, v.String())
}

func fake(t *testing.T) *OciDarPuller {
	tmpDamlHome := t.TempDir()

	config, err := assistantconfig.GetWithCustomDamlHome(tmpDamlHome)
	if err != nil {
		require.NoError(t, err)
	}
	if err := config.EnsureDirs(); err != nil {
		require.NoError(t, err)
	}

	return New(config)
}
