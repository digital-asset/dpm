package yamledit

import (
	"testing"

	"daml.com/x/assistant/pkg/yamledit/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var item = attachLineComment(`name: newly-added
path: /newly/added`, "# some foo comment")

func TestAddToList(t *testing.T) {
	t.Run("non-empty list", func(t *testing.T) {
		output, err := AddToList(testdata.InputNonEmptyList, "components", item)
		require.NoError(t, err)
		assert.Equal(t, string(testdata.ExpectedNonEmptyList), output)
	})

	t.Run("empty list", func(t *testing.T) {
		output, err := AddToList(testdata.InputEmptyList, "components", item)
		require.NoError(t, err)
		assert.Equal(t, string(testdata.ExpectedEmptyList), output)
	})
}

func TestUpdateItemInList(t *testing.T) {
	t.Run("last item", func(t *testing.T) {
		output, err := ReplaceItemInList(testdata.InputReplaceLast, "components", 2, item)
		require.NoError(t, err)
		assert.Equal(t, string(testdata.ExpectedReplaceLast), output)
	})

	t.Run("not last item", func(t *testing.T) {
		output, err := ReplaceItemInList(testdata.InputReplaceNotLast, "components", 0, item)
		require.NoError(t, err)
		assert.Equal(t, string(testdata.ExpectedReplaceNotLast), output)
	})
}
