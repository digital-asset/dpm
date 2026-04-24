package componentlist

import (
	"testing"

	"daml.com/x/assistant/pkg/componentlist/testdata"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Manifest struct {
	Components ComponentList `yaml:"components"`
}

func TestComponentList(t *testing.T) {
	m := Manifest{}
	require.NoError(t, yaml.Unmarshal(testdata.Valid, &m))

	cs, err := m.Components.ToMap()
	require.NoError(t, err)

	assert.Len(t, cs, 4)

	path := "./meep"
	assert.Equal(t, &sdkmanifest.Component{Name: "meep", LocalPath: &path}, cs["meep"])

	version, _ := semver.StrictNewVersion("1.2.3")
	assert.Equal(t, &sdkmanifest.Component{Name: "damlc", Version: sdkmanifest.AssemblySemVer(version)}, cs["damlc"])

	uri := "oci://example.com/a/b/foo:1.2.3"
	assert.Equal(t, &sdkmanifest.Component{Name: "example.com/a/b/foo", Uri: &uri}, cs["example.com/a/b/foo"])

	uri = "oci://127.0.0.1:8080/foo/baz:1.2.3"
	assert.Equal(t, &sdkmanifest.Component{Name: "127.0.0.1:8080/foo/baz", Uri: &uri}, cs["127.0.0.1:8080/foo/baz"])
}
