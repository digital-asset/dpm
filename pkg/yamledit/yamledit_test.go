package yamledit

import (
	"testing"

	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/yamledit/testdata"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendToYaml(t *testing.T) {
	item, err := yaml.Marshal(componentlist.ComponentList{
		&componentlist.ComponentEntry{FileBased: &componentlist.FileBased{
			Name: "newly-added",
			Path: "/newly/added",
		},
		},
	})
	require.NoError(t, err)

	t.Run("non-empty list", func(t *testing.T) {
		output := AppendToYaml(testdata.InputNonEmptyList, "components", string(item))
		assert.Equal(t, string(testdata.ExpectedNonEmptyList), output)
	})

	t.Run("empty list", func(t *testing.T) {
		output := AppendToYaml(testdata.InputEmptyList, "components", string(item))
		assert.Equal(t, string(testdata.ExpectedEmptyList), output)
	})
}
