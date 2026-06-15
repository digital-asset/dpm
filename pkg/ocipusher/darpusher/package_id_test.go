package darpusher

import (
	"testing"

	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestDar(t *testing.T) {
	darPath := testutil.TestdataPath(t, "test-dar", "test.dar")
	hash, err := GetMainPackageId(darPath)
	require.NoError(t, err)
	require.Equal(t, "0984ff5e3082add400bfcc6e3244bf9822ca5a617cfd92429e3fbce58058dbfa", hash)
}
