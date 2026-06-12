package yamledit

import (
	"testing"

	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/yamledit/testdata"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
)

func TestAppendToYaml(t *testing.T) {

	item, err := yaml.Marshal(componentlist.ComponentList{
		&componentlist.ComponentEntry{FileBased: &componentlist.FileBased{
			Name: component,
			Path: "/",
		},
		},
	})

	if err != nil {
		return err
	}

	output := AppendToYaml(testdata.Input, "components", item)
	assert.Equal(t, testdata.Expected, output)
}
