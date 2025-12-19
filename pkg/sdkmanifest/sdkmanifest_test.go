// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifest

import (
	"testing"

	"daml.com/x/assistant/pkg/sdkmanifest/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSdkManifestContents(t *testing.T) {
	assembly, err := ReadSdkManifestContents(testdata.Valid, "-")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", assembly.Spec.Version.Value().String())

	for _, y := range [][]byte{testdata.Empty, testdata.ZeroComponents, testdata.EmptyComponent, testdata.InvalidEdition} {
		_, err = ReadSdkManifestContents(y, "-")
		assert.ErrorIs(t, err, ErrInvalidAssemblyManifest)
	}

}
