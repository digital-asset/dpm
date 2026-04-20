package damlpackage

import (
	"fmt"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDarDependencies(t *testing.T) {
	t.Setenv(assistantconfig.DpmLockfileEnabledEnvVar, "true")
	t.Setenv("TEST_DPM_REGISTRY_PORT", "5000")
	p, err := Read(testutil.TestdataPath(t, "daml-dependencies", "daml.yaml"))
	require.NoError(t, err)

	assert.Len(t, p.Dependencies, 4)
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

func makeDamlYaml(fields ...string) []byte {
	s := fmt.Sprintf("sdk-version: 3.4.5\n%s", strings.Join(fields, "\n"))
	return []byte(s)
}

func TestComponentFields(t *testing.T) {
	componentsField := `
components:
    - foo:4.5.6
`
	overrideComponentsField := `
override-components:
    damlc:
        version: 1.2.3
`

	t.Run("cannot use both fields together", func(t *testing.T) {
		_, err := ReadFromContents(makeDamlYaml(componentsField, overrideComponentsField))
		require.Error(t, err)
	})

	t.Run("populates components fields", func(t *testing.T) {
		p, err := ReadFromContents(makeDamlYaml(componentsField))
		require.NoError(t, err)
		assert.Contains(t, p.Components, "foo")
	})

	t.Run("populates components fields from override-components", func(t *testing.T) {
		p, err := ReadFromContents(makeDamlYaml(overrideComponentsField))
		require.NoError(t, err)
		assert.Contains(t, p.Components, "damlc")
	})

}
