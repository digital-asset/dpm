package damlpackage

import (
	"testing"

	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDarDependencies(t *testing.T) {
	t.Setenv("TEST_DPM_REGISTRY_PORT", "5000")
	p, err := Read(testutil.TestdataPath(t, "daml-dependencies", "daml.yaml"))
	require.NoError(t, err)

	assert.Len(t, p.Dependencies, 5)
	assert.True(t, p.ArtifactLocations["@digital-asset"].Default)
	assert.False(t, p.ArtifactLocations["@my-location"].Default)

	assert.Len(t, p.ResolvedDependencies, len(p.Dependencies))

	assert.NotNil(t, p.ResolvedDependencies["foo:devnet"].Location)
	assert.NotNil(t, p.ResolvedDependencies["@my-location/foo:4.5.6"].Location)
	assert.Nil(t, p.ResolvedDependencies["oci://localhost:5000/some/dars/foo:1.2.3"].Location)

	assert.Equal(t, p.ResolvedDependencies["foo:devnet"].FullUrl.String(), "oci://localhost:5000/more/official/dars/foo:devnet")
	assert.Equal(t, p.ResolvedDependencies["oci://localhost:5000/some/dars/foo:1.2.3"].FullUrl.String(), "oci://localhost:5000/some/dars/foo:1.2.3")
	assert.Equal(t, p.ResolvedDependencies["@my-location/foo:4.5.6"].FullUrl.String(), "oci://localhost:5000/some/dars/n/stuff/foo:4.5.6")

}
